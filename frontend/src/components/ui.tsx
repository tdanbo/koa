import type { ButtonHTMLAttributes, ReactNode } from "react";

/** cx joins class names, dropping falsy entries. */
export function cx(...parts: Array<string | false | null | undefined>): string {
  return parts.filter(Boolean).join(" ");
}

/* --------------------------------------------------------------------------
   Buttons
   --------------------------------------------------------------------------
   Every button in the reference is a 1px-outlined rectangle with a transparent
   or barely-tinted fill. There are no filled buttons and no rounded corners.
-------------------------------------------------------------------------- */

type ButtonVariant = "default" | "row" | "primary" | "amber" | "clay";
type ButtonSize = "sm" | "md";

const VARIANTS: Record<ButtonVariant, string> = {
  default:
    "border-edge-strong text-secondary hover:bg-wash disabled:hover:bg-transparent",
  row: "border-edge-strong text-primary bg-white/[0.03] dark:bg-white/[0.03] hover:bg-wash-strong",
  primary:
    "border-sage-border text-sage-strong bg-sage-fill hover:bg-sage-fill-hover",
  amber: "border-amber-border text-amber-strong hover:bg-amber-hover",
  clay: "border-clay-border text-clay-strong hover:bg-clay-fill",
};

const SIZES: Record<ButtonSize, string> = {
  sm: "text-control px-[13px] py-[6px]",
  md: "text-copy px-4 py-2",
};

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
}

export function Button({
  variant = "default",
  size = "sm",
  className,
  ...rest
}: ButtonProps) {
  return (
    <button
      type="button"
      className={cx(
        "no-drag inline-flex shrink-0 items-center justify-center gap-2 border whitespace-nowrap transition-colors",
        "disabled:cursor-default disabled:opacity-45",
        VARIANTS[variant],
        SIZES[size],
        className,
      )}
      {...rest}
    />
  );
}

/** LinkButton is the underlined text affordance used in Settings. */
export function LinkButton({
  className,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      className={cx(
        "no-drag shrink-0 border-b border-edge-strong pb-[3px] text-control text-dim transition-colors hover:text-tertiary",
        className,
      )}
      {...rest}
    />
  );
}

/** DangerButton is the bare destructive text action (Uninstall). */
export function DangerButton({
  className,
  ...rest
}: ButtonHTMLAttributes<HTMLButtonElement>) {
  return (
    <button
      type="button"
      className={cx(
        "no-drag text-copy text-clay transition-colors hover:text-clay-strong",
        className,
      )}
      {...rest}
    />
  );
}

/* --------------------------------------------------------------------------
   Surfaces
-------------------------------------------------------------------------- */

export function Card({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cx("border border-edge-card bg-panel", className)}>{children}</div>
  );
}

/** SectionLabel is the 10.5px uppercase label above each card section. */
export function SectionLabel({ children }: { children: ReactNode }) {
  return (
    <div className="text-tag uppercase tracking-[0.07em] text-label">{children}</div>
  );
}

/* --------------------------------------------------------------------------
   Badges
-------------------------------------------------------------------------- */

export function VisibilityBadge({ value }: { value: string }) {
  if (!value) return null;
  return (
    <span className="border border-edge-control px-[5px] py-px text-micro uppercase tracking-[0.06em] text-dim">
      {value}
    </span>
  );
}

export function UpdateBadge() {
  return (
    <span className="border border-amber-border px-[6px] py-px text-tag text-amber">
      Update
    </span>
  );
}

export function RunningBadge() {
  return (
    <span className="flex items-center gap-[5px] text-tag text-sage">
      <span className="inline-block size-[5px] rounded-full bg-sage-dot" />
      Running
    </span>
  );
}

export function OutlineBadge({
  children,
  tone = "neutral",
}: {
  children: ReactNode;
  tone?: "neutral" | "sage";
}) {
  return (
    <span
      className={cx(
        "border px-[6px] py-px text-tag",
        tone === "sage" ? "border-sage-border text-sage" : "border-edge-control text-dim",
      )}
    >
      {children}
    </span>
  );
}

/* --------------------------------------------------------------------------
   Banners
   --------------------------------------------------------------------------
   The amber inline banner is the reference's treatment for incompatibility and
   warnings; PRD §5.5 asks for the same language for error and empty states.
-------------------------------------------------------------------------- */

