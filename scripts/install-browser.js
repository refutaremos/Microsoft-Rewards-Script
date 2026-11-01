import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'
import { createRequire } from 'node:module'

const require = createRequire(import.meta.url)
const projectRoot = process.env.MRS_APP_DIR
    ? path.resolve(process.env.MRS_APP_DIR)
    : path.resolve(import.meta.dirname, '..')

if (process.env.MRS_SKIP_BROWSER_INSTALL === '1') {
    console.log('[Setup] Chromium installation skipped by MRS_SKIP_BROWSER_INSTALL=1.')
    process.exit(0)
}

let chromium
try {
    chromium = require('patchright').chromium
    if (!chromium) throw new Error('Chromium API unavailable')
} catch {
    console.error('[Setup] Patchright is not installed yet.')
    process.exit(1)
}

const cliPath = path.join(projectRoot, 'node_modules', 'patchright', 'cli.js')
const browser = process.env.MRS_HEADLESS_ONLY === '1' ? 'chromium-headless-shell' : 'chromium'
const browserCachePath = process.env.PLAYWRIGHT_BROWSERS_PATH
const browserInstalled =
    process.env.MRS_HEADLESS_ONLY === '1'
        ? Boolean(
              browserCachePath &&
              fs.existsSync(browserCachePath) &&
              fs
                  .readdirSync(browserCachePath, { withFileTypes: true })
                  .some(entry => entry.isDirectory() && entry.name.startsWith('chromium_headless_shell-'))
          )
        : fs.existsSync(chromium.executablePath())
const args = [cliPath, 'install', browser]
if (process.platform === 'linux' && typeof process.getuid === 'function' && process.getuid() === 0) {
    args.push('--with-deps')
}

if (!browserInstalled) {
    console.log(`[Setup] Downloading ${browser}...`)
    const result = spawnSync(process.execPath, args, { cwd: projectRoot, stdio: 'inherit' })
    if (result.error) {
        console.error(`[Setup] Browser installation failed: ${result.error.message}`)
        process.exit(1)
    }
    if (result.status !== 0) process.exit(result.status ?? 1)
}

if (process.platform === 'linux') {
    const check = spawnSync(process.execPath, [cliPath, 'install-deps', browser, '--dry-run'], {
        cwd: projectRoot,
        encoding: 'utf8'
    })
    if (check.error) {
        console.error(`[Setup] Linux browser dependency check failed: ${check.error.message}`)
        process.exit(1)
    }

    if (check.status !== 0) {
        if (check.stdout?.trim()) console.log(check.stdout.trim())
        if (check.stderr?.trim()) console.error(check.stderr.trim())
        console.log('Installing dependencies...')
        console.log('[Setup] Installing missing Linux dependencies for Chromium...')
        const install = spawnSync(process.execPath, [cliPath, 'install-deps', browser], {
            cwd: projectRoot,
            stdio: 'inherit'
        })
        if (install.error) {
            console.error(`[Setup] Linux dependency installation failed: ${install.error.message}`)
            process.exit(1)
        }
        if (install.status !== 0) {
            console.error('[Setup] Could not install Chromium dependencies. Run npm run install:browser-deps manually.')
            process.exit(install.status ?? 1)
        }
    }
}
