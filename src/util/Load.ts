import fs from 'fs'
import path from 'path'
import process from 'node:process'
import { randomBytes } from 'node:crypto'

import type { Account } from '../interface/Account'
import type { Config } from '../interface/Config'
import { APP_CONFIG } from '../constants/runtimeConstants'
import { validateAccounts, validateConfig } from './Validator'
import { getDataDirectory, getWorkspaceDirectory, isSeaBuild } from './AppPaths'

let configCache: Config
let envLoaded = false

function getProjectRoot(): string {
    if (process.env.MRS_ROOT_DIR) return path.resolve(process.env.MRS_ROOT_DIR)
    if (isSeaBuild()) return getDataDirectory()
    const cwd = process.cwd()
    if (fs.existsSync(path.join(cwd, 'package.json'))) return cwd

    let dir = __dirname
    while (dir !== path.parse(dir).root) {
        if (fs.existsSync(path.join(dir, 'package.json'))) return dir
        dir = path.dirname(dir)
    }

    return cwd
}

// Check root -> dist -> src (not in dist, but root)
function resolveProjectFile(filename: string): string | undefined {
    const root = getProjectRoot()
    const candidates = [
        path.join(process.cwd(), filename),
        path.join(root, filename),
        path.join(root, 'dist', filename),
        path.join(root, 'src', filename)
    ]
    return candidates.find(p => fs.existsSync(p))
}

function ensureEnvLoaded(): void {
    if (envLoaded) return
    envLoaded = true

    const envFile = resolveProjectFile('.env')
    if (envFile) process.loadEnvFile(envFile)
}

function envStr(key: string): string | undefined {
    const v = process.env[key]
    if (v === undefined) return undefined
    const trimmed = v.trim()
    return trimmed.length ? trimmed : undefined
}

function isDevelopmentMode(): boolean {
    return process.env.MRS_ENV === 'development' || process.argv.includes('-dev') || process.argv.includes('--dev')
}

export function workspaceFileName(filename: string): string {
    if (!isDevelopmentMode()) return filename
    const extension = path.extname(filename)
    return `${filename.slice(0, -extension.length)}.dev${extension}`
}

