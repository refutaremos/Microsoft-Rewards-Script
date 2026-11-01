# Architecture

## Startup and Execution

`src/index.ts` checks the Node.js version, loads accounts from `.env`, reads and validates `config.json`, and creates the browser, login, activity, search, and logging components. Accounts run sequentially by default. The `clusters` option distributes accounts among Node.js workers; searches can also run separate mobile and desktop contexts in parallel.

Each run restores or creates the browser session, authenticates when necessary, loads dashboard state, runs enabled activities, updates points, and closes contexts in a `finally` block. Errors are logged per account when possible so later accounts can continue.

## Sessions and Files

- `sessions/sessions.db` stores mobile and desktop browser state and, optionally, saved fingerprints. `sessionPath` can change this location.
- `.env` contains credentials and account-specific options.
- `config.json` contains application preferences and is created from the example only when it does not exist.
- `config/schedule.json` stores the schedule persisted by the control API.
- `workspace/diagnostics/` receives error captures when diagnostics are enabled.

Sessions contain authentication cookies. Treat them as sensitive data, keep them private, and remove records only through the provided commands.

## API, Scheduling, and Shutdown

The optional API in `scripts/api/` runs as a separate Node.js process. It starts the compiled bot as a child process, parses structured logs, keeps status and history in memory, and provides HTTP and Server-Sent Events endpoints. API history is not stored in a database.

The API checks the schedule with a timer aligned to the minute. A match starts a normal bot run; no cron service or other scheduler process is used. The schedule persists across restarts but does not trigger while the API is stopped. Account exclusions are applied to the child process environment without changing the original account list.

Ctrl+C and SIGTERM initiate cleanup. The bot flushes notification queues and closes the session store; the API stops its child process and clears the timer. The API also provides explicit stop and restart endpoints.

## Modules and Maintenance

| Path                           | Responsibility                                           |
| ------------------------------ | -------------------------------------------------------- |
| `src/index.ts`                 | Lifecycle, account coordination, workers, and cleanup    |
| `src/browser/`                 | Chromium contexts, page actions, and authentication      |
| `src/functions/Activities.ts`  | Dispatch activities                                      |
| `src/functions/activities/`    | Rewards, app, search, and experimental workflows         |
| `src/functions/QueryEngine.ts` | Query external sources and build topics                  |
| `src/util/Load.ts`             | Read `.env`, load accounts, and initialize configuration |
| `src/util/Validator.ts`        | Schemas, defaults, and input validation                  |
| `src/util/SessionStore.ts`     | SQLite session persistence                               |
| `src/logging/`                 | Console output and notification queues                   |
| `scripts/main/`                | Manual login and session commands                        |
| `scripts/api/`                 | HTTP API, process execution, and scheduling              |

To add an application preference, update `APP_CONFIG` in `src/constants/runtimeConstants.ts`, its type in `src/interface/Config.ts`, and its schema in `src/util/Validator.ts`. User-facing overrides belong in `workspace/config.json`. Account fields belong in `workspace/accounts.txt` and are mapped in `src/util/Load.ts`. Change an activity in its group under `src/functions/activities/`; use `src/functions/Activities.ts` to dispatch it; and `src/index.ts` to adjust when it runs. Search sources belong in `src/functions/QueryEngine.ts`, and RSS feeds in `src/constants/rssFeeds.ts`.

# Arquitetura

## Inicialização e execução

`src/index.ts` verifica a versão do Node, carrega as contas de `.env`, lê e
valida `config.json` e cria os componentes de navegador, login, atividades,
busca e logs. Por padrão, as contas são executadas em sequência. A opção
`clusters` distribui as contas entre workers do Node.js; a busca também pode
usar contextos móveis e desktop separados em paralelo.

Cada execução restaura ou cria a sessão do navegador, autentica quando
necessário, carrega o estado do painel, executa atividades habilitadas, atualiza
os pontos e fecha os contextos em um bloco `finally`. Erros são registrados por
conta quando possível para permitir que as seguintes continuem.

## Sessões e arquivos

- `sessions/sessions.db` armazena o estado do navegador móvel e desktop e,
  opcionalmente, fingerprints salvas. `sessionPath` pode mudar esse local.
- `.env` contém credenciais e opções específicas das contas.
- `config.json` contém preferências e é criado do exemplo apenas se ausente.
- `config/schedule.json` contém agenda persistida pela API de controle.
- `workspace/diagnostics/` recebe capturas de erro quando os diagnósticos estão ativos.

As sessões contêm cookies de autenticação. Trate-as como dados sensíveis,
mantenha-as privadas e remova registros apenas por meio dos comandos próprios.

## API, agenda e cancelamento

A API opcional em `scripts/api/` é um processo Node separado. Ela inicia o bot
compilado como processo filho, interpreta logs estruturados, mantém status e
histórico em memória e oferece endpoints HTTP e Server-Sent Events. O histórico
da API não é gravado em banco.

A agenda é verificada por um timer alinhado ao minuto no processo da API. Uma
correspondência inicia uma execução normal do bot; não há cron ou outro processo
de agendamento. A agenda persiste após reinicializações, mas não dispara com a
API parada. Exclusões de contas são aplicadas ao ambiente do processo filho sem
alterar a lista original.

Ctrl+C e SIGTERM iniciam a limpeza. O bot descarrega filas de notificação e
fecha o armazenamento de sessão; a API interrompe o processo filho e limpa o
timer. A API também oferece endpoints explícitos para parar e reiniciar.

## Módulos e manutenção

| Caminho                        | Responsabilidade                                        |
| ------------------------------ | ------------------------------------------------------- |
| `src/index.ts`                 | Ciclo de vida, coordenação de contas, workers e limpeza |
| `src/browser/`                 | Contextos Chromium, ações de página e autenticação      |
| `src/functions/Activities.ts`  | Encaminha o trabalho às atividades                      |
| `src/functions/activities/`    | Fluxos Rewards, app, busca e atividades experimentais   |
| `src/functions/QueryEngine.ts` | Consulta fontes externas e monta tópicos                |
| `src/util/Load.ts`             | Lê `.env`, carrega contas e inicializa configuração     |
| `src/util/Validator.ts`        | Schemas, padrões e validação de entrada                 |
| `src/util/SessionStore.ts`     | Persistência SQLite das sessões                         |
| `src/logging/`                 | Console e filas de notificações                         |
| `scripts/main/`                | Login manual e comandos para sessões                    |
| `scripts/api/`                 | API HTTP, execução de processos e agenda                |

Para adicionar uma preferência, atualize `APP_CONFIG` em `src/constants/runtimeConstants.ts`, o tipo
em `src/interface/Config.ts` e o schema em `src/util/Validator.ts`. Campos de
conta pertencem a `workspace/accounts.txt` e são mapeados em `src/util/Load.ts`.
Altere uma atividade no seu grupo em `src/functions/activities/`; use
`src/functions/Activities.ts` para encaminhá-la e `src/index.ts` para ajustar
quando executa. Fontes de busca ficam em `src/functions/QueryEngine.ts` e feeds
RSS em `src/constants/rssFeeds.ts`. Mantenha os fluxos nas responsabilidades
existentes e evite wrappers para chamadas únicas.
