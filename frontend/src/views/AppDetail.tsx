import { useEffect, useState } from "react";

import { koa } from "../lib/bridge";
import { errorMessage, formatBytes, formatDate, formatRelative } from "../lib/format";
import { useActions, useStore, type AppTab } from "../lib/store";
import type { Version } from "../lib/types";
import {
  Banner,
  Button,
  Card,
  DangerButton,
  FactRow,
  LoadingLine,
  Mono,
  OutlineBadge,
  Tabs,
  Toggle,
  VisibilityBadge,
  cx,
} from "../components/ui";
import { progressLabel } from "./Discover";

/** App detail (PRD §10, §12): overview facts, version picker, readme. */
export function AppDetailView({
  owner,
  name,
  tab,
}: {
  owner: string;
  name: string;
  tab: AppTab;
}) {
  const { appDetail, installs, boot } = useStore();
  const actions = useActions();
  const [checking, setChecking] = useState(false);

  if (appDetail.loading && !appDetail.data) {
    return (
      <div className="px-6 pt-6 pb-10">
        <LoadingLine>Loading {name}…</LoadingLine>
      </div>
    );
  }

  if (appDetail.error || !appDetail.data) {
    return (
      <div className="px-6 pt-6 pb-10">
        <Banner
          title="Could not load this app"
          action={<Button onClick={actions.goInstalled}>Back to Installed</Button>}
        >
          {appDetail.error || `${owner}/${name} is no longer installed.`}
        </Banner>
      </div>
    );
  }

  const { app, latestPublishedAt } = appDetail.data;
  const progress = installs[app.id];
  const prompt = boot?.platform === "windows" ? ">" : "$";

  const check = async () => {
    setChecking(true);
    try {
      await actions.checkForUpdates(owner, name);
    } finally {
      setChecking(false);
    }
  };

  return (
    <div className="flex flex-col gap-5 px-6 pt-6 pb-10">
      <Card className="px-6 py-[22px]">
        <div className="flex items-start justify-between gap-8">
          <div className="min-w-0">
            <h2 className="text-display font-medium tracking-[-0.015em] text-strong">
              {app.name}
            </h2>
            <div className="mt-[9px] flex flex-wrap items-center gap-[10px]">
              <span className="font-mono text-control text-dim">{app.owner}</span>
              <VisibilityBadge value={app.visibility} />
              <span className="text-control text-ghost">·</span>
              <Mono className="text-control text-tertiary">{app.version}</Mono>
            </div>
            {app.description ? (
              <p className="mt-4 max-w-[64ch] text-value leading-[1.6] text-faded">
                {app.description}
              </p>
            ) : null}
          </div>
          <Button size="md" disabled={checking} onClick={() => void check()}>
            {checking ? "Checking…" : "Check for updates"}
          </Button>
        </div>
      </Card>

      {app.missing ? (
        <Banner
          tone="clay"
          title="Binary missing"
          action={
            <Button
              variant="primary"
              disabled={Boolean(progress)}
              onClick={() => void actions.install(owner, name, app.version)}
            >
              {progress ? progressLabel(progress) : "Reinstall"}
            </Button>
          }
        >
          Koa tracks this app but <Mono>{app.binaryPath}</Mono> is not on disk.
        </Banner>
      ) : null}

      {app.hasUpdate ? (
        <div className="flex items-center justify-between gap-5 border border-amber-border bg-amber-fill px-5 py-[13px]">
          <div className="text-copy text-amber-soft">
            Version <Mono className="text-amber-strong">{app.latestVersion}</Mono> is
            available — released {formatRelative(latestPublishedAt)}.
          </div>
          <Button
            variant="amber"
            disabled={Boolean(progress)}
            onClick={() => void actions.install(owner, name)}
          >
            {progress ? progressLabel(progress) : "Update"}
          </Button>
        </div>
      ) : null}

      <Card>
        <Tabs<AppTab>
          value={tab}
          onChange={actions.setAppTab}
          options={[
            { value: "overview", label: "Overview" },
            { value: "versions", label: "Versions" },
            { value: "readme", label: "Readme" },
          ]}
        />

        {tab === "overview" ? (
          <div className="px-6 pt-[6px] pb-[22px]">
            <FactRow label="Installed version" mono>
              {app.version}
            </FactRow>
            <FactRow label="Release published">{formatDate(app.publishedAt)}</FactRow>
            <FactRow label="Binary path" mono>
              {app.binaryPath}
            </FactRow>
            <FactRow label="Command" mono>
              {prompt} {app.command}
            </FactRow>
            <FactRow label="Asset" mono>
              {app.assetName || "—"}
            </FactRow>
            <FactRow label="Size on disk">{formatBytes(app.sizeBytes)}</FactRow>
            <FactRow label="Last checked">{formatRelative(app.lastChecked)}</FactRow>

            <div className="grid grid-cols-[var(--spacing-fact)_1fr] items-center gap-6 border-b border-edge py-[18px]">
              <div className="text-control text-label">Auto-update</div>
              <div className="flex items-center justify-between gap-5">
                <div className="max-w-[52ch] text-copy leading-[1.55] text-faded">
                  Install the latest release automatically on launch and on refresh,
                  without confirming.
                </div>
                <Toggle
                  label="Auto-update this app"
                  checked={app.autoUpdate}
                  onChange={(next) => void actions.setAutoUpdate(owner, name, next)}
                />
              </div>
            </div>

            <div className="flex items-center gap-5 pt-[22px]">
              <DangerButton onClick={() => void actions.uninstall(owner, name)}>
                Uninstall
              </DangerButton>
              <div className="text-control text-faint">
                Removes the binary from <Mono>{app.binaryPath}</Mono> and forgets this
                app.
              </div>
            </div>
          </div>
        ) : null}

        {tab === "versions" ? <VersionsTab owner={owner} name={name} /> : null}
        {tab === "readme" ? <ReadmeTab owner={owner} name={name} /> : null}
      </Card>
    </div>
  );
}

