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
- Keeping multiple installed versions side-by-side on disk — only one "current" version per app at a time (see §9 for how rollback still works without this).
- A GitHub App / fine-grained per-repo installation flow — v1 uses an OAuth App with Device Flow (see §6). Worth revisiting later if precise per-repo scoping becomes important.
- Self-updating koa from inside its own UI — koa's own updates are handled via re-running the install script (§16) for now.

## 4. Tech Stack

- **Backend:** Go
- **Frontend:** Wails + Tailwind CSS
- **Platforms:** Windows, Linux
- **Local state:** a single local JSON file (OS config dir) for tracked/installed apps and app settings
- **Credentials:** OS-native credential store (e.g. `go-keyring` — Windows Credential Manager / Linux Secret Service), with a plaintext-file fallback (clearly flagged in the UI) when no keyring service is available

## 5. Core Concepts

- **Tracked repo** — a GitHub repo carrying the `koa` topic, owned by the signed-in user or by an org they belong to.
- **koa bin folder** — a dedicated directory (e.g. `~/.koa/bin` on Linux, `%LOCALAPPDATA%\koa\bin` on Windows) that koa creates on first run and adds to the user's PATH. All installed binaries live here, renamed to a clean command name (`{repo-name}` / `{repo-name}.exe`).
- **Naming convention** — the required release-asset filename pattern maintainers must follow for koa to recognize their binaries (§8).

## 6. Authentication

- **Flow:** GitHub OAuth Device Flow via a registered OAuth App (Client ID embedded in the app — safe, since koa is open source and Device Flow requires no client secret).
  - User clicks "Sign in with GitHub" → koa requests a device code → shows a short code and opens the browser to `github.com/login/device` → user approves → koa polls until authorized.
- **Scopes requested:** `repo` (read private repos + release assets) and `read:org` (list org memberships, including private ones — required just to know which orgs to search).
- **Storage:** resulting token stored in the OS keychain (plaintext-file fallback if unavailable).
- **Validation:** koa makes one lightweight API call on save/login to confirm the token works before persisting it.
- **Advanced/manual option:** Settings also allows pasting a token directly (e.g. a hand-scoped fine-grained PAT) instead of using Device Flow. Caveat to document clearly in-app: GitHub's org-membership endpoint returns an empty list for fine-grained tokens by design, so users on this path must type the org names to search for manually rather than relying on auto-detected membership.
- **Sign-in is effectively required** for Discover to show anything meaningful, since results are entirely scoped to "your repos + your orgs." First run should prompt sign-in; Discover stays empty with a sign-in prompt until then.
- **SSO orgs:** if an org enforces SAML SSO, a valid token can still be blocked from that org's private resources until separately authorized on github.com. Surface this as a clear per-org error with a pointer to authorize, rather than silently returning zero repos for that org.

## 7. Discovery (Discover view)

