import { ASCII_ART } from '../constants/runtimeConstants'

export const DASHBOARD_ART = ASCII_ART.join('\n')

export interface DashboardLayoutState {
    mode: 'demo' | 'live' | 'app'
    appStatus: string
    runId: string
    startedAt: string
    startedAtMonotonic: number
    accountsSeen: number
    accountsTotal: number | null
    pointsGained: number
    currentAccount: string | null
    stage: string
    error: { code: string; message: string; logPath: string | null; details?: string } | null
    events?: { level: string; text: string }[]
}

function fit(text: unknown, width: number): string {
    const value = String(text ?? '').replace(/[\r\n\t]/g, ' ')
    return value.length > width ? `${value.slice(0, Math.max(0, width - 3))}...` : value
}

function wrap(text: unknown, width: number): string[] {
    const lines: string[] = []
    const paragraphs = String(text ?? '')
        .replace(/\r/g, '')
        .split('\n')

    for (const paragraph of paragraphs) {
        const words = paragraph.trim().split(/\s+/).filter(Boolean)
        if (!words.length) {
            lines.push('')
            continue
        }

        let line = ''
        for (const word of words) {
            const candidate = line ? `${line} ${word}` : word
            if (line && candidate.length > width) {
                lines.push(line)
                line = word
            } else {
                line = candidate
            }
        }
        if (line) lines.push(line)
    }

    return lines
}

function cyan(text: string): string {
    return `\u001b[36m${text}\u001b[39m`
}

function elapsed(startedAtMonotonic: number): string {
    const seconds = Math.max(0, Math.floor((performance.now() - startedAtMonotonic) / 1000))
    return `${Math.floor(seconds / 60)}m ${String(seconds % 60).padStart(2, '0')}s`
}

export function maskEmail(email: string | null): string {
    if (!email) return 'Waiting for account'
    const [local = '', domain = ''] = email.split('@', 2)
    if (!domain) return maskSegment(local)

    const lastDot = domain.lastIndexOf('.')
    if (lastDot < 0) return `${maskSegment(local)}@${maskSegment(domain)}`

    const domainName = domain.slice(0, lastDot)
    const suffix = domain.slice(lastDot)
    return `${maskSegment(local)}@${maskSegment(domainName)}${suffix}`
}

function maskSegment(value: string): string {
    const visibleLength = Math.min(2, value.length)
    const visible = value.slice(0, visibleLength)
    return `${visible}${'*'.repeat(Math.max(0, value.length - visibleLength))}`
}

function row(label: string, value: unknown, width: number): string {
    const labelWidth = 16
    return `${label.padEnd(labelWidth)}${fit(value, width - labelWidth)}`
}

export function renderDashboard(state: DashboardLayoutState, width = 80, color = process.stdout.isTTY): string {
    const screenWidth = Math.max(40, Math.min(100, width))
    const accountCount = state.accountsTotal ?? state.accountsSeen
    const art = color && !process.env.NO_COLOR ? cyan(DASHBOARD_ART) : DASHBOARD_ART

    if (state.error) {
        const lines = [...art.split('\n'), '', ...wrap(`Error: ${state.error.message}`, screenWidth)]
        if (process.argv.includes('-dev') && state.error.details) {
            lines.push('', ...wrap(state.error.details, screenWidth))
        } else {
            lines.push(...wrap(`Reference: ${state.error.code}`, screenWidth))
        }
        lines.push('', 'Press Q to quit')
        return lines.join('\n')
    }

    const currentAccount =
        state.mode === 'demo' && !state.currentAccount
            ? 'Waiting for account activity'
            : maskEmail(state.currentAccount)
    const lines = [
        ...art.split('\n'),
        '',
        ...(state.mode === 'demo' ? ['DEMO - DUMMY DATA', ''] : []),
        row('Run', state.runId, screenWidth),
        row('Runtime', elapsed(state.startedAtMonotonic), screenWidth),
        row('Accounts', accountCount, screenWidth),
        ...(state.pointsGained > 0 ? [row('Points earned', `+${state.pointsGained}`, screenWidth)] : []),
        row('Current', currentAccount, screenWidth),
        row('Activity', state.stage, screenWidth),
        '',
        'Press Q to quit'
    ]

    return lines.join('\n')
}
