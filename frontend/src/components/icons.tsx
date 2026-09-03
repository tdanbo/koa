import type { SVGProps } from "react";

/** Line icons traced from PRD/UI/koa.dc.html — 1.3–1.4 stroke, no fills. */

type IconProps = SVGProps<SVGSVGElement> & { size?: number };

function Icon({ size = 14, children, ...rest }: IconProps) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth={1.3}
      aria-hidden="true"
      {...rest}
    >
      {children}
    </svg>
  );
}

export const SearchIcon = (props: IconProps) => (
  <Icon {...props}>
    <circle cx="7" cy="7" r="4.6" />
    <path d="M10.4 10.4L14 14" />
  </Icon>
);

export const BoxIcon = (props: IconProps) => (
  <Icon {...props}>
    <path d="M2.5 5.5L8 2.5l5.5 3v5L8 13.5l-5.5-3z" />
    <path d="M2.5 5.5L8 8.5l5.5-3M8 8.5v5" />
  </Icon>
);

export const TerminalIcon = (props: IconProps) => (
  <Icon {...props}>
    <rect x="2.5" y="3" width="11" height="10" rx="1.6" />
    <path d="M5 6.5l1.8 1.6L5 9.7M8.8 10h2.6" />
  </Icon>
);

export const GearIcon = (props: IconProps) => (
  <Icon {...props}>
    <circle cx="8" cy="8" r="2.1" />
    <path d="M8 1.8v1.7M8 12.5v1.7M14.2 8h-1.7M3.5 8H1.8M12.4 3.6l-1.2 1.2M4.8 11.2l-1.2 1.2M12.4 12.4l-1.2-1.2M4.8 4.8L3.6 3.6" />
  </Icon>
);

export const ChevronLeftIcon = (props: IconProps) => (
  <Icon strokeWidth={1.4} {...props}>
    <path d="M9.5 3.5L5 8l4.5 4.5" />
  </Icon>
);

export const GitHubIcon = ({ size = 14, ...rest }: IconProps) => (
  <svg width={size} height={size} viewBox="0 0 16 16" fill="currentColor" aria-hidden="true" {...rest}>
    <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-2.91-.88-2.91-2.9 0-.58.21-1.06.55-1.43-.05-.14-.24-.69.05-1.44 0 0 .56-.18 1.84.68a5.9 5.9 0 0 1 3.2 0c1.28-.87 1.84-.68 1.84-.68.29.75.1 1.3.05 1.44.34.37.55.85.55 1.43 0 2.03-1.14 2.7-2.92 2.9.3.26.56.76.56 1.54 0 1.11-.01 2.01-.01 2.29 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z" />
  </svg>
);

/** Window chrome glyphs, 10×10 as in the reference. */

export const MinimiseGlyph = () => (
  <svg width="10" height="10" viewBox="0 0 10 10" stroke="currentColor" strokeWidth={1} aria-hidden="true">
    <path d="M0 5h10" />
  </svg>
);

export const MaximiseGlyph = () => (
  <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth={1} aria-hidden="true">
    <rect x="0.5" y="0.5" width="9" height="9" />
  </svg>
);

export const RestoreGlyph = () => (
  <svg width="10" height="10" viewBox="0 0 10 10" fill="none" stroke="currentColor" strokeWidth={1} aria-hidden="true">
    <path d="M2.5 0.5h7v7h-2" />
    <rect x="0.5" y="2.5" width="7" height="7" />
  </svg>
);

export const CloseGlyph = () => (
  <svg width="10" height="10" viewBox="0 0 10 10" stroke="currentColor" strokeWidth={1} aria-hidden="true">
    <path d="M0 0l10 10M10 0L0 10" />
  </svg>
);
