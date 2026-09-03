import type { MouseEvent } from "react";

// Wails' own hover-near-edge resize fallback is unreliable on Linux/GTK
// (frameless windows lose the OS's resize grips); "resize:<edge>" is the same
// message its fallback sends, just triggered by a real hit zone instead.
type Edge = "n" | "ne" | "e" | "se" | "s" | "sw" | "w" | "nw";

const EDGE_CURSOR: Record<Edge, string> = {
  n: "n-resize",
  ne: "ne-resize",
  e: "e-resize",
  se: "se-resize",
  s: "s-resize",
  sw: "sw-resize",
  w: "w-resize",
  nw: "nw-resize",
};

function startResize(edge: Edge) {
  return (event: MouseEvent) => {
    if (event.button !== 0) return;
    event.preventDefault();
    window.WailsInvoke?.(`resize:${EDGE_CURSOR[edge]}`);
  };
}

function Handle({ edge, className }: { edge: Edge; className: string }) {
  return (
    <div
      className={`no-drag absolute ${className}`}
      style={{ cursor: EDGE_CURSOR[edge] }}
      onMouseDown={startResize(edge)}
    />
  );
}

export function ResizeHandles() {
  return (
    <div className="pointer-events-none absolute inset-0 z-40">
      <Handle edge="n" className="pointer-events-auto inset-x-[10px] top-0 h-[6px]" />
      <Handle edge="s" className="pointer-events-auto inset-x-[10px] bottom-0 h-[6px]" />
      <Handle edge="w" className="pointer-events-auto inset-y-[10px] left-0 w-[6px]" />
      <Handle edge="e" className="pointer-events-auto inset-y-[10px] right-0 w-[6px]" />
      <Handle edge="nw" className="pointer-events-auto top-0 left-0 h-[10px] w-[10px]" />
      <Handle edge="ne" className="pointer-events-auto top-0 right-0 h-[10px] w-[10px]" />
      <Handle edge="sw" className="pointer-events-auto bottom-0 left-0 h-[10px] w-[10px]" />
      <Handle edge="se" className="pointer-events-auto bottom-0 right-0 h-[10px] w-[10px]" />
    </div>
  );
}