export function Banner({
  tone = "amber",
  title,
  children,
  action,
}: {
  tone?: "amber" | "clay";
  title?: ReactNode;
  children?: ReactNode;
  action?: ReactNode;
}) {
  const palette =
    tone === "clay"
      ? "border-clay-border bg-clay-fill"
      : "border-amber-border bg-amber-fill";
  const titleColor = tone === "clay" ? "text-clay-strong" : "text-amber";
  return (
    <div className={cx("flex items-start justify-between gap-5 border px-4 py-[14px]", palette)}>
      <div className="min-w-0">
        {title ? <div className={cx("mb-[5px] text-copy", titleColor)}>{title}</div> : null}
        {children ? (
          <div className="text-control leading-[1.55] text-faded">{children}</div>
        ) : null}
      </div>
      {action ? <div className="shrink-0">{action}</div> : null}
    </div>
  );
}

/* --------------------------------------------------------------------------
   Controls
-------------------------------------------------------------------------- */

export function Toggle({
  checked,
  onChange,
  label,
  disabled,
}: {
  checked: boolean;
  onChange: (next: boolean) => void;
  label: string;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cx(
        "no-drag h-5 w-9 shrink-0 rounded-[10px] border border-edge-strong p-[2px] transition-colors",
        checked ? "bg-sage-track" : "bg-wash-soft",
        disabled && "opacity-45",
      )}
    >
      <span
        className={cx(
          "block size-[14px] rounded-full transition-transform duration-[160ms] ease-out",
          checked ? "translate-x-4 bg-sage-knob" : "translate-x-0 bg-faint",
        )}
      />
    </button>
  );
}

export function Segmented<T extends string>({
  value,
  options,
  onChange,
  label,
}: {
  value: T;
  options: Array<{ value: T; label: string }>;
  onChange: (next: T) => void;
  label: string;
}) {
  return (
    <div className="flex border border-edge-control" role="radiogroup" aria-label={label}>
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            role="radio"
            aria-checked={active}
            onClick={() => onChange(option.value)}
            className={cx(
              "no-drag px-[15px] py-[6px] text-control transition-colors",
              active ? "bg-wash-strong text-title" : "text-dim hover:text-tertiary",
            )}
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}

export function Tabs<T extends string>({
  value,
  options,
  onChange,
}: {
  value: T;
  options: Array<{ value: T; label: ReactNode; dot?: string }>;
  onChange: (next: T) => void;
}) {
  return (
    <div className="flex gap-6 border-b border-edge px-6">
      {options.map((option) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type="button"
            onClick={() => onChange(option.value)}
            className={cx(
              "-mb-px flex items-center gap-2 border-b pt-[14px] pb-3 text-copy transition-colors",
              active
                ? "border-sage-mid text-title"
                : "border-transparent text-dim hover:text-tertiary",
            )}
          >
            {option.dot ? (
              <span className="size-[5px] rounded-full" style={{ background: option.dot }} />
            ) : null}
            {option.label}
          </button>
        );
      })}
    </div>
  );
}

/* --------------------------------------------------------------------------
   States
   --------------------------------------------------------------------------
   PRD §5.5: empty states follow the muted-label pattern; the reference has no
   spinner, so loading is a quiet line of text rather than motion.
-------------------------------------------------------------------------- */

export function EmptyState({
  title,
  children,
  action,
}: {
  title: string;
  children?: ReactNode;
  action?: ReactNode;
}) {
  return (
    <div className="flex flex-col items-start gap-3 border border-edge-card bg-panel px-6 py-8">
      <div className="text-tag uppercase tracking-[0.07em] text-label">{title}</div>
      {children ? (
        <div className="max-w-[64ch] text-copy leading-[1.6] text-faded">{children}</div>
      ) : null}
      {action}
    </div>
  );
}

export function LoadingLine({ children = "Loading…" }: { children?: ReactNode }) {
  return (
    <div className="text-tag uppercase tracking-[0.07em] text-label" role="status">
      {children}
    </div>
  );
}

export function FactRow({
  label,
  children,
  mono,
  last,
}: {
  label: string;
  children: ReactNode;
  mono?: boolean;
  last?: boolean;
}) {
  return (
    <div
      className={cx(
        "grid grid-cols-[var(--spacing-fact)_1fr] items-baseline gap-6 py-[14px]",
        !last && "border-b border-edge",
      )}
    >
      <div className="text-control text-label">{label}</div>
      <div
        className={cx(
          "min-w-0 break-words text-copy text-secondary",
          mono && "font-mono",
        )}
      >
        {children}
      </div>
    </div>
  );
}

/** Mono is the reference's monospace treatment for versions, paths and tags. */
export function Mono({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return <span className={cx("font-mono", className)}>{children}</span>;
}