function VersionsTab({ owner, name }: { owner: string; name: string }) {
  const { versions, installs } = useStore();
  const actions = useActions();
  const busy = Boolean(installs[`${owner.toLowerCase()}/${name.toLowerCase()}`]);

  return (
    <div className="px-6 pt-[18px] pb-[22px]">
      <p className="mb-[14px] max-w-[70ch] text-copy leading-[1.6] text-dim">
        One version is kept on disk at a time. Selecting a tag downloads that
        release's asset and replaces the current binary.
      </p>

      {versions.loading && !versions.data ? <LoadingLine>Loading releases…</LoadingLine> : null}

      {versions.error ? (
        <Banner title="Could not list releases">{versions.error}</Banner>
      ) : null}

      {versions.data && versions.data.length === 0 ? (
        <div className="text-copy text-faded">This repository has no published releases.</div>
      ) : null}

      {versions.data && versions.data.length > 0 ? (
        <div className="border-t border-edge">
          {versions.data.map((version) => (
            <VersionRow
              key={version.tag}
              version={version}
              busy={busy}
              onSelect={() => void actions.install(owner, name, version.tag)}
            />
          ))}
        </div>
      ) : null}
    </div>
  );
}

function VersionRow({
  version,
  busy,
  onSelect,
}: {
  version: Version;
  busy: boolean;
  onSelect: () => void;
}) {
  return (
    <div className="grid grid-cols-[1fr_auto] items-center gap-6 border-b border-edge px-[2px] py-[15px] transition-colors hover:bg-subtle">
      <div className="flex flex-wrap items-baseline gap-3">
        <Mono
          className={cx("text-value", version.isCurrent ? "text-title" : "text-primary")}
        >
          {version.tag}
        </Mono>
        <span className="text-meta text-label">{formatDate(version.publishedAt)}</span>
        {version.isCurrent ? <OutlineBadge tone="sage">Current</OutlineBadge> : null}
        {version.isLatest ? <OutlineBadge>Latest</OutlineBadge> : null}
        {!version.compatible ? (
          <span className="text-tag text-amber">no compatible asset</span>
        ) : null}
      </div>
      <div className="flex items-center gap-4">
        <Mono className="text-meta text-faint">{formatBytes(version.sizeBytes)}</Mono>
        <Button
          variant={version.isLatest ? "primary" : "default"}
          disabled={busy || !version.compatible}
          onClick={onSelect}
        >
          {version.action}
        </Button>
      </div>
    </div>
  );
}

function ReadmeTab({ owner, name }: { owner: string; name: string }) {
  const [state, setState] = useState<{ html: string; error: string; loading: boolean }>({
    html: "",
    error: "",
    loading: true,
  });

  useEffect(() => {
    let cancelled = false;
    setState({ html: "", error: "", loading: true });
    void koa
      .readme(owner, name)
      .then((html) => {
        if (!cancelled) setState({ html, error: "", loading: false });
      })
      .catch((err: unknown) => {
        if (!cancelled) setState({ html: "", error: errorMessage(err), loading: false });
      });
    return () => {
      cancelled = true;
    };
  }, [owner, name]);

  return (
    <div className="px-6 pt-[22px] pb-[26px]">
      {state.loading ? <LoadingLine>Loading readme…</LoadingLine> : null}
      {state.error ? <div className="text-copy text-faded">{state.error}</div> : null}
      {!state.loading && !state.error && !state.html ? (
        <div className="text-copy text-faded">This repository has no readme.</div>
      ) : null}
      {state.html ? (
        <div
          className="koa-md select-text-content"
          dangerouslySetInnerHTML={{ __html: state.html }}
        />
      ) : null}
    </div>
  );
}
