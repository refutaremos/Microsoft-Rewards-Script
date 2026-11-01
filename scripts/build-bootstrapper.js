import fs from 'node:fs'
import path from 'node:path'
import crypto from 'node:crypto'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const projectRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const require = createRequire(import.meta.url)
const bootstrapperRoot = path.join(projectRoot, 'scripts', 'bootstrapper')
const payloadRoot = path.join(bootstrapperRoot, 'payload')
const outputRoot = path.join(projectRoot, 'dist', 'bootstrapper')
const maxBinarySize = 50 * 1024 * 1024

function parseTarget(value) {
    const [goos, goarch] = value.split('/')
    const architecture = goarch === 'x64' ? 'amd64' : goarch === 'arm64' ? 'arm64' : goarch
    if (!['windows', 'linux', 'darwin'].includes(goos) || !['amd64', 'arm64'].includes(architecture)) {
        throw new Error(
            `Unsupported target: ${goos}/${architecture}. Supported targets: windows|linux|darwin and amd64|arm64.`
        )
    }
    return { goos, goarch: architecture }
}

function parseTargets() {
    const targetArgument = process.argv.find(argument => argument.startsWith('--target='))
    if (targetArgument) return [parseTarget(targetArgument.slice('--target='.length))]

    const hostOS = process.platform === 'win32' ? 'windows' : process.platform === 'darwin' ? 'darwin' : 'linux'
    const targets = [parseTarget(`${hostOS}/${process.arch}`)]
    if (hostOS !== 'windows') targets.push(parseTarget('windows/amd64'))
    return [...new Map(targets.map(target => [`${target.goos}/${target.goarch}`, target])).values()]
}

function hashPayload(directory, hash = crypto.createHash('sha256'), relative = '') {
    for (const entry of fs
        .readdirSync(directory, { withFileTypes: true })
        .sort((left, right) => left.name.localeCompare(right.name))) {
        const absolutePath = path.join(directory, entry.name)
        const relativePath = path.posix.join(relative, entry.name)
        if (entry.isDirectory()) {
            hashPayload(absolutePath, hash, relativePath)
        } else if (entry.isFile() && entry.name !== 'build-id') {
            hash.update(relativePath)
            hash.update(fs.readFileSync(absolutePath))
        }
    }
    return hash
}

function stableStringify(value) {
    if (Array.isArray(value)) return `[${value.map(stableStringify).join(',')}]`
    if (value && typeof value === 'object') {
        return `{${Object.keys(value)
            .sort()
            .map(key => `${JSON.stringify(key)}:${stableStringify(value[key])}`)
            .join(',')}}`
    }
    return JSON.stringify(value)
}

function productionDependencyId(packageLock) {
    const lock = JSON.parse(packageLock)
    const packages = Object.fromEntries(
        Object.entries(lock.packages)
            .filter(([name, metadata]) => !name || !metadata.dev)
            .map(([name, metadata]) => {
                const normalized = { ...metadata }
                delete normalized.dev
                delete normalized.devOptional
                if (!name) delete normalized.devDependencies
                return [name, normalized]
            })
    )
    const manifest = { lockfileVersion: lock.lockfileVersion, name: lock.name, version: lock.version, packages }
    return crypto.createHash('sha256').update(stableStringify(manifest)).digest('hex')
}

function main() {
    let stagingCreated = false
    try {
        const targets = parseTargets()
        execFileSync('go', ['version'], { cwd: projectRoot, stdio: 'inherit' })
        const npmCommand = process.platform === 'win32' ? 'npm.cmd' : 'npm'
        execFileSync(npmCommand, ['run', 'build'], {
            cwd: projectRoot,
            stdio: 'inherit',
            shell: process.platform === 'win32'
        })

        const esbuildCommand = path.join(
            projectRoot,
            'node_modules',
            '.bin',
            process.platform === 'win32' ? 'esbuild.cmd' : 'esbuild'
        )
        const bundlePath = path.join(projectRoot, 'dist', 'bundle.js')
        execFileSync(
            esbuildCommand,
            [
                'src/index.ts',
                '--bundle',
                '--packages=external',
                '--platform=node',
                '--format=cjs',
                '--target=node24',
                `--outfile=${bundlePath}`
            ],
            { cwd: projectRoot, stdio: 'inherit', shell: process.platform === 'win32' }
        )
        const { APP_CONFIG, ASCII_ART } = require(path.join(projectRoot, 'dist', 'constants', 'runtimeConstants.js'))

        fs.rmSync(payloadRoot, { recursive: true, force: true })
        stagingCreated = true
        fs.mkdirSync(payloadRoot, { recursive: true })
        const installPackage = JSON.parse(fs.readFileSync(path.join(projectRoot, 'package.json'), 'utf8'))
        installPackage.scripts = {}
        fs.writeFileSync(path.join(payloadRoot, 'install-package.json'), `${JSON.stringify(installPackage, null, 4)}\n`)
        const runtimePackage = {
            name: installPackage.name,
            version: installPackage.version,
            scripts: { start: 'node -' }
        }
        fs.writeFileSync(path.join(payloadRoot, 'package.json'), `${JSON.stringify(runtimePackage, null, 4)}\n`)
        const packageLock = fs.readFileSync(path.join(projectRoot, 'package-lock.json'))
        fs.writeFileSync(path.join(payloadRoot, 'package-lock.json'), packageLock)
        fs.writeFileSync(path.join(payloadRoot, 'dependency-id'), `${productionDependencyId(packageLock)}\n`)
        fs.writeFileSync(path.join(payloadRoot, 'setup-constants'), JSON.stringify({ APP_CONFIG, ASCII_ART }))
        fs.copyFileSync(bundlePath, path.join(payloadRoot, 'bundle.js'))
        for (const filename of ['setup.js', 'install-browser.js']) {
            fs.copyFileSync(path.join(projectRoot, 'scripts', filename), path.join(payloadRoot, filename))
        }

        const buildId = hashPayload(payloadRoot).digest('hex').slice(0, 16)
        fs.writeFileSync(path.join(payloadRoot, 'build-id'), `${buildId}\n`)

        execFileSync('go', ['test', './scripts/bootstrapper'], {
            cwd: projectRoot,
            stdio: 'inherit',
            env: { ...process.env, CGO_ENABLED: '0', GO111MODULE: 'off' }
        })

        fs.mkdirSync(outputRoot, { recursive: true })
        for (const target of targets) {
            const extension = target.goos === 'windows' ? '.exe' : ''
            const outputPath = path.join(outputRoot, `mrs-bootstrapper-${target.goos}-${target.goarch}${extension}`)
            execFileSync('go', ['build', '-trimpath', '-ldflags=-s -w', '-o', outputPath, './scripts/bootstrapper'], {
                cwd: projectRoot,
                stdio: 'inherit',
                env: {
                    ...process.env,
                    CGO_ENABLED: '0',
                    GO111MODULE: 'off',
                    GOOS: target.goos,
                    GOARCH: target.goarch
                }
            })

            const size = fs.statSync(outputPath).size
            console.log(
                `[Build] Created ${path.relative(projectRoot, outputPath)} (${(size / 1024 / 1024).toFixed(1)} MiB).`
            )
            if (size > maxBinarySize) throw new Error('Bootstrapper exceeds the 50 MiB size target.')
        }
    } catch (error) {
        const detail = error instanceof Error ? error.message : String(error)
        console.error(`[Build] Bootstrapper build failed: ${detail}`)
        process.exitCode = 1
    } finally {
        if (stagingCreated) fs.rmSync(payloadRoot, { recursive: true, force: true })
    }
}

main()
