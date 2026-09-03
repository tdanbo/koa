# koa — Product Requirements Document

## 1. Summary

koa is a small cross-platform desktop application for discovering, installing, updating, and launching binaries published as GitHub releases. Maintainers opt their repos in by adding a `koa` topic on GitHub and publishing release assets that follow koa's naming convention. Users sign in with GitHub, and koa shows every `koa`-tagged repo they own or belong to via an organization, lets them install the right binary for their OS with one click, keeps it on PATH so it runs from any terminal, and can launch it from inside the app with a log panel.

Think of it as a personal/team-scoped installer — closer to a lightweight internal package manager than a public app store.

## 2. Goals

- Cross-platform desktop app (Windows + Linux), Go backend, Wails shell, Tailwind CSS frontend.
- Discover `koa`-tagged repos across the signed-in user's own account and every org they belong to (not global GitHub search).
- Install the correct release binary per OS via a strict, documented naming convention.
- Keep installed binaries on the system PATH, runnable by typing the repo/app name in any terminal.
- Support private repos the user has access to, using the same flow as public ones.
- Provide update checking, manual version selection/rollback, optional per-app auto-update, and uninstall.
- Launch installed apps from inside koa with streamed output in an in-app log panel.
- Show each repo's README, both while browsing (pre-install) and for installed apps.
- Light/dark mode, system tray presence, and a settings panel covering account, appearance, and close behavior.
- Ship a `curl | sh`-style install script for koa itself.

## 3. Non-Goals (v1)

Explicitly out of scope — call these out if asked, don't build them:

- macOS support.
- Architectures other than amd64/x64 (arm64 is a plausible fast-follow, not v1).
- Global/public discovery across all of GitHub — only the signed-in user's own repos + their orgs.
- A manual "pick the right asset yourself" override for repos that don't match the naming convention — non-matching repos are simply marked Incompatible.
- Checksum/signature verification of downloaded assets (no consistent cross-project convention to rely on).
- Package formats beyond raw binaries, `.tar.gz`, and `.zip` — no `.deb`/`.rpm`/`.msi`/installers.
- Keeping multiple installed versions side-by-side on disk — only one "current" version per app at a time (see §10 for how rollback still works without this).
- A GitHub App / fine-grained per-repo installation flow — v1 uses an OAuth App with Device Flow (see §7). Worth revisiting later if precise per-repo scoping becomes important.

## 4. Tech Stack

- **Backend:** Go
- **Frontend:** Wails + Tailwind CSS
- **Platforms:** Windows, Linux
- **Local state:** a single local JSON file (OS config dir) for tracked/installed apps and app settings
- **UI reference:** `PRD/UI/koa.dc.html` — the normative visual and interaction spec for the whole frontend (§5)
- **Credentials:** OS-native credential store (e.g. `go-keyring` — Windows Credential Manager / Linux Secret Service), with a plaintext-file fallback (clearly flagged in the UI) when no keyring service is available

## 5. UI Reference (Visual & Interaction Spec)

**`PRD/UI/koa.dc.html` is the authoritative guideline for the GUI.** It is a complete, clickable mockup of the application — every view, plus its layout, typography, color, spacing, and navigation behavior. Where the rest of this document describes *what* each screen does, the reference defines *how it should look and behave*.

**Precedence:** on questions of presentation (layout, color, type, spacing, chrome, hover behavior) the reference wins. On questions of functionality (what an action does, which API is called, what gets stored) this document wins. Where the reference is silent, see "Gaps" at the end of this section.

### 5.1 How to use the reference

- Open `PRD/UI/koa.dc.html` in a browser and click through every view before writing any frontend code. Navigation, tabs, and toggles are live.
- It is a **design reference, not source code to port.** The file is a Claude Design canvas document: it uses `<x-dc>`, `<sc-if>`, `<sc-for>`, `{{ … }}` bindings and a `DCLogic` class, and `PRD/UI/support.js` is the canvas runtime. None of that ships. Read it for structure, values, and behavior; reimplement in Wails + Tailwind (§4).
- The inline `style="…"` attributes are the spec: extract the concrete values (colors, px sizes, gaps, borders) into Tailwind theme tokens rather than hard-coding them per component. `style-hover="…"` attributes are hover states.
- The sample repos, apps, versions, and log lines in the script block (`pace-cli`, `dumpscope`, `assetlint`, …) are **placeholder data** illustrating each state — not fixtures to ship.
- `hint-placeholder-*` attributes are canvas authoring hints and carry no product meaning.

