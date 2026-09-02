import { useActions, useStore } from "../lib/store";
import { formatRelative } from "../lib/format";
import type { App, InstallProgress } from "../lib/types";
import {
  Button,
  EmptyState,
  RunningBadge,
  UpdateBadge,
  cx,
} from "../components/ui";
import { progressLabel } from "./Discover";

/** Installed: one row per tracked app (PRD §10). */
export function InstalledView() {
  const { apps, installs } = useStore();
  const actions = useActions();

  if (apps.length === 0) {
    return (
      <div className="px-6 pt-5 pb-[26px]">
        <EmptyState
          title="Nothing installed"
          action={<Button onClick={actions.goDiscover}>Browse Discover</Button>}
        >
          Apps you install land in your koa bin folder and appear here.
        </EmptyState>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-[10px] px-6 pt-5 pb-[26px]">
      {apps.map((app) => (
        <AppRow key={app.id} app={app} progress={installs[app.id]} />
      ))}
    </div>
  );
}

function AppRow({ app, progress }: { app: App; progress?: InstallProgress }) {
  const actions = useActions();

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => actions.openApp(app.owner, app.name)}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          actions.openApp(app.owner, app.name);
        }
      }}
      className="grid grid-cols-[1fr_auto] items-center gap-7 border border-edge-card bg-panel px-5 py-[18px] transition-colors hover:border-edge-hover hover:bg-hover"
    >
      <div className="min-w-0">
        <div className="flex flex-wrap items-baseline gap-[10px]">
          <span className="text-row font-medium text-title">{app.name}</span>
          <span className="font-mono text-meta text-dim">{app.version}</span>
          {app.hasUpdate ? <UpdateBadge /> : null}
          {app.running ? <RunningBadge /> : null}
          {app.missing ? (
            <span className="border border-clay-border px-[6px] py-px text-tag text-clay">
              Missing
            </span>
          ) : null}
        </div>
        {app.description ? (
          <div className="mt-[7px] text-copy leading-[1.5] text-faded">
            {app.description}
          </div>
        ) : null}
      </div>

      <div className="flex shrink-0 items-center gap-[18px]">
        <span
          className={cx(
            "font-mono text-meta whitespace-nowrap",
            progress ? "text-sage" : "text-label",
          )}
        >
          {progress ? progressLabel(progress) : `checked ${formatRelative(app.lastChecked)}`}
        </span>
        <Button
          variant="row"
          disabled={Boolean(progress) || app.missing}
          onClick={(event) => {
            event.stopPropagation();
            void actions.launch(app.owner, app.name);
          }}
        >
          Launch
        </Button>
      </div>
    </div>
  );
}
