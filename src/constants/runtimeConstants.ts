export const APP_CONFIG = {
    sessionPath: 'workspace/sessions',
    headless: true,
    clusters: 1,
    errorDiagnostics: true,
    ensureStreakProtection: true,
    autoClaimPunchcardRewards: false,
    continueOnBotWarning: false,
    skipNonPointTasks: true,
    accountDelay: {
        min: '1min',
        max: '3min'
    },
    workers: {
        doDailySet: true,
        doClaimBonusPoints: true,
        doMorePromotions: true,
        doPunchCards: true,
        doAppPromotions: true,
        doDesktopSearch: true,
        doMobileSearch: true,
        doBonusSearches: false,
        doDailyCheckIn: true,
        doReadToEarn: true,
        doActivateSearchPerk: true,
        doVisualSearch: false
    },
    activities: {
        urlReward: true,
        searchOnBing: true
    },
    globalTimeout: '30sec',
    searchSettings: {
        maxBonusSearches: 110,
        parallelSearching: true,
        clusterSearch: true,
        queryEngines: [
            'google',
            'wikipedia',
            'wikirandom',
            'hackernews',
            'reddit',
            'rss.googleTrends',
            'rss.googleNews',
            'rss.bbc',
            'rss.guardian.news',
            'rss.guardian.theverge'
        ],
        searchResultVisitTime: '10sec',
        searchDelay: {
            min: '1min',
            max: '2min'
        },
        readDelay: {
            min: '1sec',
            max: '3sec'
        }
    },
    experimental: {
        apiSearch: false,
        apiSearchOnBing: false,
        blockMedia: false,
        edgeBrowsing: false
    },
    debugLogs: false,
    consoleLogFilter: {
        enabled: false,
        mode: 'whitelist',
        levels: ['error', 'warn'],
        keywords: ['starting account', 'Fraud_UserWarning_BotScore_UX']
    },
    proxy: {
        queryEngine: true,
        ignoreCertificateErrors: false
    },
    webhook: {
        discord: {
            enabled: false,
            url: ''
        },
        webhookLogFilter: {
            enabled: false,
            mode: 'whitelist',
            levels: ['error'],
            keywords: ['starting account', 'collected']
        }
    }
} as const

export const ASCII_ART = [
    '                                        ',
    '                    ,--.    ,--.        ',
    " ,---. ,---.  ,---. |  |,-. `--' ,---.  ",
    "| .--'| .-. || .-. ||     / ,--.| .-. : ",
    "\\ `--.' '-' '' '-' '|  \\  \\ |  |\\   --. ",
    " `---' `---'  `---' `--'`--'`--' `----' ",
    '                                        ',
    '                                        '
] as const