- **Query construction:** one search per scope — `topic:koa user:{username}` for the signed-in user's own repos, plus `topic:koa org:{org-name}` for every org returned by the org-membership call. Merge and de-duplicate results.
- **Per-repo status:** Not installed / Installed vX / Update available / Incompatible (no release asset matches the naming convention for the user's OS).
- Incompatible repos are still shown (so users know the repo exists and why it can't be installed) with install disabled and a short inline reason.
- **Badges:** owner (your account vs. which org) and visibility (Private/Public).
- **README preview:** each repo's README is fetched via GitHub's dedicated repo README endpoint (`GET /repos/{owner}/{repo}/readme`) and rendered as Markdown in the UI — not extracted from a downloaded release archive, since Discover needs to show it before anything is installed, and not every release archive bundles a README anyway. Use this same mechanism for installed apps (§11) so there's one consistent code path.

## 8. Naming Convention & Asset Matching

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

## 9. Install / Update / Version Management / Uninstall

- **Install:** match asset (§8) → download → extract → locate binary → move into the koa bin folder, renamed to the clean command name → `chmod +x` on Linux → record in local state.
- **Update check:** manual, per-app "Check for updates" action, plus a "refresh all" on the Installed view. Compares the installed tag against the repo's current latest release.
- **Auto-update (per app, opt-in, default off):** a toggle per installed app that, when enabled, checks and installs the latest release automatically (e.g. on app launch and on manual refresh) without requiring a click to confirm.
- **Version picker / rollback:** rather than keeping every version cached on disk, let the user browse an installed app's available release tags and choose one to (re-)install — this downloads that specific tag's matching asset and replaces the current binary. This covers "switching versions" without the complexity of maintaining multiple versions side-by-side.
- **Uninstall:** delete the binary from the koa bin folder and remove the app from local state. The repo reverts to "Not installed" in Discover.

## 10. Launch

- Launching from the UI spawns the binary as a subprocess and streams its stdout/stderr into an in-app log panel (not an OS terminal window).
- Support multiple concurrently running apps, each with its own log tab, with a Stop button to kill the process.
- **Terminal runnability:** since installed binaries live in the koa bin folder on PATH under their clean repo name, running `{repo-name}` (or `{repo-name}.exe`) from any terminal must work identically to launching from the UI — this is a direct consequence of §5/§9 and should be treated as an acceptance criterion, not a separate mechanism.

## 11. README Display

- Fetched via the GitHub repo README API endpoint (see §7), rendered as Markdown.
- Shown in two places: the Discover detail view (pre-install) and the Installed app detail view (post-install) — same fetch/render path both times.

## 12. Settings Panel

- **Account:** sign in / sign out with GitHub; manual token entry as an advanced alternative (§6).
- **Appearance:** light / dark / follow-system toggle.
- **Behavior:** "Minimize to tray on close" toggle — when on, closing the window hides it to the tray instead of quitting; when off (default), closing the window quits the app normally.

## 13. System Tray

- Tray icon: a simple "K" monogram. Provide a version that reads well against both light and dark OS tray backgrounds.
- Baseline menu: Open koa, Quit. (Agent may add sensible extras, e.g. a quick "check for updates" — not required.)
- Tray presence and close-to-tray behavior are governed by the Settings toggle in §12.

## 14. Light / Dark Mode

- Three-way setting: Light / Dark / System (default: System).
- Implement via Tailwind's class-based dark mode strategy.

## 15. Data & Storage

- **App state (local JSON file, OS config dir):** tracked/installed apps (repo, owner, installed version, install path), settings (theme, minimize-to-tray, auto-update flags per app), and — for the manual-token path only — any user-entered org names to search.
- **Credentials:** GitHub token stored via OS keychain, plaintext-file fallback (flagged in UI) if no keyring service is present.

## 16. Install Script (for koa itself)

- Provide a one-line install command in the style of `curl -fsSL https://raw.githubusercontent.com/{org}/koa/main/install.sh | sh`, with `install.sh` committed to and served from the koa repo itself.
- Script responsibilities: detect OS, fetch koa's own latest release from GitHub, download the asset matching koa's own naming convention (§8 — koa should dogfood its own convention), extract, and place the binary appropriately for the platform.
- Document this install command prominently in the koa repo's README.
- **Windows note:** `curl | sh` is inherently a Unix-shell pattern and primarily covers Linux. A comparable Windows one-liner (e.g. `irm .../install.ps1 | iex`) is a reasonable fast-follow, not required for v1 — Windows users can download the release directly from GitHub in the meantime.

## 17. Security Notes

- Since koa installs and runs third-party binaries surfaced via a public GitHub topic convention, include a brief, visible reminder in the UI (e.g. on first install) that users should only install repos they trust.
- Token scope is intentionally minimal for what's needed (no scopes at all for public-only use if using a manually pasted token; `repo` + `read:org` for the Device Flow login) — document this in Settings so users understand what access they're granting.

## 18. Prerequisites (manual, not built by the agent)

- Register a GitHub OAuth App (github.com → Developer settings), enable Device Flow, and obtain the Client ID to embed in the app.
- Publish `install.sh` in the koa repo and link it from the README.

## 19. Deferred / Future Enhancements

- arm64 support.
- GitHub App–based fine-grained, per-repo token scoping (as an alternative/addition to Device Flow).
- Per-org filtering in Discover (currently: always search every org the user belongs to).
- Checksum/signature verification.
- Windows install script parity.
- koa self-update from within its own UI.