### 5.2 Application shell

Fixed three-band vertical layout, full window, no page-level scrolling — only the content area scrolls.

- **Custom title bar, 32px, frameless** (`#101213`): small "K" monogram box + `koa` wordmark on the left; minimize / maximize / close on the right as 46px hit targets. Close hovers to red (`#8a3a35`); the others to `rgba(255,255,255,.06)`. The app draws its own window chrome — do not use the native OS title bar.
- **Left nav rail** (`#161819`), 206px expanded / 58px icon-only when collapsed: a 56px branded header, then Discover / Installed / Running, a flexible spacer, and Settings pinned to the bottom. A 52px account footer shows avatar, username, and "Signed in". The active item gets `rgba(255,255,255,.06)` background and `#dcdedb` text; Running carries a pulsing 5px sage dot whenever a process is live. Repo detail keeps Discover active; app detail keeps Installed active.
- **Content header, 56px:** optional breadcrumb (back chevron + parent name + `/`) followed by the view title at 15px/500. Right side holds contextual actions only — Discover: a 230px "Filter repositories" input plus a "Refresh" button (forces a fresh GitHub search rather than serving the cached result — needed because a topic added on GitHub after Discover last ran otherwise stays invisible until sign-out or restart); Installed: "Check all for updates"; App detail: "Launch"; Running: "Clear" and "Stop".
- **Content area:** scrolls vertically, `#131516`, 20–24px padding.
- **Status footer, 52px:** contextual sentence on the left (e.g. "3 apps in `%LOCALAPPDATA%\koa\bin` — on your PATH."), mono metadata on the right (e.g. "1 update available", "pid 48213 · uptime 00:04:11"). Every view supplies both.

### 5.3 Views

The reference covers six views; they map onto the functional sections of this document:

| Reference view | Section | Shape |
|---|---|---|
| Discover | §8 | List of full-width repo rows: name + owner (mono) + visibility badge, description, status text (color-coded), right-aligned action button in a fixed 82px column |
| Repo detail | §8, §12 | Header card (name, owner, visibility, description, primary action) → 3-column release facts (latest / published / asset) → Readme card |
| Installed | §10 | List rows: name + version (mono) + optional Update / Running badges, description, last-checked, Launch |
| App detail | §10, §12 | Header card + "Check for updates" → amber update banner when applicable → tabbed panel: **Overview** (label/value fact rows in a 190px grid, auto-update toggle, Uninstall), **Versions** (tag rows with Current/Latest badges and Reinstall / Update / Roll back), **Readme** |
| Running | §11 | One tab per live process (mono name + status dot) over a 74px-gutter timestamped log grid |
| Settings | §13 | Stacked cards — Account, Appearance, Behavior — each with an uppercase label and separator-divided rows; closes with the trust reminder from §18 |

Incompatible repos (§9) appear in the reference as a dimmed status plus an amber-bordered inline explanation naming the expected asset pattern — reuse that banner treatment for the Discover row and detail view.

### 5.4 Design language

Flat, dense, low-chroma, and squared-off — closer to a developer tool than a consumer app store.

