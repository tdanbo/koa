import { useMemo } from "react";

import { useActions, useStore } from "../lib/store";
import { errorMessage } from "../lib/format";
import type { InstallProgress, Repo, StatusKind } from "../lib/types";
import {
  Banner,
  Button,
  EmptyState,
  LoadingLine,
  VisibilityBadge,
  cx,
} from "../components/ui";

/** Status text colour follows the semantic accent roles in PRD §5.4. */
const STATUS_TONE: Record<StatusKind, string> = {
  installed: "text-sage",
  update: "text-amber",
  none: "text-dim",
  incompatible: "text-label",
  norelease: "text-label",
};

/** Discover: koa-tagged repos across the account and every org (PRD §8). */
export function DiscoverView() {
  const { discovery, filter, installs, account } = useStore();
  const actions = useActions();

  const repos = useMemo(() => {
    const all = discovery.data?.repos ?? [];
    const needle = filter.trim().toLowerCase();
    if (!needle) return all;
    return all.filter((r) =>
      `${r.name} ${r.owner} ${r.description}`.toLowerCase().includes(needle),
    );
  }, [discovery.data, filter]);

  if (!account.signedIn) {
    return (
      <div className="px-6 pt-5 pb-[26px]">
        <EmptyState
          title="Not signed in"
          action={
            <Button variant="primary" onClick={actions.goSignIn}>
              Sign in with GitHub
            </Button>
          }
        >
          Discover shows repositories tagged <span className="font-mono">koa</span> across
          your own account and the organizations you belong to. Sign in to see them.
        </EmptyState>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[10px] px-6 pt-5 pb-[26px]">
      {discovery.error ? (
        <Banner
          title="Could not reach GitHub"
          action={
            <Button onClick={() => void actions.refreshDiscovery(true)}>Retry</Button>
          }
        >
          {discovery.error}
        </Banner>
      ) : null}

      {(discovery.data?.errors ?? []).map((scopeError) => (
        <Banner
          key={scopeError.scope}
          title={`${scopeError.scope} could not be searched`}
          action={
            scopeError.sso && scopeError.ssoUrl ? (
              <Button onClick={() => actions.openExternal(scopeError.ssoUrl)}>
                Authorize
              </Button>
            ) : null
          }
        >
          {scopeError.message}
        </Banner>
      ))}

      {discovery.loading && !discovery.data ? (
        <LoadingLine>Searching your account and organizations…</LoadingLine>
      ) : null}

      {!discovery.loading && discovery.data && discovery.data.repos.length === 0 ? (
        <EmptyState title="No tagged repositories">
          None of your repositories, or those in your organizations, carry the{" "}
          <span className="font-mono">koa</span> topic yet. Add the topic on GitHub and
          refresh.
        </EmptyState>
      ) : null}

      {discovery.data && discovery.data.repos.length > 0 && repos.length === 0 ? (
        <EmptyState title="No matches">
          Nothing matches “{filter}”.
        </EmptyState>
      ) : null}

      {repos.map((repo) => (
        <RepoRow key={repo.id} repo={repo} progress={installs[repo.id]} />
      ))}
    </div>
  );
}

function RepoRow({ repo, progress }: { repo: Repo; progress?: InstallProgress }) {
  const actions = useActions();

  const open = () =>
    repo.installed
      ? actions.openApp(repo.owner, repo.name)
      : actions.openRepo(repo.owner, repo.name);

  const primary = () => {
    if (repo.installed && repo.action === "Open") {
      actions.openApp(repo.owner, repo.name);
      return;
    }
    void actions.install(repo.owner, repo.name);
  };

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={open}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          open();
        }
      }}
      className="grid grid-cols-[1fr_auto] items-center gap-7 border border-edge-card bg-panel px-5 py-[18px] transition-colors hover:border-edge-hover hover:bg-hover"
    >
      <div className="min-w-0">
        <div className="flex flex-wrap items-baseline gap-[10px]">
          <span className="text-row font-medium tracking-[-0.005em] text-title">
            {repo.name}
          </span>
          <span
            className="font-mono text-badge text-label"
            title={
              repo.ownerScope === "you"
                ? "Your account"
                : `Organization: ${repo.owner}`
            }
          >
            {repo.owner}
          </span>
          <VisibilityBadge value={repo.visibility} />
        </div>
        {repo.description ? (
          <div className="mt-[7px] text-copy leading-[1.5] text-faded">
            {repo.description}
          </div>
        ) : null}
      </div>

      <div className="flex shrink-0 items-center gap-[18px]">
        <span
          className={cx(
            "font-mono text-meta whitespace-nowrap",
            progress ? "text-sage" : STATUS_TONE[repo.statusKind],
          )}
        >
          {progress ? progressLabel(progress) : repo.status}
        </span>
        <div className="flex w-action justify-end">
          {repo.action && !progress ? (
            <Button
              variant="row"
              onClick={(event) => {
                event.stopPropagation();
                primary();
              }}
            >
              {repo.action}
            </Button>
          ) : null}
        </div>
      </div>
    </div>
  );
}

/** progressLabel renders the install stage in the status column. */
export function progressLabel(progress: InstallProgress): string {
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
    case "done":
      return "Installed";
    case "failed":
      return errorMessage(progress.error);
    default:
      return "Working…";
  }
}
