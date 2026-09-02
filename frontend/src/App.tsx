import { useEffect, useState, type ReactNode } from "react";

import { ContentHeader, StatusFooter } from "./components/Chrome";
import { NavRail } from "./components/NavRail";
import { TitleBar } from "./components/TitleBar";
import { Toasts } from "./components/Toasts";
import { TrustDialog } from "./components/TrustDialog";
import { Button, LoadingLine } from "./components/ui";
import { SearchIcon } from "./components/icons";
import {
  formatRelative,
  formatUptime,
  plural,
} from "./lib/format";
import { useActions, useStore, type View } from "./lib/store";
import { AppDetailView } from "./views/AppDetail";
import { DiscoverView } from "./views/Discover";
import { InstalledView } from "./views/Installed";
import { RepoDetailView } from "./views/RepoDetail";
import { RunningView } from "./views/Running";
import { SettingsView } from "./views/Settings";
import { SignInView } from "./views/SignIn";

/** useNow re-renders on an interval, for the live uptime in the footer. */
function useNow(intervalMs: number, enabled: boolean): number {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    if (!enabled) return;
    const timer = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(timer);
  }, [intervalMs, enabled]);
  return now;
}

export function App() {
  const state = useStore();
  const actions = useActions();

  if (!state.ready) {
    return (
      <Shell>
        <div className="flex flex-1 items-center justify-center">
          <LoadingLine>Starting Koa…</LoadingLine>
        </div>
      </Shell>
    );
  }

  if (state.fatal) {
    return (
      <Shell>
        <div className="flex flex-1 items-center justify-center px-10">
          <div className="max-w-[60ch] border border-clay-border bg-clay-fill px-6 py-5">
            <div className="text-copy text-clay-strong">Koa could not start</div>
            <div className="mt-2 text-control leading-[1.6] text-faded">{state.fatal}</div>
          </div>
        </div>
      </Shell>
    );
  }

  const chrome = buildChrome(state, actions);

  return (
    <Shell>
      <div className="flex min-h-0 flex-1 bg-content">
        <NavRail />
        <div className="relative flex min-w-0 flex-1 flex-col">
          <ContentHeader
            title={chrome.title}
            crumb={chrome.crumb}
            onBack={chrome.onBack}
            actions={chrome.actions}
          />
          <main
            className={
              state.view.kind === "running"
                ? "flex min-h-0 flex-1 flex-col bg-content"
                : "min-h-0 flex-1 overflow-y-auto bg-content"
            }
          >
            <ViewBody view={state.view} />
          </main>
          <StatusFooter left={chrome.footerLeft} right={chrome.footerRight} />
          <Toasts />
          <TrustDialog />
        </div>
      </div>
    </Shell>
  );
}

function Shell({ children }: { children: ReactNode }) {
  return (
    <div className="flex h-full w-full flex-col overflow-hidden border border-edge-control bg-window">
      <TitleBar />
      {children}
    </div>
  );
}

function ViewBody({ view }: { view: View }) {
  switch (view.kind) {
    case "signin":
      return <SignInView />;
    case "discover":
      return <DiscoverView />;
    case "repo":
      return <RepoDetailView owner={view.owner} name={view.name} />;
    case "installed":
      return <InstalledView />;
    case "app":
      return <AppDetailView owner={view.owner} name={view.name} tab={view.tab} />;
    case "running":
      return <RunningView />;
    case "settings":
      return <SettingsView />;
    default:
      return null;
  }
}

interface Chrome {
  title: string;
  crumb?: string;
  onBack?: () => void;
  actions?: ReactNode;
  footerLeft: ReactNode;
  footerRight: ReactNode;
}

/**
 * buildChrome mirrors the reference's per-view chrome table: title, optional
 * breadcrumb, contextual right-hand actions, and both halves of the footer.
 */