- **Surfaces (dark):** window `#0d0e0f`, title bar `#101213`, content `#131516`, rail/panels/cards `#161819`, row hover `#191c1d`.
- **Borders instead of shadows:** `1px solid rgba(255,255,255,.06)` for structural dividers, `.07` for card outlines, `.09–.12` for interactive edges. **No drop shadows anywhere.**
- **Text ramp:** `#dfe1de` titles, `#d7d9d6` body, `#a9aeaa`/`#b0b5b1` secondary, `#7c837f` muted, `#5f6663`/`#616864` labels, `#454b48`/`#4f5552` faintest (log timestamps, sizes).
- **Accents, used semantically:** sage `#8fa896` / `#9db0a4` / `#b3c4b8` = installed, current, active, primary action, success log lines; amber `#b8a179` / `#c8b287` = update available, warnings, incompatible; clay `#a8837e` / `#bb928c` = destructive (Uninstall, Stop). Neutral `#6d7370` = "not installed" / inert.
- **Typography:** IBM Plex Sans (400/450/500/600) for UI; **IBM Plex Mono** for versions and tags, file paths, asset filenames, owner handles, log output and timestamps, and footer metadata. Scale is small and tight: 10–10.5px uppercase section labels (`letter-spacing:.07em`), 11.5–12.5px body/controls, 13px values, 14.5–15px row and header titles, 20px detail-page titles. Negative tracking on large text; `line-height` 1.5–1.7 for prose, 2.05 for log lines.
- **Geometry:** **square corners throughout** — border-radius appears only on avatars (50%) and toggle pills (10px). Buttons are 1px-outlined rectangles with transparent or barely-tinted fills, ~6px/13px padding; the primary action is the same rectangle tinted sage (`rgba(158,178,164,.10)`, border `rgba(158,178,164,.28)`). There are no filled/solid buttons.
- **Controls:** toggle = 36×20 track with a 14px knob, `translateX(16px)` on, `transition:transform .16s ease`, sage track when on. Tabs = text with a 1px sage underline when active. Segmented control (theme picker) = bordered row of equal cells, active cell tinted.
- **Motion:** effectively none beyond hover color changes, the toggle knob, and the 2.4s `koaPulse` running indicator. No page transitions.
- **Density:** the reference exposes Airy (18px/20px row padding) vs Compact (13px/18px), description visibility, and rail collapse as switches. Build the layout so these remain cheap to add; only the Airy, descriptions-on, rail-expanded default is required for v1.

### 5.5 Gaps in the reference

The reference does not cover these — resolve them consistent with its language rather than inventing a new one:

- **Light theme.** The reference is dark-only. §15 still requires Light / Dark / System, so derive light by inverting the neutral ramp while keeping layout, geometry, and accent *roles* identical; adjust the sage/amber/clay lightness to hold contrast on light surfaces.
- **Linux paths.** The reference shows Windows paths (`%LOCALAPPDATA%\koa\bin`, `%APPDATA%\koa\state.json`). Show the platform-correct path (`~/.koa/bin`, etc.) at runtime.
- **Empty, loading, and error states** — signed-out Discover, zero tracked repos, in-flight download/extract progress, network and API failures, the SSO-authorization error (§7), and the keyring-fallback warning (§4). Follow the amber inline-banner pattern for warnings and the muted-label pattern for empty states.
- **Sign-in / Device Flow screen**, including the user code display (§7).
- **Window minimum size and rail auto-collapse breakpoint.**
- **Keyboard navigation and focus styling.** The reference sets `input:focus{outline:none}` with no replacement; supply visible focus indicators rather than shipping none.

**Acceptance criterion:** a reviewer clicking through the built app side by side with `PRD/UI/koa.dc.html` should find the same views, the same information in the same places, and the same visual language.

## 6. Core Concepts

- **Tracked repo** — a GitHub repo carrying the `koa` topic, owned by the signed-in user or by an org they belong to.
- **koa bin folder** — a dedicated directory (e.g. `~/.koa/bin` on Linux, `%LOCALAPPDATA%\koa\bin` on Windows) that koa creates on first run and adds to the user's PATH. All installed binaries live here, renamed to a clean command name (`{repo-name}` / `{repo-name}.exe`).
- **Naming convention** — the required release-asset filename pattern maintainers must follow for koa to recognize their binaries (§9).

## 7. Authentication

- **Flow:** GitHub OAuth Device Flow via a registered OAuth App (Client ID embedded in the app — safe, since koa is open source and Device Flow requires no client secret).
  - User clicks "Sign in with GitHub" → koa requests a device code → shows a short code and opens the browser to `github.com/login/device` → user approves → koa polls until authorized.
- **Scopes requested:** `repo` (read private repos + release assets) and `read:org` (list org memberships, including private ones — required just to know which orgs to search).
- **Storage:** resulting token stored in the OS keychain (plaintext-file fallback if unavailable).
- **Validation:** koa makes one lightweight API call on save/login to confirm the token works before persisting it.
- **Advanced/manual option:** Settings also allows pasting a token directly (e.g. a hand-scoped fine-grained PAT) instead of using Device Flow. Caveat to document clearly in-app: GitHub's org-membership endpoint returns an empty list for fine-grained tokens by design, so users on this path must type the org names to search for manually rather than relying on auto-detected membership.
- **Sign-in is effectively required** for Discover to show anything meaningful, since results are entirely scoped to "your repos + your orgs." First run should prompt sign-in; Discover stays empty with a sign-in prompt until then.
- **SSO orgs:** if an org enforces SAML SSO, a valid token can still be blocked from that org's private resources until separately authorized on github.com. Surface this as a clear per-org error with a pointer to authorize, rather than silently returning zero repos for that org.

