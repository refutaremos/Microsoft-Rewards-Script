package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//go:embed payload/**
var embedded embed.FS
var errInterrupted = errors.New("interrupted")

type nodeRelease struct {
	Version string   `json:"version"`
	Files   []string `json:"files"`
}

type nodeRuntime struct {
	Path       string
	NpmPath    string
	Portable   bool
	SearchPath string
}

func main() {
	for _, argument := range os.Args[1:] {
		if argument == "--help" || argument == "-h" {
			fmt.Println("Usage: mrs-bootstrapper [--setup] [--help] [--version]")
			return
		}
		if argument == "--version" {
			fmt.Println("Microsoft Rewards Script bootstrapper")
			return
		}
	}

	printBanner()
	fmt.Println("Installing dependencies...")
	if err := run(); err != nil {
		if errors.Is(err, errInterrupted) {
			return
		}
		fmt.Fprintf(os.Stderr, "\n[Bootstrapper] %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not locate the bootstrapper executable: %w", err)
	}
	dataDirectory := filepath.Dir(executablePath)
	workspaceDirectory := filepath.Join(dataDirectory, "workspace")
	runtimeDirectory := filepath.Join(workspaceDirectory, ".runtime")
	temporaryDirectory := filepath.Join(runtimeDirectory, "tmp")
	for _, directory := range []string{workspaceDirectory, runtimeDirectory, temporaryDirectory} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return fmt.Errorf("could not create workspace directory %q: %w", directory, err)
		}
	}

	buildID, err := embedded.ReadFile("payload/build-id")
	if err != nil {
		return fmt.Errorf("embedded app version is missing: %w", err)
	}
	setupConstants, err := embedded.ReadFile("payload/setup-constants")
	if err != nil {
		return fmt.Errorf("embedded setup constants are missing: %w", err)
	}
	bundle, err := embedded.ReadFile("payload/bundle.js")
	if err != nil {
		return fmt.Errorf("embedded application bundle is missing: %w", err)
	}
	appDirectory := filepath.Join(runtimeDirectory, "app", "current")
	if err := ensureApp(appDirectory, runtimeDirectory, strings.TrimSpace(string(buildID))); err != nil {
		return err
	}

	runtimeInfo, err := findNodeRuntime(runtimeDirectory)
	if err != nil {
		return err
	}

	environment := os.Environ()
	for key, value := range map[string]string{
		"MRS_DATA_DIR":             dataDirectory,
		"MRS_HEADLESS_ONLY":        "1",
		"MRS_ROOT_DIR":             dataDirectory,
		"MRS_APP_DIR":              appDirectory,
		"MRS_RUNTIME_CONSTANTS":    string(setupConstants),
		"MRS_SETUP_SKIPPED":        "1",
		"MRS_SEA":                  "1",
		"PLAYWRIGHT_BROWSERS_PATH": filepath.Join(runtimeDirectory, "browsers"),
		"npm_config_cache":         filepath.Join(runtimeDirectory, "npm-cache"),
		"TEMP":                     temporaryDirectory,
		"TMP":                      temporaryDirectory,
		"TMPDIR":                   temporaryDirectory,
	} {
		environment = setEnvironment(environment, key, value)
	}
	if runtimeInfo.SearchPath != "" {
		key := "PATH"
		if runtime.GOOS == "windows" {
			key = "Path"
		}
		environment = setEnvironment(
			environment,
			key,
			runtimeInfo.SearchPath+string(os.PathListSeparator)+os.Getenv(key),
		)
	}

	dependencyID, err := embedded.ReadFile("payload/dependency-id")
	if err != nil {
		return fmt.Errorf("embedded dependency ID is missing: %w", err)
	}
	if !dependenciesReady(appDirectory, strings.TrimSpace(string(dependencyID))) {
		if err := withEmbeddedPackage(appDirectory, true, func() error {
			return runNpm(runtimeInfo, appDirectory, environment, "ci", "--omit=dev")
		}); err != nil {
			return fmt.Errorf("npm dependency installation failed; check the network and retry: %w", err)
		}
		if err := markDependenciesReady(appDirectory, strings.TrimSpace(string(dependencyID))); err != nil {
			return fmt.Errorf("could not mark installed dependencies: %w", err)
		}
	}
	installBrowserScript, err := embedded.ReadFile("payload/install-browser.js")
	if err != nil {
		return fmt.Errorf("embedded browser installer is missing: %w", err)
	}
	if err := runNodeInput(runtimeInfo, appDirectory, environment, installBrowserScript); err != nil {
		return fmt.Errorf("browser setup failed; on Linux, rerun with permission to install system packages: %w", err)
	}
	if err := cleanupHeadlessBrowserCache(filepath.Join(runtimeDirectory, "browsers")); err != nil {
		return fmt.Errorf("could not remove unused browser files: %w", err)
	}

	if hasArgument("--setup") || !hasConfiguredAccount(filepath.Join(workspaceDirectory, "accounts.txt")) {
		setupScript, err := embedded.ReadFile("payload/setup.js")
		if err != nil {
			return fmt.Errorf("embedded setup wizard is missing: %w", err)
		}
		if err := runNodeSource(runtimeInfo, appDirectory, environment, setupScript); err != nil {
			return fmt.Errorf("setup wizard failed: %w", err)
		}
	}

	fmt.Println("[Bootstrapper] Starting Microsoft Rewards Script...")
	if err := withEmbeddedPackage(appDirectory, false, func() error {
		return runNpmInput(runtimeInfo, appDirectory, environment, bundle, "run", "start")
	}); err != nil {
		return fmt.Errorf("npm run start failed: %w", err)
	}
	return nil
}

