import { useState } from "react";

import { useActions, useStore } from "../lib/store";
import { auth as authCopy } from "../lib/copy";
import { Banner, Button, Card, LinkButton, Mono, SectionLabel } from "../components/ui";

/**
 * The sign-in screen PRD §5.5 asks for, including the Device Flow user code.
 * Discover is empty until this completes, since results are scoped entirely to
 * the signed-in account and its organizations (PRD §7).
 */
export function SignInView() {
  const { signIn, boot } = useStore();
  const actions = useActions();
  const [token, setToken] = useState("");

  const prompt = signIn.prompt;

  return (
    <div className="flex flex-col gap-5 px-6 pt-6 pb-10">
      <Card className="px-6 py-[22px]">
        <SectionLabel>Sign in</SectionLabel>
        <h2 className="mt-4 text-display font-medium tracking-[-0.015em] text-strong">
          Connect your GitHub account
        </h2>
        <p className="mt-4 max-w-[64ch] text-value leading-[1.65] text-faded">
          {authCopy.intro}
        </p>

        {!boot?.deviceFlowReady ? (
          <div className="mt-5">
            <Banner title="Device Flow is not configured in this build">
              No OAuth App Client ID was compiled in. Set{" "}
              <Mono>KOA_GITHUB_CLIENT_ID</Mono> before launching koa, or paste a token
              below.
            </Banner>
          </div>
        ) : null}

        {prompt ? (
          <div className="mt-6 border border-edge-card bg-content px-5 py-5">
            <div className="text-tag uppercase tracking-[0.07em] text-label">
              Your device code
            </div>
            <div className="mt-3 font-mono text-display tracking-[0.14em] text-strong">
              {prompt.userCode}
            </div>
            <div className="mt-4 max-w-[62ch] text-copy leading-[1.6] text-faded">
              {prompt.browserOpened
                ? "Your browser is open at github.com — enter the code there to approve koa."
                : "Open the link below and enter the code to approve koa."}
            </div>
            <div className="mt-4 flex items-center gap-[9px]">
              <Button onClick={() => actions.openExternal(prompt.verificationUri)}>
                Open github.com
              </Button>
              <Button onClick={() => void actions.cancelSignIn()}>Cancel</Button>
            </div>
            <div className="mt-4 font-mono text-meta text-faint">
              {prompt.verificationUri}
            </div>
          </div>
        ) : (
          <div className="mt-6 flex items-center gap-[9px]">
            <Button
              variant="primary"
              size="md"
              disabled={signIn.pending || !boot?.deviceFlowReady}
              onClick={() => void actions.signInWithGitHub()}
            >
              {signIn.pending ? "Contacting GitHub…" : "Sign in with GitHub"}
            </Button>
            <LinkButton onClick={() => actions.openTokenEntry(!signIn.tokenOpen)}>
              {signIn.tokenOpen ? "Hide token entry" : "Use a token instead"}
            </LinkButton>
          </div>
        )}

        {signIn.error ? (
          <div className="mt-5">
            <Banner tone="clay" title="Sign-in failed">
              {signIn.error}
            </Banner>
          </div>
        ) : null}

        {signIn.tokenOpen && !prompt ? (
          <div className="mt-6 border-t border-edge pt-5">
            <div className="text-value text-title">Paste a token</div>
            <p className="mt-2 max-w-[64ch] text-control leading-[1.6] text-faded">
              {authCopy.manualCaveat}
            </p>
            <div className="mt-3 flex items-center gap-[9px]">
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
          </div>
        ) : null}

        <div className="mt-6 border-t border-edge pt-5 text-control leading-[1.65] text-faint">
          {authCopy.scopes}
        </div>
      </Card>
    </div>
  );
}
