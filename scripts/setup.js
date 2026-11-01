import fs from 'node:fs'
import path from 'node:path'
import readline from 'node:readline/promises'
import { stdin as input, stdout as output } from 'node:process'

const runtimeConstants = process.env.MRS_RUNTIME_CONSTANTS
    ? JSON.parse(process.env.MRS_RUNTIME_CONSTANTS)
    : await import('../src/constants/runtimeConstants.ts')
const BANNER = `${runtimeConstants.ASCII_ART.join('\n')}\n`
let activeInterface

function validEmail(email) {
    return /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)
}

function developmentMode() {
    return process.env.MRS_ENV === 'development' || process.argv.includes('-dev') || process.argv.includes('--dev')
}

function workspaceFileName(filename) {
    if (!developmentMode()) return filename
    const extension = path.extname(filename)
    return `${filename.slice(0, -extension.length)}.dev${extension}`
}

function readUserConfigDefault() {
    return { webhook: runtimeConstants.APP_CONFIG.webhook }
}

function readAccounts(filePath) {
    if (!fs.existsSync(filePath)) return []
    return fs
        .readFileSync(filePath, 'utf8')
        .replace(/\r/g, '')
        .split(/\n\s*\n/)
        .map(block => block.trim())
        .filter(block => block && !block.startsWith('#'))
        .map(block => {
            const values = {}
            for (const line of block.split('\n')) {
                const separator = line.indexOf(':')
                if (separator > 0) values[line.slice(0, separator).trim()] = line.slice(separator + 1).trim()
            }
            return values
        })
        .filter(account => account.email && account.password)
}

function writeAccounts(filePath, accounts) {
    const text = accounts
        .map(account =>
            [
                `email: ${account.email}`,
                `password: ${account.password}`,
                ...(account.totpSecret ? [`totpSecret: ${account.totpSecret}`] : [])
            ].join('\n')
        )
        .join('\n\n')
    fs.writeFileSync(filePath, text ? `${text}\n` : '', { mode: 0o600 })
    fs.chmodSync(filePath, 0o600)
}

function printSummary(accounts, config) {
    const discord = config.webhook?.discord
    output.write('\nCurrent setup:\n')
    output.write(`Accounts: ${accounts.length}\n`)
    accounts.forEach((account, index) => {
        output.write(`  ${index + 1}. ${account.email}\n`)
    })
    output.write(`Webhook: ${discord?.enabled && discord.url ? 'configured' : 'disabled'}\n`)
    output.write('\n')
}

function printHeader() {
    output.write(`${BANNER}\n`)
}

function clearScreen() {
    output.write('\x1Bc')
}

function printScreen(title) {
    clearScreen()
    printHeader()
    if (title) output.write(`\n${title}\n`)
}

async function waitToReturn(rl) {
    await rl.question('\nPress Enter to return to the menu: ')
}

async function validateDiscordWebhook(url) {
    let parsed
    try {
        parsed = new URL(url)
    } catch {
        return false
    }
    if (
        parsed.protocol !== 'https:' ||
        !['discord.com', 'discordapp.com', 'canary.discord.com'].includes(parsed.hostname)
    ) {
        return false
    }

    const controller = new AbortController()
    const timeout = setTimeout(() => controller.abort(), 10000)
    try {
        const response = await fetch(`${parsed.origin}${parsed.pathname}${parsed.search}`, {
            method: 'POST',
            headers: { 'content-type': 'application/json' },
            body: JSON.stringify({ content: 'Cookie is now using this webhook!' }),
            signal: controller.signal
        })
        return response.ok || response.status === 204
    } catch {
        return false
    } finally {
        clearTimeout(timeout)
    }
}

async function promptSecret(rl, question) {
    if (!input.isTTY || typeof input.setRawMode !== 'function') {
        return rl.question(question)
    }

    rl.pause()
    output.write(question)
    input.setRawMode(true)
    input.resume()
    input.setEncoding('utf8')

    let value = ''
    return new Promise((resolve, reject) => {
        const cleanup = () => {
            input.setRawMode(false)
            input.pause()
            input.removeListener('data', onData)
        }
        const onData = chunk => {
            for (const character of chunk) {
                if (character === '\r' || character === '\n') {
                    cleanup()
                    output.write('\n')
                    rl.resume()
                    resolve(value)
                    return
                }
                if (character === '\u0003') {
                    cleanup()
                    rl.resume()
                    reject(new Error('Setup cancelled'))
                    return
                }
                if (character === '\u007f' || character === '\b') {
                    value = value.slice(0, -1)
                    continue
                }
                if (character >= ' ') value += character
            }
        }
        input.on('data', onData)
    })
}

