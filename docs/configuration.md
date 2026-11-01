# Configuration

Application preferences are maintained internally by the bot. `workspace/config.json` is for user integrations, currently the Discord webhook.

Account-specific data lives in `workspace/accounts.txt` and is written by `npm run setup`. The workspace directory is Git ignored; do not commit passwords, TOTP secrets, API tokens, or webhook credentials.

The password for protected login and error dumps is generated locally at runtime and stored in `workspace/.diagnostics-password`. Each dump is saved as one `workspace/diagnostics/<reference-id>.rrdump` ZIP archive containing `dump.html`, `screenshot.png`, and `error.txt`. Error dumps are enabled by default with `errorDiagnostics: true`.

## Accounts

Add one block per account in `workspace/accounts.txt`. Separate multiple accounts with a blank line.

```text
email: person@example.com
password: account-password
totpSecret: base32-secret

email: second@example.com
password: another-password
```

Use `npm run setup` to create or append accounts safely. Passwords stay in the local workspace directory, which is Git ignored.

## Application Options

Keep these options in `config.json`; there are no `CONFIG_*` environment variable overrides.

| Section                                             | Purpose                                                               |
| --------------------------------------------------- | --------------------------------------------------------------------- |
| Application internals                               | Storage, browser visibility, concurrency, and timeouts                |
| `workers`                                           | Enable or disable individual workflows                                |
| `activities`                                        | URL Reward and Explore on Bing activities                             |
| `searchSettings`                                    | Query sources, intervals, result interaction, and concurrency         |
| `experimental`                                      | Optional API search, media blocking, and Edge browsing                |
| `proxy`                                             | Proxy use for queries and TLS handling for trusted inspection proxies |
| `consoleLogFilter`, `debugLogs`, `errorDiagnostics` | Console output and diagnostics                                        |
| `webhook`                                           | Discord, ntfy, Telegram, and notification filters                     |

Durations accept numeric milliseconds or strings such as `30sec` and `1min`. For intervals, `max` must be greater than or equal to `min`. Query sources include `google`, `wikipedia`, `wikirandom`, `hackernews`, `reddit`, and configured RSS feeds. Queries come from these external sources; if none respond, search is skipped rather than using a local fallback list. Internal defaults are in [`src/constants/runtimeConstants.ts`](../src/constants/runtimeConstants.ts).

The distributed example sets `headless` to `false`. Use headless mode on servers without a graphical environment.

## API and Scheduling

API options are environment variables documented in the [API guide](../scripts/api/README.md). API configuration and schedule changes are disabled by default. The schedule is stored in `config/schedule.json`; the API process checks it once per minute using the system's local time zone or the `TZ` value. Keep the API running to trigger scheduled runs. Existing runs are not overlapped.

# Configuração

As preferências do aplicativo ficam em `workspace/config.json`. Se o arquivo não existir, a aplicação o cria a partir dos padrões internos e valida as opções ao iniciar.

Os dados das contas ficam em `workspace/accounts.txt` e são criados pelo comando `npm run setup`. O diretório workspace é ignorado pelo Git. Não versione senhas, segredos, tokens da API ou credenciais de webhook.

## Contas

Adicione um bloco por conta em `workspace/accounts.txt`. Separe contas múltiplas com uma linha vazia.

```text
email: pessoa@example.com
password: senha-da-conta

email: segunda@example.com
password: outra-senha
```

Use `npm run setup` para criar ou adicionar contas. Senhas ficam no workspace local, que é ignorado pelo Git.

## Opções do aplicativo

Mantenha essas opções em `config.json`; não há overrides `CONFIG_*` por
variáveis de ambiente.

| Seção                                                                  | Finalidade                                                                      |
| ---------------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| `sessionPath`, `headless`, `clusters`, `accountDelay`, `globalTimeout` | Armazenamento, visibilidade do navegador, concorrência e tempos limite          |
| `workers`                                                              | Ativar ou desativar fluxos individuais                                          |
| `activities`                                                           | Atividades de URL Reward e Explore on Bing                                      |
| `searchSettings`                                                       | Fontes de consulta, intervalos, interação com resultados e concorrência         |
| `experimental`                                                         | Busca por API, bloqueio de mídia e navegação Edge opcionais                     |
| `proxy`                                                                | Uso de proxy nas consultas e tratamento TLS para proxies de inspeção confiáveis |
| `consoleLogFilter`, `debugLogs`, `errorDiagnostics`                    | Saída do console e diagnósticos                                                 |
| `webhook`                                                              | Destinos Discord, ntfy, Telegram e filtros de notificação                       |

Durações aceitam milissegundos numéricos ou strings como `30sec` e `1min`. Em
intervalos, `max` deve ser maior ou igual a `min`. Fontes de consulta incluem
`google`, `wikipedia`, `wikirandom`, `hackernews`, `reddit` e feeds RSS
configurados. As consultas vêm dessas fontes externas; se nenhuma responder, a
execução de busca é ignorada, sem lista local de reserva. Os valores padrão estão em
[`src/constants/runtimeConstants.ts`](../src/constants/runtimeConstants.ts).

O padrão distribuído define `headless` como `true`. Use o modo gráfico no workspace quando precisar acompanhar a janela do navegador.

## API e agenda

As opções da API estão documentadas no [guia da API](../scripts/api/README.md). Alterações de configuração e agenda
pela API ficam desativadas por padrão. A agenda é armazenada em
`config/schedule.json`; o processo da API a verifica uma vez por minuto no fuso
local do sistema ou no valor de `TZ`. Mantenha a API em execução para disparar
agendamentos. Execuções existentes não são sobrepostas.
