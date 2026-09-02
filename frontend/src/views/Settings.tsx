import { useEffect, useState } from "react";

import { errorMessage } from "../lib/format";
import { useActions, useStore } from "../lib/store";
import type { SelfUpdateProgress, ThemeName } from "../lib/types";
import {
  Banner,
  Button,
  Card,
  LinkButton,
  Mono,
  SectionLabel,
  Segmented,
  Toggle,
} from "../components/ui";

/** Settings: Account, Appearance, Behavior (PRD §13), closing with §18's
 *  trust reminder. */
export function SettingsView() {
  const { account, settings, boot, path, signIn, selfUpdate, selfUpdateProgress } = useStore();
  const actions = useActions();

  const [orgs, setOrgs] = useState(settings.manualOrgs.join(", "));
  useEffect(() => {
    setOrgs(settings.manualOrgs.join(", "));
  }, [settings.manualOrgs]);

  const manualPath = account.source === "manual";

  return (
    <div className="flex flex-col gap-5 px-6 pt-6 pb-10">
      {account.usingPlaintextFallback ? (
        <Banner title="Token stored in a plaintext file">
          No OS keyring was available, so Koa saved your GitHub token to{" "}
          <Mono>{account.plaintextPath}</Mono> with owner-only permissions. Install a
          Secret Service provider (for example gnome-keyring) and sign in again to move
          it into the keyring.
        </Banner>
      ) : null}

      {selfUpdate.available ? (
        <Banner
          title={`Koa ${selfUpdate.latest} is available`}
          action={
            <Button
              variant="primary"
              disabled={Boolean(selfUpdateProgress) && selfUpdateProgress?.stage !== "failed"}
              onClick={() => void actions.installSelfUpdate()}
            >
              {selfUpdateProgress ? selfUpdateStageLabel(selfUpdateProgress) : "Update now"}
            </Button>
          }
        >
          You're on <Mono>{boot?.version}</Mono>. Updating downloads the release
          asset, replaces this binary, and restarts Koa.
        </Banner>
      ) : null}

      {!path.onPath ? (
        <Banner
          title="Koa bin folder is not on your PATH"
          action={<Button onClick={() => void actions.ensurePath()}>Add to PATH</Button>}
        >
          Installed binaries live in <Mono>{boot?.binDir}</Mono>. Until it is on PATH,
          they run only from inside Koa. {path.detail ? `Currently ${path.detail}.` : ""}
        </Banner>
      ) : null}

      <Card className="px-6 py-5">
        <SectionLabel>Account</SectionLabel>
        <div className="flex items-center justify-between gap-6 border-b border-edge py-4">
          <div className="flex items-center gap-3">
            {account.avatarUrl ? (
              <img
                src={account.avatarUrl}
                alt=""
                className="size-[30px] rounded-full border border-edge-control object-cover"
              />
            ) : (
              <span className="size-[30px] rounded-full border border-edge-control bg-wash-strong" />
            )}
            <div>
              <div className="text-value text-title">
                {account.signedIn ? account.login : "Not signed in"}
              </div>
              <div className="mt-[2px] text-meta text-label">
                {account.signedIn ? accountSummary(account) : "Discover stays empty until you sign in."}
              </div>
            </div>
          </div>
          {account.signedIn ? (
            <Button onClick={() => void actions.signOut()}>Sign out</Button>
          ) : (
            <Button variant="primary" onClick={actions.goSignIn}>
              Sign in
            </Button>
          )}
        </div>

        <div className="flex items-center justify-between gap-6 pt-4">
          <div className="max-w-[60ch] text-copy leading-[1.55] text-faded">
            Paste a token instead. Fine-grained tokens return no org memberships, so
            you'll need to name orgs manually.
          </div>
          <LinkButton onClick={() => actions.openTokenEntry(!signIn.tokenOpen)}>
            {signIn.tokenOpen ? "Cancel" : "Enter token"}
          </LinkButton>
        </div>

        {signIn.tokenOpen ? <TokenForm /> : null}

        {manualPath ? (
          <div className="mt-5 border-t border-edge pt-4">
            <div className="text-value text-title">Organizations to search</div>
            <div className="mt-1 max-w-[60ch] text-control leading-[1.55] text-faded">
              GitHub's org-membership endpoint returns nothing for fine-grained tokens
              by design, so name the organizations Koa should search. Comma separated.
            </div>
            <div className="mt-3 flex items-center gap-[9px]">
              <input
                value={orgs}
                onChange={(event) => setOrgs(event.target.value)}
                placeholder="playdead, playdead-tools"
                aria-label="Organizations to search"
                className="w-[320px] border border-edge-control bg-content px-[10px] py-[6px] font-mono text-copy text-body placeholder:text-faint"
              />
              <Button
                onClick={() =>
                  void actions.setManualOrgs(
                    orgs
                      .split(",")
                      .map((value) => value.trim())
                      .filter(Boolean),
                  )
                }
              >
                Save
              </Button>
            </div>
          </div>
        ) : null}

        <div className="mt-5 border-t border-edge pt-4 text-control leading-[1.6] text-faint">
          Device Flow asks for <Mono>repo</Mono> — to read private repositories and
          their release assets — and <Mono>read:org</Mono>, only to learn which
          organizations to search. A manually pasted token needs no scopes at all if
          you only use public repositories.
        </div>
      </Card>

      <Card className="px-6 py-5">
        <SectionLabel>Appearance</SectionLabel>
        <div className="flex items-center justify-between gap-6 pt-4">
          <div className="text-value text-title">Theme</div>
          <Segmented<ThemeName>
            label="Theme"
            value={settings.theme}
            onChange={(theme) => void actions.setTheme(theme)}
            options={[
              { value: "light", label: "Light" },
              { value: "dark", label: "Dark" },
              { value: "system", label: "System" },
            ]}
          />
        </div>
      </Card>

      <Card className="px-6 py-5">
        <SectionLabel>Behavior</SectionLabel>
        <div className="flex items-center justify-between gap-6 border-b border-edge py-4">
          <div>
            <div className="text-value text-title">Minimize to tray on close</div>
            <div className="mt-1 text-control text-label">
              Closing the window hides Koa instead of quitting.
            </div>
          </div>
          <Toggle
            label="Minimize to tray on close"
            checked={settings.minimizeToTray}
            onChange={(next) => void actions.setMinimizeToTray(next)}
          />
        </div>
        <div className="flex items-center justify-between gap-6 pt-4">
          <div>
            <div className="text-value text-title">Bin folder</div>
            <div className="mt-1 font-mono text-control text-label">
              {boot?.binDir} · {path.detail || "not on your PATH"}
            </div>
          </div>
          <LinkButton onClick={() => void actions.revealBinFolder()}>Reveal</LinkButton>
        </div>
      </Card>

      <Card className="px-6 py-5">
        <SectionLabel>Asset convention</SectionLabel>
        <div className="pt-4 text-copy leading-[1.6] text-faded">
          Koa installs a release asset only when its filename follows{" "}
          <Mono className="text-secondary">{boot?.assetPattern}</Mono>. Repositories
          that publish nothing matching are shown as Incompatible.
        </div>
      </Card>

      <Card className="px-6 py-5">
        <SectionLabel>About</SectionLabel>
        <div className="flex items-center justify-between gap-6 pt-4">
          <div>
            <div className="text-value text-title">
              Koa <Mono>{boot?.version}</Mono>
            </div>
            <div className="mt-1 text-control text-label">
              {selfUpdate.available
                ? `${selfUpdate.latest} is available.`
                : "You're up to date."}
            </div>
          </div>
          <LinkButton onClick={() => void actions.checkSelfUpdate()}>
            Check for updates
          </LinkButton>
        </div>
      </Card>

      <p className="max-w-[70ch] text-control leading-[1.65] text-faint">
        Koa installs and runs third-party binaries published by repositories you have
        access to. Only install repositories you trust.
      </p>
    </div>
  );
}