func ensureApp(appDirectory, runtimeDirectory, buildID string) error {
	if err := os.MkdirAll(appDirectory, 0o700); err != nil {
		return fmt.Errorf("could not create app runtime: %w", err)
	}
	if err := reuseDependencies(appDirectory, runtimeDirectory); err != nil {
		return err
	}

	marker := filepath.Join(appDirectory, ".bootstrap-ready")
	for _, transientFile := range []string{
		"bundle.js",
		"setup.js",
		"install-browser.js",
		"package.json",
		"package-lock.json",
		"scripts",
	} {
		if err := os.RemoveAll(filepath.Join(appDirectory, transientFile)); err != nil {
			return fmt.Errorf("could not remove transient runtime file %q: %w", transientFile, err)
		}
	}
	if contents, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(contents)) == buildID {
		cleanupLegacyApps(runtimeDirectory, appDirectory)
		return nil
	}
	if err := fsCopyEmbedded("payload", appDirectory); err != nil {
		return fmt.Errorf("could not unpack the application: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(appDirectory, "scripts")); err != nil {
		return fmt.Errorf("could not remove obsolete runtime scripts: %w", err)
	}
	if err := os.WriteFile(marker, []byte(buildID+"\n"), 0o600); err != nil {
		return fmt.Errorf("could not mark app cache as ready: %w", err)
	}
	cleanupLegacyApps(runtimeDirectory, appDirectory)
	return nil
}

func reuseDependencies(appDirectory, runtimeDirectory string) error {
	dependencyID, err := embedded.ReadFile("payload/dependency-id")
	if err != nil {
		return err
	}
	expectedID := strings.TrimSpace(string(dependencyID))
	if dependenciesReady(appDirectory, expectedID) {
		return nil
	}

	nodeModules := filepath.Join(appDirectory, "node_modules")
	if _, err := os.Stat(nodeModules); err == nil {
		if packageLockMatches(filepath.Join(appDirectory, "package-lock.json"), expectedID) && hasPatchright(nodeModules) {
			return markDependenciesReady(appDirectory, expectedID)
		}
		return nil
	}

	legacyRoot := filepath.Join(runtimeDirectory, "app")
	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == filepath.Base(appDirectory) {
			continue
		}
		legacyApp := filepath.Join(legacyRoot, entry.Name())
		if _, err := os.Stat(filepath.Join(legacyApp, ".bootstrap-ready")); err != nil {
			continue
		}
		legacyModules := filepath.Join(legacyApp, "node_modules")
		if !packageLockMatches(filepath.Join(legacyApp, "package-lock.json"), expectedID) || !hasPatchright(legacyModules) {
			continue
		}
		if err := os.Rename(legacyModules, nodeModules); err != nil {
			return fmt.Errorf("could not reuse existing node_modules from %q: %w", legacyApp, err)
		}
		return markDependenciesReady(appDirectory, expectedID)
	}
	return nil
}