async function main() {
    const dataDirectory = process.env.MRS_DATA_DIR ? path.resolve(process.env.MRS_DATA_DIR) : process.cwd()
    const workspaceDirectory = path.join(dataDirectory, 'workspace')
    const accountsPath = path.join(workspaceDirectory, workspaceFileName('accounts.txt'))
    const configPath = path.join(workspaceDirectory, workspaceFileName('config.json'))
    fs.mkdirSync(workspaceDirectory, { recursive: true })

    if (!fs.existsSync(configPath)) {
        fs.writeFileSync(configPath, `${JSON.stringify(readUserConfigDefault(), null, 4)}\n`, { mode: 0o600 })
    }

    const rl = readline.createInterface({ input, output })
    activeInterface = rl
    const accounts = readAccounts(accountsPath)

    let config
    try {
        config = JSON.parse(fs.readFileSync(configPath, 'utf8'))
    } catch {
        config = readUserConfigDefault()
    }

    while (true) {
        printScreen()
        printSummary(accounts, config)
        output.write('1. Add account\n2. Remove account\n3. Configure webhook\n4. Show summary\n5. Save and start\n')
        const choice = (await rl.question('Choose an option: ')).trim()

        if (choice === '1') {
            printScreen('Add account')
            const email = (await rl.question('Email: ')).trim()
            if (!validEmail(email)) {
                output.write('Enter a valid email address.\n')
                await waitToReturn(rl)
                continue
            }
            if (accounts.some(account => account.email.toLowerCase() === email.toLowerCase())) {
                output.write('That account is already configured.\n')
                await waitToReturn(rl)
                continue
            }
            const password = (await promptSecret(rl, 'Password: ')).trim()
            if (!password) {
                output.write('Password cannot be empty.\n')
                await waitToReturn(rl)
                continue
            }
            const totpSecret = (await rl.question('TOTP secret (optional): ')).trim()
            accounts.push({ email, password, ...(totpSecret ? { totpSecret } : {}) })
            output.write(`Added ${email}.\n`)
            await waitToReturn(rl)
        } else if (choice === '2') {
            printScreen('Remove account')
            if (!accounts.length) {
                output.write('There are no accounts to remove.\n')
                await waitToReturn(rl)
                continue
            }
            printSummary(accounts, config)
            const selection = Number.parseInt(await rl.question('Account number to remove: '), 10)
            if (!Number.isInteger(selection) || selection < 1 || selection > accounts.length) {
                output.write('Enter a valid account number.\n')
                await waitToReturn(rl)
                continue
            }
            const [removed] = accounts.splice(selection - 1, 1)
            output.write(`Removed ${removed.email}.\n`)
            await waitToReturn(rl)
        } else if (choice === '3') {
            printScreen('Configure webhook')
            while (true) {
                const webhookUrl = (await rl.question('Discord webhook URL (leave empty to disable): ')).trim()
                if (!webhookUrl) {
                    config.webhook ??= {}
                    config.webhook.discord = { enabled: false, url: '' }
                    output.write('Webhook disabled.\n')
                    break
                }
                output.write('Testing webhook...\n')
                if (await validateDiscordWebhook(webhookUrl)) {
                    config.webhook ??= {}
                    config.webhook.discord = { enabled: true, url: webhookUrl }
                    output.write('Webhook is valid. Test message sent.\n')
                    break
                }
                output.write('Webhook is invalid or unreachable. Please try again.\n')
            }
            await waitToReturn(rl)
        } else if (choice === '4') {
            printScreen('Setup summary')
            printSummary(accounts, config)
            await waitToReturn(rl)
        } else if (choice === '5') {
            if (!accounts.length) {
                output.write('Add at least one account before starting.\n')
                await waitToReturn(rl)
                continue
            }
            printScreen('Save and start')
            break
        } else {
            printScreen('Invalid option')
            output.write('Enter a number from 1 to 5.\n')
            await waitToReturn(rl)
        }
    }

    rl.close()
    activeInterface = undefined
    writeAccounts(accountsPath, accounts)
    fs.writeFileSync(configPath, `${JSON.stringify(config, null, 4)}\n`, { mode: 0o600 })
    fs.chmodSync(configPath, 0o600)
    output.write('Setup complete. Starting Microsoft Rewards Script...\n')
}

main().catch(error => {
    activeInterface?.close()
    activeInterface = undefined
    if (error instanceof Error && ['Aborted with Ctrl+C', 'Setup cancelled'].includes(error.message)) {
        process.exitCode = 130
        return
    }
    console.error(error instanceof Error ? error.message : String(error))
    process.exitCode = 1
})