function isRecord(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function mergeConfig(base: unknown, override: unknown): unknown {
    if (!isRecord(base) || !isRecord(override)) return override
    const merged = { ...base }
    for (const [key, value] of Object.entries(override)) {
        merged[key] = key in base ? mergeConfig(base[key], value) : value
    }
    return merged
}

function resolveAccountsFile(filename: string): string {
    return path.isAbsolute(filename) ? filename : path.resolve(getProjectRoot(), filename)
}

function ensureWorkspaceConfig(): string {
    const workspaceDirectory = getWorkspaceDirectory()
    fs.mkdirSync(workspaceDirectory, { recursive: true })
    const configPath = path.join(workspaceDirectory, workspaceFileName('config.json'))
    if (!fs.existsSync(configPath)) {
        const development = workspaceFileName('config.json').includes('.dev.')
        const examplePath = path.join(workspaceDirectory, 'config.json')
        const seededConfig = development
            ? APP_CONFIG
            : fs.existsSync(examplePath)
              ? JSON.parse(fs.readFileSync(examplePath, 'utf8'))
              : APP_CONFIG
        fs.writeFileSync(configPath, `${JSON.stringify(seededConfig, null, 4)}\n`, { mode: 0o600 })
        return configPath
    }
    return configPath
}

function ensureDiagnosticsPassword(): string {
    const passwordPath = path.join(getProjectRoot(), 'workspace', '.diagnostics-password')
    try {
        const password = fs.readFileSync(passwordPath, 'utf8').trim()
        if (password.length >= 32) return password
    } catch {
        // Create a new password for installations without one.
    }

    const password = randomBytes(32).toString('base64url')
    fs.writeFileSync(passwordPath, `${password}\n`, { encoding: 'utf8', mode: 0o600 })
    fs.chmodSync(passwordPath, 0o600)
    return password
}

function resolveWorkspaceAccountsFile(filename?: string): string | undefined {
    if (filename) return resolveAccountsFile(filename)
    return path.join(getProjectRoot(), 'workspace', workspaceFileName('accounts.txt'))
}

export function parseAccountsText(text: string): unknown[] {
    const blocks: string[] = []
    let current: string[] = []

    for (const rawLine of text.replace(/\r/g, '').split('\n')) {
        if (!rawLine.trim()) {
            if (current.length) {
                blocks.push(current.join('\n'))
                current = []
            }
            continue
        }
        if (rawLine.trimStart().startsWith('#')) continue
        current.push(rawLine)
    }
    if (current.length) blocks.push(current.join('\n'))

    return blocks.map((block, index) => {
        const account: Record<string, unknown> = { password: '' }
        const proxy: Record<string, unknown> = {
            proxyHttp: false,
            url: '',
            port: 0,
            password: '',
            username: ''
        }
        for (const line of block.split('\n')) {
            const separator = line.indexOf(':')
            if (separator < 1) throw new Error(`Invalid account setting on line ${index + 1}: ${line}`)
            const key = line.slice(0, separator).trim()
            const value = line.slice(separator + 1).trim()
            if (
                key === 'email' ||
                key === 'password' ||
                key === 'totpSecret' ||
                key === 'recoveryEmail' ||
                key === 'region'
            ) {
                account[key] = value
                continue
            }
            if (key === 'proxyHttp') {
                proxy.proxyHttp = value.toLowerCase() === 'true'
                continue
            }
            if (key === 'proxyUrl') {
                proxy.url = value
                continue
            }
            if (key === 'proxyPort') {
                proxy.port = value ? Number(value) : 0
                continue
            }
            if (key === 'proxyUsername') {
                proxy.username = value
                continue
            }
            if (key === 'proxyPassword') {
                proxy.password = value
                continue
            }
            throw new Error(`Unknown account setting: ${key}`)
        }
        if (typeof account.email !== 'string' || !account.email) {
            throw new Error(`Account block ${index + 1} must start with email: ...`)
        }
        if (Object.keys(proxy).length) account.proxy = proxy
        return account
    })
}
export function loadAccounts(): Account[] {
    try {
        ensureEnvLoaded()

        const accountsFile = resolveWorkspaceAccountsFile(envStr('ACCOUNTS_FILE'))
        if (accountsFile && fs.existsSync(accountsFile)) {
            return validateAccounts(parseAccountsText(fs.readFileSync(accountsFile, 'utf8')))
        }

        throw new Error('workspace/accounts.txt was not found. Run npm run setup to add an account.')
    } catch (error) {
        if (error instanceof Error) throw error
        throw new Error(String(error))
    }
}

export function loadConfig(): Config {
    try {
        if (configCache) return configCache
        ensureEnvLoaded()
        const configPath = ensureWorkspaceConfig()

        const overrides: unknown = JSON.parse(fs.readFileSync(configPath, 'utf8'))
        const envConfig = envStr('BOT_CONFIG')
        let botOverrides: unknown = {}
        if (envConfig) {
            let parsed: unknown
            try {
                parsed = JSON.parse(envConfig)
            } catch (error) {
                throw new Error(`BOT_CONFIG must contain valid JSON: ${error instanceof Error ? error.message : error}`)
            }
            if (!isRecord(parsed)) throw new Error('BOT_CONFIG must be a JSON object.')
            botOverrides = parsed
        }

        const userOverrides = isRecord(overrides) ? overrides : {}
        const mergedConfig = mergeConfig(mergeConfig(APP_CONFIG, botOverrides), userOverrides)
        if (process.env.MRS_HEADLESS_ONLY === '1' && isRecord(mergedConfig) && mergedConfig.headless === false) {
            throw new Error(
                'This bootstrapper includes headless Chromium only. Set "headless": true in workspace/config.json.'
            )
        }
        configCache = validateConfig({
            ...(isRecord(mergedConfig) ? mergedConfig : {}),
            diagnosticsPassword: ensureDiagnosticsPassword(),
            sessionPath: APP_CONFIG.sessionPath,
            headless: APP_CONFIG.headless,
            clusters: APP_CONFIG.clusters
        })
        return configCache
    } catch (error) {
        if (error instanceof Error) throw error
        throw new Error(String(error))
    }
}
