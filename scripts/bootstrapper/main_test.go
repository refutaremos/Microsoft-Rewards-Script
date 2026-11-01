package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEmbeddedPayloadUnpacks(t *testing.T) {
	destination := t.TempDir()
	if err := fsCopyEmbedded("payload", destination); err != nil {
		t.Fatalf("unpack embedded payload: %v", err)
	}

	for _, relativePath := range []string{
		"bundle.js",
		"setup.js",
		"install-browser.js",
		"package.json",
		"package-lock.json",
	} {
		if _, err := os.Stat(filepath.Join(destination, relativePath)); !os.IsNotExist(err) {
			t.Errorf("expected embedded file %q to remain off disk, stat error: %v", relativePath, err)
		}
	}
	if _, err := os.Stat(filepath.Join(destination, "scripts")); !os.IsNotExist(err) {
		t.Fatalf("expected runtime payload to omit scripts directory, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "src")); !os.IsNotExist(err) {
		t.Fatalf("expected runtime payload to omit source constants directory, stat error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destination, "setup-constants")); !os.IsNotExist(err) {
		t.Fatalf("expected setup constants to remain embedded, stat error: %v", err)
	}
	setupConstants, err := embedded.ReadFile("payload/setup-constants")
	if err != nil {
		t.Fatalf("read embedded setup constants: %v", err)
	}
	var constants struct {
		AppConfig map[string]any `json:"APP_CONFIG"`
		ASCIIArt  []string       `json:"ASCII_ART"`
	}
	if err := json.Unmarshal(setupConstants, &constants); err != nil {
		t.Fatalf("parse embedded setup constants: %v", err)
	}
	if len(constants.AppConfig) == 0 || len(constants.ASCIIArt) == 0 {
		t.Fatal("expected embedded app config and ASCII art")
	}
	dependencyLock, err := embedded.ReadFile("payload/package-lock.json")
	if err != nil {
		t.Fatalf("read embedded package lock: %v", err)
	}
	actualID, err := productionDependencyID(dependencyLock)
	if err != nil {
		t.Fatalf("calculate production dependency ID: %v", err)
	}
	expectedID, err := embedded.ReadFile("payload/dependency-id")
	if err != nil || actualID != string(bytes.TrimSpace(expectedID)) {
		t.Fatalf("production dependency ID mismatch: got %q, want %q (error: %v)", actualID, expectedID, err)
	}

	for _, embeddedPath := range []string{
		"payload/bundle.js",
		"payload/setup.js",
		"payload/install-browser.js",
		"payload/install-package.json",
	} {
		contents, err := embedded.ReadFile(embeddedPath)
		if err != nil || len(contents) == 0 {
			t.Fatalf("expected non-empty embedded resource %q, error: %v", embeddedPath, err)
		}
	}
	if err := withEmbeddedPackage(destination, false, func() error {
		packageFile, err := os.ReadFile(filepath.Join(destination, "package.json"))
		if err != nil {
			return err
		}
		var packageJSON struct {
			Scripts map[string]string `json:"scripts"`
		}
		if err := json.Unmarshal(packageFile, &packageJSON); err != nil {
			return err
		}
		if packageJSON.Scripts["start"] != "node -" {
			return fmt.Errorf("packaged npm start = %q, want node -", packageJSON.Scripts["start"])
		}
		if _, err := os.Stat(filepath.Join(destination, "package-lock.json")); !os.IsNotExist(err) {
			return fmt.Errorf("runtime manifest unexpectedly wrote package-lock.json: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("temporarily materialize runtime manifest: %v", err)
	}
	if err := withEmbeddedPackage(destination, true, func() error {
		packageFile, err := os.ReadFile(filepath.Join(destination, "package.json"))
		if err != nil {
			return err
		}
		var packageJSON struct {
			Dependencies map[string]string `json:"dependencies"`
			Scripts      map[string]string `json:"scripts"`
		}
		if err := json.Unmarshal(packageFile, &packageJSON); err != nil {
			return err
		}
		if packageJSON.Dependencies["patchright"] == "" {
			return fmt.Errorf("install manifest does not declare patchright")
		}
		if _, err := os.Stat(filepath.Join(destination, "package-lock.json")); err != nil {
			return err
		}
		if len(packageJSON.Scripts) != 0 {
			return fmt.Errorf("install manifest contains unexpected lifecycle scripts")
		}
		return nil
	}); err != nil {
		t.Fatalf("temporarily materialize install manifest: %v", err)
	}
	for _, transientFile := range []string{"package.json", "package-lock.json"} {
		if _, err := os.Stat(filepath.Join(destination, transientFile)); !os.IsNotExist(err) {
			t.Errorf("expected temporary package file %q to be removed, stat error: %v", transientFile, err)
		}
	}
}

func TestEnsureAppReusesDependenciesFromPreviousBuild(t *testing.T) {
	runtimeDirectory := t.TempDir()
	appRoot := filepath.Join(runtimeDirectory, "app")
	currentApp := filepath.Join(appRoot, "current")
	legacyApp := filepath.Join(appRoot, "previous-build")
	legacyModules := filepath.Join(legacyApp, "node_modules")
	if err := os.MkdirAll(filepath.Join(legacyModules, "patchright"), 0o700); err != nil {
		t.Fatal(err)
	}
	packageLock, err := embedded.ReadFile("payload/package-lock.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyApp, "package-lock.json"), packageLock, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyModules, "patchright", "package.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyApp, ".bootstrap-ready"), []byte("legacy\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureApp(currentApp, runtimeDirectory, "current-build"); err != nil {
		t.Fatalf("ensure app: %v", err)
	}

	expectedID, err := productionDependencyID(packageLock)
	if err != nil {
		t.Fatal(err)
	}
	if !dependenciesReady(currentApp, expectedID) {
		t.Fatal("expected compatible node_modules to be reused")
	}
	if _, err := os.Stat(legacyApp); !os.IsNotExist(err) {
		t.Fatalf("expected old runtime directory to be cleaned, stat error: %v", err)
	}
}

func TestProductionDependencyIDIgnoresDevOnlyChanges(t *testing.T) {
	packageLock, err := embedded.ReadFile("payload/package-lock.json")
	if err != nil {
		t.Fatal(err)
	}
	originalID, err := productionDependencyID(packageLock)
	if err != nil {
		t.Fatal(err)
	}
	var lock map[string]any
	if err := json.Unmarshal(packageLock, &lock); err != nil {
		t.Fatal(err)
	}
	packages := lock["packages"].(map[string]any)
	root := packages[""].(map[string]any)
	root["devDependencies"].(map[string]any)["new-build-tool"] = "1.0.0"
	packages["node_modules/new-build-tool"] = map[string]any{"dev": true, "version": "1.0.0"}
	modifiedLock, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	modifiedID, err := productionDependencyID(modifiedLock)
	if err != nil {
		t.Fatal(err)
	}
	if modifiedID != originalID {
		t.Fatalf("dev-only change altered production dependency ID: %q != %q", modifiedID, originalID)
	}
}

func TestFindCachedNodeRuntimeUsesWorkspaceRuntime(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell stub for the Node executable")
	}
	runtimeRoot := t.TempDir()
	nodeDirectory := filepath.Join(runtimeRoot, "node", "24.22.1")
	nodePath := filepath.Join(nodeDirectory, "bin", "node")
	npmPath := filepath.Join(nodeDirectory, "lib", "node_modules", "npm", "bin", "npm-cli.js")
	if err := os.MkdirAll(filepath.Dir(nodePath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(npmPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nodePath, []byte("#!/bin/sh\nprintf 'v24.22.1\\n'\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(npmPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	cached, found := findCachedNodeRuntime(runtimeRoot)
	if !found {
		t.Fatal("expected cached Node.js runtime to be found")
	}
	if cached.Path != nodePath || cached.NpmPath != npmPath || !cached.Portable {
		t.Fatalf("unexpected cached runtime: %+v", cached)
	}
}

func TestCleanupHeadlessBrowserCacheRemovesUnusedAssets(t *testing.T) {
	browserDirectory := t.TempDir()
	for _, directory := range []string{
		"chromium-1228",
		"chromium_headless_shell-1228",
		"ffmpeg-1011",
	} {
		if err := os.Mkdir(filepath.Join(browserDirectory, directory), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := cleanupHeadlessBrowserCache(browserDirectory); err != nil {
		t.Fatalf("cleanup browser cache: %v", err)
	}
	for _, directory := range []string{"chromium-1228", "ffmpeg-1011"} {
		if _, err := os.Stat(filepath.Join(browserDirectory, directory)); !os.IsNotExist(err) {
			t.Errorf("expected unused browser asset %q to be removed", directory)
		}
	}
	if _, err := os.Stat(filepath.Join(browserDirectory, "chromium_headless_shell-1228")); err != nil {
		t.Fatalf("headless shell should be preserved: %v", err)
	}
}

func TestHasHeadlessBrowserOnlyMatchesShellCache(t *testing.T) {
	browserDirectory := t.TempDir()
	if err := os.Mkdir(filepath.Join(browserDirectory, "chromium-1228"), 0o700); err != nil {
		t.Fatal(err)
	}
	if hasHeadlessBrowser(browserDirectory) {
		t.Fatal("full Chromium should not satisfy headless shell detection")
	}
	if err := os.Mkdir(filepath.Join(browserDirectory, "chromium_headless_shell-1228"), 0o700); err != nil {
		t.Fatal(err)
	}
	if !hasHeadlessBrowser(browserDirectory) {
		t.Fatal("expected installed headless shell to be detected")
	}
}
