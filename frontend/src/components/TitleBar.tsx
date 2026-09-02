import { win } from "../lib/bridge";
import { CloseGlyph, MaximiseGlyph, MinimiseGlyph } from "./icons";

/**
 * The 32px frameless title bar from PRD §5.2. koa draws its own chrome, so the
 * bar is the window's drag handle and the three controls are 46px hit targets.
 */
export function TitleBar() {
  return (
    <div className="drag-window flex h-titlebar shrink-0 items-stretch justify-between border-b border-edge bg-titlebar">
      <div className="flex items-center gap-[9px] pl-3">
        <div className="flex size-[13px] items-center justify-center border border-edge-hover text-[8px] font-semibold text-dim">
          K
        </div>
        <span className="text-meta tracking-[0.01em] text-dim">Koa</span>
      </div>

      <div className="flex items-stretch">
        <ChromeButton label="Minimise" onClick={() => void win.minimise()}>
          <MinimiseGlyph />
        </ChromeButton>
        <ChromeButton label="Maximise" onClick={() => void win.toggleMaximise()}>
          <MaximiseGlyph />
        </ChromeButton>
        <ChromeButton label="Close" danger onClick={() => void win.close()}>
          <CloseGlyph />
        </ChromeButton>
      </div>
    </div>
  );
}

function ChromeButton({
  label,
  onClick,
  danger,
  children,
}: {
  label: string;
  onClick: () => void;
  danger?: boolean;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      onClick={onClick}
      className={
        "no-drag flex w-chrome items-center justify-center text-faint transition-colors " +
        (danger ? "hover:bg-close hover:text-close-glyph" : "hover:bg-wash-strong")
      }
    >
      {children}
    </button>
  );
}
