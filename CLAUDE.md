# koa — working notes

koa is a Wails desktop app for installing binaries published as GitHub releases
by repos carrying the `koa` topic. `PRD/PRD.md` is the requirements document and
`PRD/UI/koa.dc.html` is the **normative** visual and interaction spec.

## Precedence

- On presentation — layout, colour, type, spacing, chrome, hover behaviour —
  `PRD/UI/koa.dc.html` wins.
- On functionality — what an action does, which API is called, what is stored —
  `PRD/PRD.md` wins.
- The reference is a Claude Design canvas document (`<x-dc>`, `sc-if`, `sc-for`,
  `DCLogic`). None of that ships. Its sample repos, versions and log lines are
  placeholders, not fixtures.

## Architecture

All logic lives in pure-Go packages under `internal/`, unit-tested without a
GUI. The Wails layer is a thin shell:

- `internal/app` — the service bound to the frontend. `Service` carries only
  methods that are safe to expose over the JS bridge; lifecycle hooks live on
  `Shell` (`shell.go`) so Wails does not bind them.
- `internal/app.Host` — the interface the shell implements (emit, open URL,
  show window, quit). It is what lets `internal/app` compile and be tested
  without the GTK/WebKit toolchain.
- `main.go` — window, tray, close behaviour, and the `Koa` / `Window` bindings.

The frontend deliberately does **not** import the generated `frontend/wailsjs`
folder. `src/lib/bridge.ts` calls `window.go.main.Koa.*` through hand-written
typed wrappers, so `npm run build` type-checks on its own and a dev mock can be
swapped in.

## Conventions

- Design values from the reference are Tailwind theme tokens in
  `frontend/src/styles.css`, not per-component literals. `:root` is the light
  theme derived per PRD §5.5; `.dark` is transcribed from the reference.
- Square corners everywhere. Borders, never shadows. No filled buttons.
- Go: every exported symbol has a doc comment; comments explain *why*, and cite
  the PRD section when a rule comes from it.
- Platform-specific code goes in `_windows.go` / `_unix.go` files, never
  `runtime.GOOS` branches inside shared logic. Cross-check with
  `GOOS=windows go build ./internal/...`.

## Commands

```sh
go test ./...                       # backend
cd frontend && npm run build        # type-check + bundle
GOOS=windows go build ./internal/…  # cross-check the Windows files
go run ./build/icons/gen.go         # regenerate tray and app icons

wails build -tags webkit2_41        # real desktop build (needs the deps below)
wails dev -tags webkit2_41
cd frontend && npm run dev          # UI alone in a browser, against src/lib/mock.ts
```

## Build dependencies

A Wails build needs cgo and the GTK/WebKit headers. Without them `go build`
"succeeds" but produces a binary that exits with *"Wails applications will not
build without the correct build tags"* — it is not a working app.

```sh
sudo apt-get install -y build-essential libgtk-3-dev libwebkit2gtk-4.1-dev
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Use `-tags webkit2_41` on distributions that ship webkit2gtk-4.1 (Ubuntu 24.04+);
drop it for 4.0.

## Device Flow

Device Flow needs an OAuth App Client ID, which is a manual prerequisite
(PRD §19). Set `KOA_GITHUB_CLIENT_ID` in the environment or bake it in with
`-ldflags "-X main.githubClientID=…"`. Without one, koa still works through
Settings › *Enter token*.

## Self-update

koa checks its own latest release against `main.selfUpdateRepo`
(`-ldflags "-X main.selfUpdateRepo=owner/koa"`, or `KOA_SELF_UPDATE_REPO` in
the environment) and, when newer, lets the user trigger an in-app update from
Settings. `release.yml` injects the repo that actually built the binary
(`github.repository`), so a fork self-updates from itself. Unset — the
default for a plain local build — disables the feature entirely; so does a
`dev` version string. See `internal/app/selfupdate.go` and PRD §17.

## Branch protection & releases

`main` is protected (PR + passing `ci` required, no approvals required, no
force-push/deletion). `tag-release.yml` auto-bumps the patch version and tags
every non-docs merge to `main`; that tag push triggers `release.yml`, which
builds and publishes. Don't hand-push tags — let `tag-release` own
versioning, or the next auto-bump will collide with a manually chosen one.