func dependenciesReady(appDirectory, expectedID string) bool {
	marker, err := os.ReadFile(filepath.Join(appDirectory, "node_modules", ".mrs-dependency-id"))
	return err == nil && strings.TrimSpace(string(marker)) == expectedID && hasPatchright(filepath.Join(appDirectory, "node_modules"))
}

func markDependenciesReady(appDirectory, dependencyID string) error {
	markerPath := filepath.Join(appDirectory, "node_modules", ".mrs-dependency-id")
	return os.WriteFile(markerPath, []byte(dependencyID+"\n"), 0o600)
}

func hasPatchright(nodeModules string) bool {
	_, err := os.Stat(filepath.Join(nodeModules, "patchright", "package.json"))
	return err == nil
}

func packageLockMatches(packageLockPath, expectedID string) bool {
	contents, err := os.ReadFile(packageLockPath)
	if err != nil {
		return false
	}
	actualID, err := productionDependencyID(contents)
	return err == nil && actualID == expectedID
}

func productionDependencyID(contents []byte) (string, error) {
	var lock map[string]any
	if err := json.Unmarshal(contents, &lock); err != nil {
		return "", err
	}
	packages, ok := lock["packages"].(map[string]any)
	if !ok {
		return "", errors.New("package lock does not contain a packages map")
	}
	productionPackages := make(map[string]any, len(packages))
	for name, rawMetadata := range packages {
		metadata, ok := rawMetadata.(map[string]any)
		if !ok {
			return "", fmt.Errorf("invalid package lock entry %q", name)
		}
		if name != "" && metadata["dev"] == true {
			continue
		}
		normalized := make(map[string]any, len(metadata))
		for key, value := range metadata {
			if key == "dev" || key == "devOptional" || (name == "" && key == "devDependencies") {
				continue
			}
			normalized[key] = value
		}
		productionPackages[name] = normalized
	}
	manifest := map[string]any{
		"lockfileVersion": lock["lockfileVersion"],
		"name":            lock["name"],
		"version":         lock["version"],
		"packages":        productionPackages,
	}
	var canonical bytes.Buffer
	encoder := json.NewEncoder(&canonical)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(manifest); err != nil {
		return "", err
	}
	digest := sha256.Sum256(bytes.TrimSpace(canonical.Bytes()))
	return hex.EncodeToString(digest[:]), nil
}

func cleanupLegacyApps(runtimeDirectory, currentApp string) {
	legacyRoot := filepath.Join(runtimeDirectory, "app")
	entries, err := os.ReadDir(legacyRoot)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == filepath.Base(currentApp) {
			continue
		}
		legacyApp := filepath.Join(legacyRoot, entry.Name())
		if _, err := os.Stat(filepath.Join(legacyApp, ".bootstrap-ready")); err == nil {
			_ = os.RemoveAll(legacyApp)
		}
	}
}