## 8. Discovery (Discover view)

- **Query construction:** one search per scope — `topic:koa user:{username}` for the signed-in user's own repos, plus `topic:koa org:{org-name}` for every org returned by the org-membership call. Merge and de-duplicate results.
- **Per-repo status:** Not installed / Installed vX / Update available / Incompatible (no release asset matches the naming convention for the user's OS).
- Incompatible repos are still shown (so users know the repo exists and why it can't be installed) with install disabled and a short inline reason.
- **Badges:** owner (your account vs. which org) and visibility (Private/Public).
- **README preview:** each repo's README is fetched via GitHub's dedicated repo README endpoint (`GET /repos/{owner}/{repo}/readme`) and rendered as Markdown in the UI — not extracted from a downloaded release archive, since Discover needs to show it before anything is installed, and not every release archive bundles a README anyway. Use this same mechanism for installed apps (§12) so there's one consistent code path.

## 9. Naming Convention & Asset Matching

**Pattern:** `{repo-name}-{version}-{arch}-{os}`

| OS | Extension | Example |
|---|---|---|
| Linux | `.tar.gz` | `myapp-1.2.0-amd64-linux.tar.gz` |
| Windows | `.zip` | `myapp-1.2.0-amd64-windows.zip` |

- **Matching rule:** an asset qualifies if its filename starts with `{repo-name}-`, contains the `amd64` arch keyword, and ends with the right OS keyword + extension for the user's platform. The version segment in the middle is not parsed or reconstructed — whatever's there, is there.
- **Arch:** amd64/x64 only in v1.
- **Extraction:** download the matching archive, extract it, and locate the binary inside (expect it named `{repo-name}` / `{repo-name}.exe`; ignore accompanying files like `LICENSE`/`README`).
- **No match found:** the repo shows as Incompatible in Discover — no manual asset picker, no fallback.
- This convention should be documented clearly in koa's own README/docs as the contract maintainers must follow to be koa-installable.

## 10. Install / Update / Version Management / Uninstall

- **Install:** match asset (§9) → download → extract → locate binary → move into the koa bin folder, renamed to the clean command name → `chmod +x` on Linux → record in local state.
- **Update check:** manual, per-app "Check for updates" action, plus a "refresh all" on the Installed view. Compares the installed tag against the repo's current latest release.
- **Auto-update (per app, opt-in, default off):** a toggle per installed app that, when enabled, checks and installs the latest release automatically (e.g. on app launch and on manual refresh) without requiring a click to confirm.
- **Version picker / rollback:** rather than keeping every version cached on disk, let the user browse an installed app's available release tags and choose one to (re-)install — this downloads that specific tag's matching asset and replaces the current binary. This covers "switching versions" without the complexity of maintaining multiple versions side-by-side.
- **Uninstall:** delete the binary from the koa bin folder and remove the app from local state. The repo reverts to "Not installed" in Discover.

## 11. Launch

- Launching from the UI spawns the binary as a subprocess and streams its stdout/stderr into an in-app log panel (not an OS terminal window).
- Support multiple concurrently running apps, each with its own log tab, with a Stop button to kill the process.
- **Terminal runnability:** since installed binaries live in the koa bin folder on PATH under their clean repo name, running `{repo-name}` (or `{repo-name}.exe`) from any terminal must work identically to launching from the UI — this is a direct consequence of §6/§10 and should be treated as an acceptance criterion, not a separate mechanism.

## 12. README Display

- Fetched via the GitHub repo README API endpoint (see §8), rendered as Markdown.
- Shown in two places: the Discover detail view (pre-install) and the Installed app detail view (post-install) — same fetch/render path both times.

## 13. Settings Panel

- **Account:** sign in / sign out with GitHub; manual token entry as an advanced alternative (§7).
- **Appearance:** light / dark / follow-system toggle.
- **Behavior:** "Minimize to tray on close" toggle — when on, closing the window hides it to the tray instead of quitting; when off (default), closing the window quits the app normally.

## 14. System Tray

- Tray icon: a simple "K" monogram. Provide a version that reads well against both light and dark OS tray backgrounds.
- Baseline menu: Open koa, Quit. (Agent may add sensible extras, e.g. a quick "check for updates" — not required.)
- Tray presence and close-to-tray behavior are governed by the Settings toggle in §13.

## 15. Light / Dark Mode

- Three-way setting: Light / Dark / System (default: System).
- Implement via Tailwind's class-based dark mode strategy.
- The UI reference (§5) specifies the **dark** palette only; derive the light palette from it as described in §5.5 — same layout, geometry, and accent roles, neutral ramp inverted.

## 16. Data & Storage

- **App state (local JSON file, OS config dir):** tracked/installed apps (repo, owner, installed version, install path), settings (theme, minimize-to-tray, auto-update flags per app), and — for the manual-token path only — any user-entered org names to search.
- **Credentials:** GitHub token stored via OS keychain, plaintext-file fallback (flagged in UI) if no keyring service is present.

## 17. Install Script (for koa itself)

- Provide a one-line install command in the style of `curl -fsSL https://raw.githubusercontent.com/{org}/koa/main/install.sh | sh`, with `install.sh` committed to and served from the koa repo itself.
- Script responsibilities: detect OS, fetch koa's own latest release from GitHub, download the asset matching koa's own naming convention (§9 — koa should dogfood its own convention), extract, and place the binary appropriately for the platform.
- Document this install command prominently in the koa repo's README.
- **Windows:** `curl | sh` is inherently a Unix-shell pattern, so Windows gets its own `install.ps1`, run as `irm https://raw.githubusercontent.com/{org}/koa/main/install.ps1 | iex`. Same responsibilities as above — resolve the release, download `koa-{version}-amd64-windows.zip`, place `koa.exe` under `%LOCALAPPDATA%\Programs\koa` by default, and tell the user how to add it to `PATH` if it isn't there yet. `irm | iex` cannot take script parameters, so like `install.sh` it takes all configuration through the same environment variables (`KOA_REPO`, `KOA_VERSION`, `KOA_INSTALL_DIR`, `GITHUB_TOKEN`, `KOA_API`).

### Self-Update

- koa checks its own latest GitHub release in the background — once shortly after launch, then on a slow interval for as long as it stays open — and compares the tag against the running build. Dev builds (no embedded version) never check.
- When a newer release exists, the UI surfaces it without interrupting anything: a small indicator next to Settings in the nav rail, plus a banner in Settings naming the version and offering an **Update now** action. The user decides when to act; koa never installs a self-update without an explicit click.
- Clicking Update now downloads the release asset matching koa's own naming convention (§9), replaces koa's own running executable in place — reusing the same match/download/extract pipeline as installing any other koa-tagged repo, just writing to koa's own binary location instead of the koa bin folder — then relaunches. A failure at any stage (no matching asset, network error, relaunch failure after a successful swap) is surfaced as a clear message rather than left silent.
- This is the primary way koa updates itself going forward. The install script (above) remains a valid fallback — for a first install, or on a platform/location where koa cannot write to its own executable.

## 18. Security Notes

- Since koa installs and runs third-party binaries surfaced via a public GitHub topic convention, include a brief, visible reminder in the UI (e.g. on first install) that users should only install repos they trust.
- Token scope is intentionally minimal for what's needed (no scopes at all for public-only use if using a manually pasted token; `repo` + `read:org` for the Device Flow login) — document this in Settings so users understand what access they're granting.

## 19. Prerequisites (manual, not built by the agent)

- Register a GitHub OAuth App (github.com → Developer settings), enable Device Flow, and obtain the Client ID to embed in the app.
- Publish `install.sh` and `install.ps1` in the koa repo and link both from the README.

## 20. Deferred / Future Enhancements

- arm64 support.
- GitHub App–based fine-grained, per-repo token scoping (as an alternative/addition to Device Flow).
- Per-org filtering in Discover (currently: always search every org the user belongs to).
- Checksum/signature verification.
