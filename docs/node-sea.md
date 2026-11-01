# Bootstrapper Distribution

The Go bootstrapper provides a small, single-file launcher while preserving the
normal application command, `npm run start`. It embeds the compiled app and
setup scripts in the executable. On first launch it prepares Node, production
dependencies, and the browser under `workspace/.runtime/` beside the executable.
The TypeScript app is compiled into one `bundle.js` resource, kept inside the
executable and piped to `node -` through `npm run start`; it is never unpacked as
a standalone file. This is a launcher, not a fully self-contained offline
executable.

## Build

Install Go, Node.js 24+, and the project dependencies, then run:

```bash
npm install
npm run build:bootstrapper
```

By default, the builder emits the host executable and Windows/amd64 (unless
the host is already Windows). To build only one target, pass:

```bash
npm run build:bootstrapper -- --target=windows/amd64
npm run build:bootstrapper -- --target=linux/amd64
npm run build:bootstrapper -- --target=darwin/arm64
```

Artifacts are written to `dist/bootstrapper/`. The builder fails if a
bootstrapper exceeds 50 MiB. Go and npm are build-time requirements; end users
do not need Go.

## First Launch

The executable looks for Node.js 24+ and npm in the system PATH, then checks
`workspace/.runtime/node/` before downloading the official Node.js 24 runtime
with SHA-256 verification. It reuses existing production `node_modules` when
the production lock entries match, including compatible installs from older
bootstrapper builds, and keeps the browser in the same stable workspace cache.
It runs `npm ci --omit=dev` only when dependencies are absent or incompatible,
then starts the existing `npm run start` flow with the embedded bundle on
stdin. The app's onboarding prompts for account setup when needed. Use `--setup`
to open the setup menu explicitly.
This production launcher installs only Patchright's `chromium-headless-shell`;
a config with `headless: false` is rejected with a clear error. Use the source
installation if you need a visible browser.

The first launch needs internet access to obtain Node.js (when needed), npm
packages, and Patchright's headless shell. This app launches Chromium only; it
does not use Firefox, WebKit, or browser video recording. On Linux, Patchright
checks system libraries for the shell and installs missing packages through
its supported installer; this may prompt for `sudo`. If package installation
fails, the launcher exits with the underlying failure instead of continuing to
a browser launch error.

The executable creates `workspace/` beside itself. Account/configuration/session
data live there, and Node, npm's cache, Chromium, temporary files, and the app
dependencies, the headless shell, npm cache, and temporary files live below
`workspace/.runtime/`. Old full Chromium and FFmpeg directories in this dedicated
cache are removed. The launcher does not use `LocalAppData` or the OS user cache
for app files. Place the executable in a writable directory. The compact runtime
supports the main bot and setup commands; API, dashboard, and session-maintenance
CLIs remain available from a source checkout, but are not included in this
launcher payload.

## Privacy and Limits

The executable is small because Node, production dependencies, and the headless
shell are downloaded or reused at runtime. The app bundle, package manifest,
lockfile, and setup scripts are embedded; manifests are materialized only while
npm installs or starts the app, then removed. The bundle is piped to Node instead
of written as a file, but the executable can still be reverse-engineered and
external Node/dependency/browser files in `workspace/.runtime/` remain writable
by the user. For production distribution, sign the executable and do not embed
credentials or account configuration.
