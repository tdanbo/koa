# koa

koa is a small cross-platform desktop app for discovering, installing, updating
and launching binaries published as GitHub releases.

Maintainers opt a repository in by adding the **`koa` topic** on GitHub and
publishing release assets that follow [koa's naming convention](#the-naming-convention).
You sign in with GitHub, and koa shows every `koa`-tagged repository you own or
can reach through an organization, installs the right binary for your OS with
one click, keeps it on your `PATH` so it runs from any terminal, and can launch
it from inside the app with a live log panel.

Think of it as a personal or team-scoped installer — closer to a lightweight
internal package manager than a public app store.

Platforms: **Windows** and **Linux**, amd64.

---

## Install koa

**Linux:**

```sh
curl -fsSL https://raw.githubusercontent.com/tdanbo/koa/main/install.sh | sh
```

Detects your OS, fetches koa's latest release, downloads the asset matching
koa's own naming convention, and installs the binary to `~/.local/bin`.

**Windows:**

```powershell
irm https://raw.githubusercontent.com/tdanbo/koa/main/install.ps1 | iex
```

Same idea, installing to `%LOCALAPPDATA%\Programs\koa` and printing the
command to add it to your `PATH` if it isn't already there.

Both scripts read the same environment variables:

| Variable | Purpose | Default |
|---|---|---|
| `KOA_INSTALL_DIR` | Where the binary is placed | `~/.local/bin` (Linux) / `%LOCALAPPDATA%\Programs\koa` (Windows) |
| `KOA_VERSION` | Install a specific tag | latest release |
| `KOA_REPO` | Install from a fork | `tdanbo/koa` |
| `GITHUB_TOKEN` | Private repos, higher rate limits | unset |

koa checks its own latest release in the background and, when a newer one
exists, shows an indicator next to Settings and a banner offering **Update
now** — click it and koa downloads the matching release, replaces its own
binary in place, and restarts. Re-running the install command remains a valid
fallback (first install, or a location koa can't write to itself).

---

## The naming convention

This is the contract. koa recognises a release asset only when its filename
matches:

```
{repo-name}-{version}-{arch}-{os}{ext}
```

| OS | Extension | Example |
|---|---|---|
| Linux | `.tar.gz` | `myapp-1.2.0-amd64-linux.tar.gz` |
| Windows | `.zip` | `myapp-1.2.0-amd64-windows.zip` |

The matching rule, precisely:

- the filename **starts with** `{repo-name}-`;
- it **contains** an amd64 architecture keyword — `amd64` is canonical, and
  `x86_64` and `x64` are accepted as equivalents;
- it **ends with** the OS keyword plus a supported extension for the user's
  platform.

The version segment in the middle is never parsed or reconstructed. Whatever is
there, is there — `1.2.0`, `v1.2.0` and `2026.09.02` all work.

A bare binary published without an archive is also accepted
(`myapp-1.2.0-amd64-linux`, `myapp-1.2.0-amd64-windows.exe`); when a release
publishes both an archive and a bare binary, the archive wins.

Inside the archive, koa looks for a file named `{repo-name}` (or
`{repo-name}.exe` on Windows), at any depth. Accompanying `LICENSE`,
`README.md`, checksum and signature files are ignored.

A repository whose latest release publishes nothing matching is shown in
Discover as **Incompatible**, with the expected pattern spelled out. There is no
manual "pick the asset yourself" override — the convention is the interface.

### Making a repository koa-installable

1. Add the topic `koa` to the repository on GitHub.
2. Publish a release whose assets follow the pattern above.
3. That's it — anyone who can see the repository and is signed into koa will
   find it in Discover.

koa dogfoods this: its own releases publish `koa-{version}-amd64-linux.tar.gz`
and `koa-{version}-amd64-windows.zip`.

---

## Using koa

**Sign in.** koa uses GitHub's OAuth Device Flow: it shows a short code, opens
your browser, and you approve it on github.com. No client secret is involved.
The token is stored in your OS credential store — Windows Credential Manager or
the Linux Secret Service — falling back to an owner-only file if no keyring is
available, which koa tells you about in Settings.

The Device Flow asks for two scopes:

- `repo` — read private repositories and download their release assets;
- `read:org` — list your organization memberships, purely to know which orgs to
  search.

You can paste a token instead (Settings › *Enter token*). One caveat worth
knowing: GitHub's org-membership endpoint returns an empty list for
fine-grained tokens by design, so on that path you name the organizations koa
should search yourself.

**Discover** runs one search per scope — `topic:koa user:{you}` plus
`topic:koa org:{each-org}` — then merges and de-duplicates. It is deliberately
*not* a global search of GitHub: you only ever see your own repositories and
those of organizations you belong to. If an organization enforces SAML SSO and
your token has not been authorized for it, koa says so and links you to the
authorization page rather than silently showing zero results.

**Install** downloads the matching asset, extracts the binary, and places it in
the koa bin folder under a clean command name:

| | Bin folder | State file |
|---|---|---|
| Linux | `~/.koa/bin` | `~/.config/koa/state.json` |
| Windows | `%LOCALAPPDATA%\koa\bin` | `%APPDATA%\koa\state.json` |

koa adds that folder to your `PATH` on first run — a marker-delimited block in
your shell profiles on Linux, the user environment in the registry on Windows.
So an app installed as `dumpscope` runs by typing `dumpscope` in any terminal,
exactly as it does when launched from koa. Open a new terminal after the first
install for the change to take effect.

**Update** is a per-app check plus a *Check all for updates* action on the
Installed view. Each app also has an opt-in **auto-update** toggle (off by
default) that installs the latest release on launch and on refresh without
asking.

**Versions.** koa keeps one version on disk at a time. The Versions tab lists a
repository's release tags; selecting one downloads that release's asset and
replaces the current binary. That is how both rollback and reinstall work.

**Launch** runs the binary as a child process and streams its stdout and stderr
into koa's log panel rather than opening a terminal window. Several apps can run
at once, each with its own tab and a Stop button.

**Readmes** are fetched from GitHub's repository readme endpoint — not from the
downloaded archive — so they are available before anything is installed, and the
same code path serves Discover and installed apps.

---

## Security

koa installs and runs third-party binaries surfaced through a public topic
convention. It shows a one-time reminder before your first install, and repeats
it in Settings: **only install repositories you trust.**

koa does **not** verify checksums or signatures — there is no consistent
cross-project convention to rely on, so it would be theatre rather than a
guarantee. Readme Markdown is rendered server-side and sanitized before it
reaches the webview.

---

## Building from source

**Prerequisites**

- Go 1.24+
- Node 20+
- [Wails v2](https://wails.io/docs/gettingstarted/installation):
  `go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- Linux only: a C toolchain and the GTK/WebKit development headers.

  ```sh
  sudo apt-get install -y build-essential libgtk-3-dev libwebkit2gtk-4.1-dev
  ```

**Build**

```sh
wails build -tags webkit2_41 \
  -ldflags "-X main.version=$(git describe --tags --always) -X main.githubClientID=$KOA_GITHUB_CLIENT_ID"
```

Drop `-tags webkit2_41` on distributions that still ship webkit2gtk-4.0.

**Develop**

```sh
wails dev -tags webkit2_41     # desktop shell with hot reload
cd frontend && npm run dev     # the UI alone in a browser, against a mock backend
```

The browser mode exists so the frontend can be worked on and compared side by
side with `PRD/UI/koa.dc.html`. Its sample data never reaches the desktop build.

**Test**

```sh
go test ./...
cd frontend && npm run build   # type-checks, then bundles
```

**Icons** are generated, not hand-drawn:

```sh
go run ./build/icons/gen.go
```

### The OAuth App

Device Flow needs a GitHub OAuth App Client ID. Register one under
*Settings › Developer settings › OAuth Apps*, enable Device Flow, and either
bake the ID in at build time (`-X main.githubClientID=…`) or set
`KOA_GITHUB_CLIENT_ID` in the environment before launching. The Client ID is not
a secret — Device Flow uses no client secret.

### Releasing

`main` is protected: every change lands through a pull request, and the `ci`
workflow (backend vet/format/test/Windows-cross-build, frontend
type-check/build) has to pass before it can merge. Force-pushes and deletion
are blocked.

Once a PR merges, `tag-release` auto-bumps the patch version (`vX.Y.Z` →
`vX.Y.(Z+1)`), tags the new commit, and pushes the tag — docs-only merges
(README, PRD) are skipped. That tag push triggers `release`, which builds
Linux and Windows binaries and publishes them to the repo's Releases page
under koa's own naming convention, exactly what `install.sh` and koa's own
self-update expect to find.

---

## Project layout

```
main.go                 Wails shell: window, tray, close behaviour, bindings
internal/app            Application service — the API the frontend binds to
internal/ghapi          GitHub REST client (search, releases, readme, assets)
internal/assetmatch     The naming convention (§9) and its matching rules
internal/installer      Download, extract, and place binaries in the bin folder
internal/pathenv        Keeping the bin folder on PATH, per platform
internal/runner         Child processes and their streamed logs
internal/auth           Device Flow and credential storage
internal/store          state.json — installed apps and settings
internal/markdown       Readme rendering and sanitizing
internal/tray           The "K" monogram in the system tray
internal/config         Platform paths, in absolute and display form
frontend/               React + Tailwind UI
PRD/                    Product requirements and the normative UI reference
```

The Go packages hold all the logic and are unit-tested without a GUI; the Wails
layer is a thin shell over them.

---

## Not in this version

macOS. Architectures other than amd64. Global discovery across all of GitHub.
A manual asset picker. Checksum or signature verification. `.deb` / `.rpm` /
`.msi` packages. Keeping multiple versions side by side on disk. A GitHub App
with fine-grained per-repository scoping.
