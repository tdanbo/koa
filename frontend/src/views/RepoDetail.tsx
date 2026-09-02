import { useStore, useActions } from "../lib/store";
import { formatBytes, formatDate } from "../lib/format";
import {
  Banner,
  Button,
  Card,
  LoadingLine,
  SectionLabel,
  VisibilityBadge,
} from "../components/ui";
import { progressLabel } from "./Discover";

/**
 * Repo detail (PRD §8, §12): header card with the primary action, three
 * release facts, then the readme fetched from the repo readme endpoint.
 */
export function RepoDetailView({ owner, name }: { owner: string; name: string }) {
  const { repoDetail, installs } = useStore();
  const actions = useActions();

  if (repoDetail.loading && !repoDetail.data) {
    return (
      <div className="px-6 pt-6 pb-10">
        <LoadingLine>Loading {name}…</LoadingLine>
      </div>
    );
  }

  if (repoDetail.error || !repoDetail.data) {
    return (
      <div className="px-6 pt-6 pb-10">
        <Banner
          title="Could not load this repository"
          action={<Button onClick={() => actions.openRepo(owner, name)}>Retry</Button>}
        >
          {repoDetail.error || "GitHub returned no data for this repository."}
        </Banner>
      </div>
    );
  }

  const { repo, readmeHtml, readmeError } = repoDetail.data;
  const progress = installs[repo.id];

  return (
    <div className="flex flex-col gap-5 px-6 pt-6 pb-10">
      <Card className="px-6 py-[22px]">
        <div className="flex items-start justify-between gap-8">
          <div className="min-w-0">
            <h2 className="text-display font-medium tracking-[-0.015em] text-strong">
              {repo.name}
            </h2>
            <div className="mt-[9px] flex items-center gap-[10px]">
              <span className="font-mono text-control text-dim">{repo.owner}</span>
              <VisibilityBadge value={repo.visibility} />
            </div>
            {repo.description ? (
              <p className="mt-4 max-w-[64ch] text-value leading-[1.6] text-faded">
                {repo.description}
              </p>
            ) : null}
          </div>

          {repo.canInstall ? (
            <Button
              variant="primary"
              size="md"
              disabled={Boolean(progress)}
              onClick={() =>
                repo.installed && repo.action === "Open"
                  ? actions.openApp(repo.owner, repo.name)
                  : void actions.install(repo.owner, repo.name)
              }
            >
              {progress ? progressLabel(progress) : repo.action}
            </Button>
          ) : null}
        </div>

        {repo.incompatible || repo.statusKind === "norelease" ? (
          <div className="mt-5">
            <Banner
              title={
                repo.incompatible
                  ? "Incompatible with this platform"
                  : "No release published"
              }
            >
              {repo.incompatibleReason}
            </Banner>
          </div>
        ) : null}

        <div className="mt-[22px] grid grid-cols-3 gap-6 border-t border-edge pt-[18px]">
          <Fact label="Latest release" mono>
            {repo.latestVersion || "—"}
          </Fact>
          <Fact label="Published">{formatDate(repo.publishedAt)}</Fact>
          <Fact label="Asset" mono truncate>
            {repo.assetName || "—"}
          </Fact>
        </div>

        {repo.assetSize > 0 ? (
          <div className="mt-3 font-mono text-meta text-faint">
            {formatBytes(repo.assetSize)}
          </div>
        ) : null}
      </Card>

      <Card className="px-6 py-[22px]">
        <div className="mb-[18px]">
          <SectionLabel>Readme</SectionLabel>
        </div>
        {readmeError ? (
          <div className="text-copy text-faded">{readmeError}</div>
        ) : readmeHtml ? (
          <div
            className="koa-md select-text-content"
            // The backend renders Markdown with goldmark and sanitizes the
            // result with bluemonday before it ever reaches the webview.
            dangerouslySetInnerHTML={{ __html: readmeHtml }}
          />
        ) : (
          <div className="text-copy text-faded">This repository has no readme.</div>
        )}
      </Card>
    </div>
  );
}

function Fact({
  label,
  children,
  mono,
  truncate,
}: {
  label: string;
  children: React.ReactNode;
  mono?: boolean;
  truncate?: boolean;
}) {
  return (
    <div className="min-w-0">
      <div className="text-tag uppercase tracking-[0.07em] text-label">{label}</div>
      <div
        className={
          "mt-[6px] text-value text-primary" +
          (mono ? " font-mono" : "") +
          (truncate ? " truncate" : "")
        }
      >
        {children}
      </div>
    </div>
  );
}
