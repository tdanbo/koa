import { useEffect } from "react";

import { useActions, useStore } from "../lib/store";
import { cx } from "./ui";

const TONE: Record<string, string> = {
  success: "border-sage-border text-sage-strong",
  info: "border-edge-strong text-secondary",
  warning: "border-amber-border text-amber",
  error: "border-clay-border text-clay-strong",
};

/**
 * Transient messages for background outcomes — an auto-update finishing, a
 * failed check. Square, bordered and unanimated, in keeping with PRD §5.4.
 */
export function Toasts() {
  const { toasts } = useStore();
  const actions = useActions();

  useEffect(() => {
    if (toasts.length === 0) return;
    const timers = toasts.map((toast) =>
      window.setTimeout(() => actions.dismissToast(toast.id), 6000),
    );
    return () => timers.forEach(window.clearTimeout);
  }, [toasts, actions]);

  if (toasts.length === 0) return null;

  return (
    <div className="pointer-events-none absolute right-6 bottom-[68px] z-20 flex w-[360px] flex-col gap-2">
      {toasts.map((toast) => (
        <button
          key={toast.id}
          type="button"
          onClick={() => actions.dismissToast(toast.id)}
          className={cx(
            "pointer-events-auto border bg-panel px-4 py-3 text-left text-copy leading-[1.5]",
            TONE[toast.kind] ?? TONE.info,
          )}
        >
          {toast.message}
        </button>
      ))}
    </div>
  );
}
