import fs from 'node:fs'
import path from 'node:path'
import { spawnSync } from 'node:child_process'

import { getWorkspaceDirectory, isSeaBuild } from './AppPaths'

const USER_CONFIG = {
    webhook: {
        discord: {
            enabled: false,
            url: ''
        },
        webhookLogFilter: {
            enabled: false,
            mode: 'whitelist',
            levels: ['error'],
            keywords: ['Starting account', 'Points collected']
        }
    }
}

const ACCOUNTS_TEMPLATE = [
    '# Add one Microsoft account per block.',
    '# Optional: totpSecret, recoveryEmail, region, and proxy settings.',
    '#',
    '# email: user@example.com',
    '# password: your-password',
    ''
].join('\n')

function writePrivateJson(filePath: string, value: unknown): void {
    fs.writeFileSync(filePath, `${JSON.stringify(value, null, 4)}\n`, { mode: 0o600 })
    fs.chmodSync(filePath, 0o600)
}

export function runOnboarding(): void {
    const workspaceDirectory = getWorkspaceDirectory()
    const development =
        process.env.MRS_ENV === 'development' || process.argv.includes('-dev') || process.argv.includes('--dev')
    const configName = development ? 'config.dev.json' : 'config.json'
    const accountsName = development ? 'accounts.dev.txt' : 'accounts.txt'
    const configPath = path.join(workspaceDirectory, configName)
    const accountsPath = path.join(workspaceDirectory, accountsName)

    if (!fs.existsSync(configPath)) writePrivateJson(configPath, USER_CONFIG)

    if (!fs.existsSync(accountsPath)) {
        fs.writeFileSync(accountsPath, ACCOUNTS_TEMPLATE, { mode: 0o600 })
        fs.chmodSync(accountsPath, 0o600)
    }

    if (!isSeaBuild()) return
    fs.mkdirSync(path.join(workspaceDirectory, 'sessions'), { recursive: true })
    fs.mkdirSync(path.join(workspaceDirectory, 'diagnostics'), { recursive: true })
}

export function runOnboardingIfNeeded(): void {
    runOnboarding()
    if (process.env.MRS_SETUP_SKIPPED === '1') return
    if (process.env.MRS_ENV === 'development' || process.argv.includes('-dev') || process.argv.includes('--dev')) return

    const workspaceDirectory = getWorkspaceDirectory()
    const accountsPath = path.join(workspaceDirectory, 'accounts.txt')
    const contents = fs.existsSync(accountsPath) ? fs.readFileSync(accountsPath, 'utf8') : ''
    const hasAccount = /^email:\s*\S+@\S+/m.test(contents)
    if (hasAccount) return

    const setupPath = process.env.MRS_SETUP_PATH ?? path.join(process.cwd(), 'scripts', 'setup.js')
    if (!fs.existsSync(setupPath)) return
    const result = spawnSync(process.execPath, [setupPath], {
        stdio: 'inherit',
        env: {
            ...process.env,
            MRS_DATA_DIR: getWorkspaceDirectory().replace(/[\\\\/]workspace$/, '')
        }
    })
    if (result.status !== 0) process.exit(result.status ?? 1)
}