function buildChrome(
  state: ReturnType<typeof useStore>,
  actions: ReturnType<typeof useActions>,
): Chrome {
  const { view, boot, apps, discovery, processes, account, path } = state;
  const version = boot?.version ?? "";

  switch (view.kind) {
    case "signin":
      return {
        title: "Sign in",
        footerLeft: "Discover stays empty until Koa knows whose repositories to list.",
        footerRight: `Koa ${version}`,
      };

    case "discover": {
      const data = discovery.data;
      const orgCount = Math.max(0, (data?.scopes.length ?? 1) - 1);
      return {
        title: "Discover",
        actions: <FilterField />,
        footerLeft: data
          ? `Repositories tagged koa across your account and ${plural(orgCount, "organization")}.`
          : "Repositories tagged koa across your account and organizations.",
        footerRight: data
          ? `${plural(data.repos.length, "repo")} · refreshed ${formatRelative(data.refreshedAt)}`
          : discovery.loading
            ? "searching…"
            : "—",
      };
    }

    case "repo": {
      const repo = state.repoDetail.data?.repo;
      return {
        title: repo?.name ?? view.name,
        crumb: "Discover",
        onBack: actions.goDiscover,
        footerLeft: `github.com/${view.owner}/${view.name}`,
        footerRight: repo?.latestVersion
          ? `${repo.latestVersion} · ${formatRelative(repo.publishedAt)}`
          : "no releases",
      };
    }

    case "installed": {
      const pending = apps.filter((a) => a.hasUpdate).length;
      return {
        title: "Installed",
        actions: (
          <Button
            disabled={state.appsRefreshing || apps.length === 0}
            onClick={() => void actions.checkAllForUpdates()}
          >
            {state.appsRefreshing ? "Checking…" : "Check all for updates"}
          </Button>
        ),
        footerLeft: `${plural(apps.length, "app")} in ${boot?.binDir ?? ""} — ${
          path.onPath ? "on your PATH." : `${path.detail || "not on your PATH"}.`
        }`,
        footerRight: pending === 0 ? "no updates" : `${plural(pending, "update")} available`,
      };
    }

    case "app": {
      const app = state.appDetail.data?.app;
      return {
        title: app?.name ?? view.name,
        crumb: "Installed",
        onBack: actions.goInstalled,
        actions: (
          <Button
            variant="primary"
            disabled={!app || app.missing}
            onClick={() => void actions.launch(view.owner, view.name)}
          >
            Launch
          </Button>
        ),
        footerLeft: app?.binaryPath ?? `${view.owner}/${view.name}`,
        footerRight: app ? `checked ${formatRelative(app.lastChecked)}` : "—",
      };
    }

    case "running":
      return {
        title: "Running",
        actions: <RunningActions />,
        footerLeft: `${plural(processes.filter((p) => p.running).length, "process")} running · output streams into Koa, not a terminal window.`,
        footerRight: <RunningMeta />,
      };

    case "settings":
      return {
        title: "Settings",
        footerLeft: `State in ${boot?.stateFile ?? ""}${
          account.signedIn ? ` · token in ${account.tokenStorage}` : ""
        }.`,
        footerRight: `Koa ${version}`,
      };

    default:
      return { title: "Koa", footerLeft: "", footerRight: `Koa ${version}` };
  }
}

function FilterField() {
  const { filter } = useStore();
  const actions = useActions();
  return (
    <div className="flex w-filter items-center gap-2 border border-edge-control bg-content px-[10px] py-[6px]">
      <SearchIcon size={12} className="shrink-0 text-label" />
      <input
        value={filter}
        onChange={(event) => actions.setFilter(event.target.value)}
        placeholder="Filter repositories"
        aria-label="Filter repositories"
        className="min-w-0 flex-1 border-none bg-transparent p-0 text-copy text-body placeholder:text-label"
      />
    </div>
  );
}

function RunningActions() {
  const { processes, activeProcess } = useStore();
  const actions = useActions();
  const current = processes.find((p) => p.id === activeProcess);

  return (
    <div className="flex gap-[9px]">
      <Button
        disabled={!current}
        onClick={() => current && void actions.clearLog(current.id)}
      >
        Clear
      </Button>
      <Button
        variant="clay"
        disabled={!current?.running}
        onClick={() => current && void actions.stopProcess(current.id)}
      >
        Stop
      </Button>
    </div>
  );
}

function RunningMeta() {
  const { processes, activeProcess } = useStore();
  const current = processes.find((p) => p.id === activeProcess);
  const now = useNow(1000, Boolean(current?.running));

  if (!current) return <>no process selected</>;
  if (!current.running) {
    return <>exited {current.exitCode} · ran {formatUptime(current.startedAt, current.exitedAt)}</>;
  }
  return (
    <>
      pid {current.pid} · uptime {formatUptime(current.startedAt, current.exitedAt, now)}
    </>
  );
}