function TokenForm() {
  const { signIn } = useStore();
  const actions = useActions();
  const [token, setToken] = useState("");

  return (
    <div className="mt-4 border-t border-edge pt-4">
      <div className="flex items-center gap-[9px]">
        <input
          type="password"
          value={token}
          autoComplete="off"
          onChange={(event) => setToken(event.target.value)}
          placeholder="ghp_… or github_pat_…"
          aria-label="GitHub token"
          className="w-[320px] border border-edge-control bg-content px-[10px] py-[6px] font-mono text-copy text-body placeholder:text-faint"
        />
        <Button
          variant="primary"
          disabled={signIn.pending || token.trim() === ""}
          onClick={() => void actions.signInWithToken(token)}
        >
          {signIn.pending ? "Verifying…" : "Save token"}
        </Button>
      </div>
      {signIn.error ? (
        <div className="mt-3 text-control text-clay-strong">{signIn.error}</div>
      ) : null}
      <div className="mt-3 max-w-[64ch] text-control leading-[1.55] text-faint">
        Koa validates the token with one lightweight API call before storing it.
      </div>
    </div>
  );
}

function selfUpdateStageLabel(progress: SelfUpdateProgress): string {
  switch (progress.stage) {
    case "resolving":
      return "Resolving…";
    case "downloading": {
      if (progress.total > 0) {
        const pct = Math.min(100, Math.round((progress.done / progress.total) * 100));
        return `Downloading ${pct}%`;
      }
      return "Downloading…";
    }
    case "extracting":
      return "Extracting…";
    case "installing":
      return "Installing…";
    case "relaunching":
      return "Restarting…";
    case "failed":
      return errorMessage(progress.error);
    default:
      return "Working…";
  }
}

function accountSummary(account: {
  source: string;
  scopes: string;
  tokenStorage: string;
}): string {
  const flow = account.source === "manual" ? "Manual token" : "Device Flow";
  const scopes = account.scopes ? ` · scopes ${account.scopes}` : " · no scopes reported";
  return `${flow}${scopes} · token in ${account.tokenStorage}`;
}
