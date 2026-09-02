import type { ReactNode } from "react";

import { ChevronLeftIcon } from "./icons";

/**
 * The 56px content header from PRD §5.2: an optional breadcrumb, the view
 * title, and contextual actions on the right.
 */
export function ContentHeader({
  title,
  crumb,
  onBack,
  actions,
}: {
  title: string;
  crumb?: string;
  onBack?: () => void;
  actions?: ReactNode;
}) {
  return (
    <header className="drag-window flex h-band shrink-0 items-center justify-between gap-6 border-b border-edge bg-panel px-6">
      <div className="flex min-w-0 items-center gap-[10px]">
        {crumb && onBack ? (
          <>
            <button
              type="button"
              onClick={onBack}
              className="no-drag flex items-center gap-[6px] text-copy text-dim transition-colors hover:text-tertiary"
            >
              <ChevronLeftIcon size={12} />
              <span>{crumb}</span>
            </button>
            <span className="text-copy text-ghost">/</span>
          </>
        ) : null}
        <h1 className="truncate text-head font-medium tracking-[-0.005em] text-strong">
          {title}
        </h1>
      </div>
      <div className="no-drag flex shrink-0 items-center gap-[10px]">{actions}</div>
    </header>
  );
}

/**
 * The 52px status footer from PRD §5.2: a contextual sentence on the left and
 * mono metadata on the right. Every view supplies both.
 */
export function StatusFooter({ left, right }: { left: ReactNode; right: ReactNode }) {
  return (
    <footer className="flex h-footer shrink-0 items-center justify-between gap-6 border-t border-edge bg-panel px-6">
      <div className="truncate text-meta text-dim">{left}</div>
      <div className="shrink-0 font-mono text-meta whitespace-nowrap text-label">{right}</div>
    </footer>
  );
}
