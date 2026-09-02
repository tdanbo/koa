import { useActions, useStore } from "../lib/store";
import { Button } from "./ui";

/**
 * The trust reminder from PRD §18, shown once before the first install: koa
 * installs and runs third-party binaries, so only trusted repos should be
 * installed.
 */
export function TrustDialog() {
  const { trustPrompt } = useStore();
  const actions = useActions();

  if (!trustPrompt) return null;

  return (
    <div className="absolute inset-0 z-30 flex items-center justify-center bg-window/80 px-6">
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="trust-title"
        className="w-full max-w-[520px] border border-edge-card bg-panel px-6 py-6"
      >
        <div id="trust-title" className="text-tag uppercase tracking-[0.07em] text-label">
          Before you install
        </div>
        <p className="mt-4 text-value leading-[1.65] text-secondary">
          Koa downloads and runs binaries published by repositories you have access
          to. It does not verify checksums or signatures. Only install repositories
          you trust.
        </p>
        <p className="mt-3 text-control leading-[1.6] text-faded">
          You are about to install{" "}
          <span className="font-mono text-secondary">
            {trustPrompt.owner}/{trustPrompt.name}
          </span>
          . This reminder is shown once.
        </p>
        <div className="mt-6 flex items-center justify-end gap-[9px]">
          <Button onClick={actions.dismissTrust}>Cancel</Button>
          <Button variant="primary" onClick={() => void actions.confirmTrust()}>
            I understand — install
          </Button>
        </div>
      </div>
    </div>
  );
}