func fsCopyEmbedded(source, destination string) error {
	entries, err := embedded.ReadDir(source)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.Name() == "build-id" || entry.Name() == "dependency-id" || entry.Name() == "setup-constants" ||
			entry.Name() == "bundle.js" || entry.Name() == "setup.js" || entry.Name() == "install-browser.js" ||
			entry.Name() == "package.json" || entry.Name() == "install-package.json" ||
			entry.Name() == "package-lock.json" {
			continue
		}
		sourcePath := path.Join(source, entry.Name())
		destinationPath := filepath.Join(destination, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(destinationPath, 0o755); err != nil {
				return err
			}
			if err := fsCopyEmbedded(sourcePath, destinationPath); err != nil {
				return err
			}
			continue
		}
		contents, err := embedded.ReadFile(sourcePath)
		if err != nil {
			return err
		}
		if err := os.WriteFile(destinationPath, contents, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func findNodeRuntime(runtimeRoot string) (nodeRuntime, error) {
	nodeName := "node"
	npmName := "npm"
	if runtime.GOOS == "windows" {
		nodeName = "node.exe"
		npmName = "npm.cmd"
	}
	if nodePath, err := exec.LookPath(nodeName); err == nil && nodeMajorVersion(nodePath) >= 24 {
		if npmPath, npmErr := exec.LookPath(npmName); npmErr == nil {
			return nodeRuntime{Path: nodePath, NpmPath: npmPath, SearchPath: filepath.Dir(nodePath)}, nil
		}
	}
	if cachedRuntime, found := findCachedNodeRuntime(runtimeRoot); found {
		fmt.Println("[Bootstrapper] Reusing the Node.js runtime from workspace/.runtime.")
		return cachedRuntime, nil
	}

	fmt.Println("[Bootstrapper] Node.js 24+ with npm was not found. Downloading the official Node.js runtime...")
	return downloadNodeRuntime(runtimeRoot)
}

func findCachedNodeRuntime(runtimeRoot string) (nodeRuntime, bool) {
	nodeRoot := filepath.Join(runtimeRoot, "node")
	entries, err := os.ReadDir(nodeRoot)
	if err != nil {
		return nodeRuntime{}, false
	}
	for index := len(entries) - 1; index >= 0; index-- {
		if !entries[index].IsDir() {
			continue
		}
		candidate := portableRuntime(filepath.Join(nodeRoot, entries[index].Name()))
		if nodeMajorVersion(candidate.Path) < 24 {
			continue
		}
		if _, err := os.Stat(candidate.NpmPath); err != nil {
			continue
		}
		return candidate, true
	}
	return nodeRuntime{}, false
}

func setEnvironment(environment []string, key, value string) []string {
	for index, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if strings.EqualFold(name, key) {
			environment[index] = key + "=" + value
			return environment
		}
	}
	return append(environment, key+"="+value)
}

func nodeMajorVersion(nodePath string) int {
	output, err := exec.Command(nodePath, "--version").Output()
	if err != nil {
		return 0
	}
	version := strings.TrimPrefix(strings.TrimSpace(string(output)), "v")
	major, _, _ := strings.Cut(version, ".")
	var result int
	_, _ = fmt.Sscanf(major, "%d", &result)
	return result
}

func downloadNodeRuntime(runtimeRoot string) (nodeRuntime, error) {
	platform, architecture, err := nodePlatform()
	if err != nil {
		return nodeRuntime{}, err
	}
	client := &http.Client{Timeout: 45 * time.Minute}
	indexBody, err := fetch(client, "https://nodejs.org/dist/index.json")
	if err != nil {
		return nodeRuntime{}, fmt.Errorf("could not check official Node.js releases: %w", err)
	}
	var releases []nodeRelease
	if err := json.Unmarshal(indexBody, &releases); err != nil {
		return nodeRuntime{}, fmt.Errorf("could not read official Node.js release list: %w", err)
	}

	archiveSuffix := ".tar.gz"
	if platform == "win" {
		archiveSuffix = ".zip"
	}
	archivePrefix := fmt.Sprintf("node-%s-%s", platform, architecture)
	var release nodeRelease
	for _, candidate := range releases {
		if !strings.HasPrefix(candidate.Version, "v24.") {
			continue
		}
		for _, file := range candidate.Files {
			if file == archivePrefix {
				release = candidate
				break
			}
		}
		if release.Version != "" {
			break
		}
	}
	if release.Version == "" {
		return nodeRuntime{}, fmt.Errorf("no Node.js 24 runtime is available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	filename := fmt.Sprintf("node-%s-%s%s", release.Version, platform, archiveSuffix)
	baseURL := "https://nodejs.org/dist/" + release.Version + "/"
	checksums, err := fetch(client, baseURL+"SHASUMS256.txt")
	if err != nil {
		return nodeRuntime{}, fmt.Errorf("could not download Node.js checksums: %w", err)
	}
	var expectedHash string
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[len(fields)-1] == filename {
			expectedHash = fields[0]
			break
		}
	}
	if expectedHash == "" {
		return nodeRuntime{}, fmt.Errorf("official Node.js checksum not found for %s", filename)
	}
	archive, err := fetch(client, baseURL+filename)
	if err != nil {
		return nodeRuntime{}, fmt.Errorf("could not download Node.js: %w", err)
	}
	actualHash := sha256.Sum256(archive)
	if hex.EncodeToString(actualHash[:]) != expectedHash {
		return nodeRuntime{}, errors.New("Node.js archive checksum mismatch")
	}

	runtimeDirectory := filepath.Join(runtimeRoot, "node", strings.TrimPrefix(release.Version, "v"))
	if _, err := os.Stat(runtimeNodePath(runtimeDirectory)); err == nil {
		return portableRuntime(runtimeDirectory), nil
	}
	if err := os.RemoveAll(runtimeDirectory); err != nil {
		return nodeRuntime{}, err
	}
	if err := os.MkdirAll(runtimeDirectory, 0o700); err != nil {
		return nodeRuntime{}, err
	}
	if platform == "win" {
		err = extractZip(archive, runtimeDirectory)
	} else {
		err = extractTarGzip(archive, runtimeDirectory)
	}
	if err != nil {
		return nodeRuntime{}, fmt.Errorf("could not unpack Node.js: %w", err)
	}
	if runtime.GOOS != "windows" {
		_ = os.Chmod(runtimeNodePath(runtimeDirectory), 0o755)
	}
	if _, err := os.Stat(runtimeNodePath(runtimeDirectory)); err != nil {
		return nodeRuntime{}, errors.New("Node.js archive did not contain its runtime executable")
	}
	return portableRuntime(runtimeDirectory), nil
}

func nodePlatform() (string, string, error) {
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "darwin"
	} else if platform == "windows" {
		platform = "win"
	}
	architecture := runtime.GOARCH
	if architecture == "amd64" {
		architecture = "x64"
	} else if architecture != "arm64" {
		return "", "", fmt.Errorf("unsupported Node.js architecture: %s", runtime.GOARCH)
	}
	if platform != "linux" && platform != "darwin" && platform != "win" {
		return "", "", fmt.Errorf("unsupported Node.js operating system: %s", runtime.GOOS)
	}
	return platform, architecture, nil
}

func runtimeNodePath(directory string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(directory, "node.exe")
	}
	return filepath.Join(directory, "bin", "node")
}

