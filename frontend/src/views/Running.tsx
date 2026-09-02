import { useEffect, useRef } from "react";

import { formatClock } from "../lib/format";
import { useActions, useStore } from "../lib/store";
import type { LogLine, Process } from "../lib/types";
import { Button, EmptyState, cx } from "../components/ui";

/** Stream colour follows the accent roles: neutral output, amber for stderr. */
const STREAM_TONE: Record<LogLine["stream"], string> = {
  stdout: "text-muted",
  stderr: "text-amber",
  system: "text-label",
};

/** Running: one tab per live process over a timestamped log grid (PRD §11). */
export function RunningView() {
  const { processes, logs, activeProcess } = useStore();
  const actions = useActions();

  const lines = logs[activeProcess] ?? [];
  const scroller = useRef<HTMLDivElement>(null);
  const pinned = useRef(true);

  // Follow the tail unless the user has scrolled up to read history.
  useEffect(() => {
    const el = scroller.current;
    if (!el || !pinned.current) return;
    el.scrollTop = el.scrollHeight;
  }, [lines.length, activeProcess]);

  if (processes.length === 0) {
    return (
      <div className="px-6 pt-5 pb-[26px]">
        <EmptyState
          title="Nothing running"
          action={<Button onClick={actions.goInstalled}>Open Installed</Button>}
        >
          Launching an app from Koa streams its output here instead of opening a
          terminal window.
        </EmptyState>
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="flex shrink-0 gap-[22px] border-b border-edge bg-panel px-6">
        {processes.map((process) => (
          <ProcessTab
            key={process.id}
            process={process}
            active={process.id === activeProcess}
            onSelect={() => actions.selectProcess(process.id)}
          />
        ))}
      </div>

      <div
        ref={scroller}
        onScroll={(event) => {
          const el = event.currentTarget;
          pinned.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24;
        }}
        className="min-h-0 flex-1 overflow-y-auto px-6 pt-[18px] pb-[26px]"
      >
        {lines.length === 0 ? (
          <div className="text-tag uppercase tracking-[0.07em] text-label">
            No output yet
          </div>
        ) : (
          <div className="select-text-content">
            {lines.map((line) => (
              <div
                key={line.seq}
                className="grid grid-cols-[var(--spacing-gutter)_1fr] gap-4 font-mono text-control leading-[2.05]"
              >
                <span className="text-fainter">{formatClock(line.time)}</span>
                <span className={cx("whitespace-pre-wrap", STREAM_TONE[line.stream])}>
                  {line.text}
                </span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function ProcessTab({
  process,
  active,
  onSelect,
}: {
  process: Process;
  active: boolean;
  onSelect: () => void;
}) {
  const actions = useActions();

  return (
    <div className="flex items-center">
      <button
        type="button"
        onClick={onSelect}
        className={cx(
          "-mb-px flex items-center gap-2 border-b pt-3 pb-[10px] text-copy transition-colors",
          active
            ? "border-sage-mid text-title"
            : "border-transparent text-dim hover:text-tertiary",
        )}
      >
        <span
          className={cx(
            "size-[5px] rounded-full",
            process.running ? "bg-sage-dot" : "bg-faint",
          )}
        />
        <span className="font-mono">{process.repo}</span>
      </button>
      {!process.running ? (
        <button
          type="button"
          aria-label={`Close ${process.repo} log`}
          title="Close log"
          onClick={() => void actions.closeProcess(process.id)}
          className="ml-2 px-1 text-control text-faint transition-colors hover:text-tertiary"
        >
          ×
        </button>
      ) : null}
    </div>
  );
}
