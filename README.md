# Microsoft Rewards Script

A TypeScript application that runs Microsoft Rewards workflows in a browser. It supports multiple accounts, configurable activities, an optional local control API, and Discord, ntfy, or Telegram notifications.

> This is an independent project and is not affiliated with Microsoft. Review the Microsoft Rewards terms before use. Activity availability and results are not guaranteed.

## Requirements

- Node.js 24 or newer
- Windows, Linux, macOS, or WSL
- Internet access and Chromium installed through Patchright

## Quick Start

```bash
git clone https://github.com/lasbt/Microsoft-Rewards-Script.git
cd Microsoft-Rewards-Script
npm install
```

`npm install` installs the dependencies and Chromium automatically.

Run the setup wizard, then build and start the application:

```bash
npm run setup
npm run build
npm run start
```

Before starting, run `npm run doctor` to check Node.js, accounts, configuration,
and the installed Chromium browser. The command reports each prerequisite
without opening the application.

Production uses `workspace/accounts.txt` and `workspace/config.json`. Development
uses `workspace/accounts.dev.txt` and `workspace/config.dev.json`; create them
with `npm run setup:dev`. Test credentials stay separate from real accounts.

The wizard creates `workspace/config.json` and `workspace/accounts.txt`. Keep those files local; the workspace is ignored by Git. See the [usage and installation guide](docs/usage.md) and [configuration guide](docs/configuration.md).

## Commands

| Command                          | Purpose                                        |
| -------------------------------- | ---------------------------------------------- |
| `npm run build`                  | Compile TypeScript and copy query data         |
| `npm run build:bootstrapper`     | Build a small one-click launcher (requires Go) |
| `npm run start`                  | Run the application                            |
| `npm run dev`                    | Run TypeScript directly with debug logging     |
| `npm run manual-login -- --help` | Show manual login options                      |
| `npm run clear-sessions -- help` | Show session management commands               |
| `npm run open-session -- --help` | Open a saved session in Chromium               |
| `npm run api -- --help`          | Show control API options                       |
| `npm test`                       | Run tests                                      |
| `npm run lint`                   | Run ESLint                                     |

The optional API can start and monitor runs and maintain a local schedule while the API process is running. See the [control API guide](scripts/api/README.md).

For small one-click launchers, run `npm run build:bootstrapper`. It creates the
host executable and a Windows/amd64 executable under `dist/bootstrapper/`; on
first launch they use or download Node.js, install production dependencies and
the Chromium headless shell, and store runtime files under `workspace/.runtime/`.
The app bundle stays embedded in the executable and runs through the existing
`npm run start` command. See the
[bootstrapper guide](docs/node-sea.md) for build targets, Linux package
permissions, and privacy limitations.

## Documentation

The [documentation index](docs/README.md) links to installation, configuration, architecture, usage, and troubleshooting guides.

# Microsoft Rewards Script

Aplicação TypeScript para executar fluxos do Microsoft Rewards usando um
navegador. Mantém sessões para várias contas, oferece atividades configuráveis,
API local opcional e notificações por Discord, ntfy ou Telegram.

> Projeto independente, sem afiliação com a Microsoft. Consulte os termos do
> Microsoft Rewards antes de usar. O funcionamento e a disponibilidade das
> atividades não são garantidos.

## Requisitos

- Node.js 24 ou mais recente
- Windows, Linux, macOS ou WSL
- Acesso à internet e Chromium instalado pelo Patchright

## Início rápido

```bash
git clone https://github.com/lasbt/Microsoft-Rewards-Script.git
cd Microsoft-Rewards-Script
npm install
```

Execute o setup, compile e inicie o aplicativo:

```bash
npm run setup
npm run build
npm run start
```

O wizard cria `workspace/config.json` e `workspace/accounts.txt`. Mantenha esses arquivos locais; o workspace é ignorado pelo Git. Consulte [Instalação e uso](docs/usage.md) e [Configuração](docs/configuration.md).

## Comandos

| Comando                          | Finalidade                                            |
| -------------------------------- | ----------------------------------------------------- |
| `npm run build`                  | Compilar TypeScript e copiar dados de consulta        |
| `npm run build:bootstrapper`     | Criar launcher one-click (requer Go)                  |
| `npm run start`                  | Executar a aplicação                                  |
| `npm run dev`                    | Executar TypeScript diretamente com logs de depuração |
| `npm run manual-login -- --help` | Ver opções de login manual                            |
| `npm run clear-sessions -- help` | Ver comandos para gerenciar sessões                   |
| `npm run open-session -- --help` | Abrir sessão salva no Chromium                        |
| `npm run api -- --help`          | Ver opções da API de controle                         |
| `npm test`                       | Executar testes                                       |
| `npm run lint`                   | Executar ESLint                                       |

A API opcional pode iniciar e monitorar execuções e manter uma agenda local
enquanto o processo da API estiver ativo. Consulte o [guia da API](scripts/api/README.md).

Para criar um launcher one-click pequeno, execute `npm run build:bootstrapper`.
Ele gera executáveis para o sistema atual e Windows/amd64 em `dist/bootstrapper/`;
na primeira execução, eles usam ou baixam Node.js, instalam as dependências de
produção e o Chromium headless shell em `workspace/.runtime/`. O bundle da
aplicação fica embutido no executável e roda pelo comando existente `npm run start`.
Consulte o
[guia do bootstrapper](docs/node-sea.md) para alvos, permissões no Linux e limites de privacidade.

## Documentação

O [índice de documentação](docs/README.md) reúne instalação, configuração,
arquitetura, módulos, uso e solução de problemas.