func portableRuntime(directory string) nodeRuntime {
	nodePath := runtimeNodePath(directory)
	npmPath := filepath.Join(directory, "lib", "node_modules", "npm", "bin", "npm-cli.js")
	searchPath := filepath.Dir(nodePath)
	if runtime.GOOS == "windows" {
		npmPath = filepath.Join(directory, "node_modules", "npm", "bin", "npm-cli.js")
		searchPath = directory
	}
	return nodeRuntime{Path: nodePath, NpmPath: npmPath, Portable: true, SearchPath: searchPath}
}

func extractTarGzip(archive []byte, destination string) error {
	reader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return err
	}
	defer reader.Close()
	tarReader := tar.NewReader(reader)
	for {
		header, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.ToSlash(header.Name)
		parts := strings.Split(name, "/")
		if len(parts) < 2 {
			continue
		}
		relative := filepath.Join(parts[1:]...)
		if relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return errors.New("unsafe path in Node.js archive")
		}
		path := filepath.Join(destination, relative)
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		} else if header.Typeflag == tar.TypeReg || header.Typeflag == tar.TypeRegA {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode)&0o755)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(file, tarReader)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
}

func extractZip(archive []byte, destination string) error {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return err
	}
	for _, entry := range reader.File {
		name := strings.ReplaceAll(filepath.ToSlash(entry.Name), "\\", "/")
		parts := strings.Split(name, "/")
		if len(parts) < 2 {
			continue
		}
		relative := filepath.Join(parts[1:]...)
		if relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
			return errors.New("unsafe path in Node.js archive")
		}
		path := filepath.Join(destination, relative)
		if entry.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		output, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		input.Close()
		closeErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func fetch(client *http.Client, url string) ([]byte, error) {
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	request.Header.Set("User-Agent", "Microsoft-Rewards-Script-bootstrapper")
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download returned HTTP %s", response.Status)
	}
	return io.ReadAll(response.Body)
}

func runNpm(runtimeInfo nodeRuntime, directory string, environment []string, arguments ...string) error {
	return runCommand(npmCommand(runtimeInfo, arguments...), directory, environment)
}

func runNpmInput(runtimeInfo nodeRuntime, directory string, environment []string, input []byte, arguments ...string) error {
	return runCommandWithInput(npmCommand(runtimeInfo, arguments...), directory, environment, bytes.NewReader(input))
}

func npmCommand(runtimeInfo nodeRuntime, arguments ...string) *exec.Cmd {
	if runtimeInfo.Portable {
		return exec.Command(runtimeInfo.Path, append([]string{runtimeInfo.NpmPath}, arguments...)...)
	}
	if runtime.GOOS == "windows" {
		return exec.Command("cmd.exe", append([]string{"/d", "/c", runtimeInfo.NpmPath}, arguments...)...)
	}
	return exec.Command(runtimeInfo.NpmPath, arguments...)
}

func runNodeInput(runtimeInfo nodeRuntime, directory string, environment []string, input []byte) error {
	command := exec.Command(runtimeInfo.Path, "--input-type=module", "-")
	return runCommandWithInput(command, directory, environment, bytes.NewReader(input))

}

func runNodeSource(runtimeInfo nodeRuntime, directory string, environment []string, source []byte) error {
	command := exec.Command(runtimeInfo.Path, "--input-type=module", "-e", string(source))
	return runCommand(command, directory, environment)
}

func runCommand(command *exec.Cmd, directory string, environment []string) error {
	return runCommandWithInput(command, directory, environment, os.Stdin)
}

func runCommandWithInput(command *exec.Cmd, directory string, environment []string, input io.Reader) error {
	command.Dir = directory
	command.Env = environment
	command.Stdin = input
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Run(); err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			if exitError.ExitCode() == 130 {
				return errInterrupted
			}
			return fmt.Errorf("process exited with code %d", exitError.ExitCode())
		}
		return err
	}
	return nil
}

func withEmbeddedPackage(directory string, install bool, action func() error) error {
	packageResource := "payload/package.json"
	if install {
		packageResource = "payload/install-package.json"
	}
	packageJSON, err := embedded.ReadFile(packageResource)
	if err != nil {
		return fmt.Errorf("embedded package manifest %q is missing: %w", packageResource, err)
	}
	var packageLock []byte
	if install {
		packageLock, err = embedded.ReadFile("payload/package-lock.json")
		if err != nil {
			return fmt.Errorf("embedded package lock is missing: %w", err)
		}
	}
	manifestPath := filepath.Join(directory, "package.json")
	lockPath := filepath.Join(directory, "package-lock.json")
	_ = os.RemoveAll(manifestPath)
	_ = os.RemoveAll(lockPath)
	if err := os.WriteFile(manifestPath, packageJSON, 0o600); err != nil {
		return err
	}
	defer os.Remove(manifestPath)
	if install {
		if err := os.WriteFile(lockPath, packageLock, 0o600); err != nil {
			return err
		}
		defer os.Remove(lockPath)
	}
	return action()
}

func hasConfiguredAccount(accountsPath string) bool {
	contents, err := os.ReadFile(accountsPath)
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.ReplaceAll(string(contents), "\r", ""), "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), ":")
		if found && strings.EqualFold(strings.TrimSpace(key), "email") && strings.Contains(strings.TrimSpace(value), "@") {
			return true
		}
	}
	return false
}

func cleanupHeadlessBrowserCache(browserDirectory string) error {
	entries, err := os.ReadDir(browserDirectory)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasPrefix(name, "chromium_headless_shell-") {
			continue
		}
		if strings.HasPrefix(name, "chromium-") || strings.HasPrefix(name, "ffmpeg-") {
			if err := os.RemoveAll(filepath.Join(browserDirectory, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func hasArgument(expected string) bool {
	for _, argument := range os.Args[1:] {
		if argument == expected {
			return true
		}
	}
	return false
}

func printBanner() {
	contents, err := embedded.ReadFile("payload/setup-constants")
	if err != nil {
		return
	}
	var constants struct {
		ASCIIArt []string `json:"ASCII_ART"`
	}
	if json.Unmarshal(contents, &constants) != nil {
		return
	}
	fmt.Println(strings.Join(constants.ASCIIArt, "\n"))
	fmt.Println()
}
