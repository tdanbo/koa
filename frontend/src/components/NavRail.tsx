import { useMediaQuery, railCollapseQuery } from "../lib/media";
import { useActions, useStore, type View } from "../lib/store";
import { BoxIcon, GearIcon, SearchIcon, TerminalIcon } from "./icons";
import { cx } from "./ui";

type NavKey = "discover" | "installed" | "running" | "settings";

/**
 * The left rail from PRD §5.2: 206px expanded, 58px icon-only below the
 * collapse breakpoint. Repo detail keeps Discover active and app detail keeps
 * Installed active, so the rail always names where you are.
 */
export function NavRail() {
  const { view, account, processes } = useStore();
  const actions = useActions();
  const collapsed = useMediaQuery(railCollapseQuery);

  const active = activeKey(view);
  const hasRunning = processes.some((p) => p.running);

  return (
    <nav
      className={cx(
        "flex shrink-0 flex-col border-r border-edge bg-panel",
        collapsed ? "w-rail-tight" : "w-rail",
      )}
    >
      <div
        className={cx(
          "flex h-band shrink-0 items-center gap-[10px] border-b border-edge",
          collapsed ? "justify-center px-0" : "px-[18px]",
        )}
      >
        <div className="flex size-[22px] items-center justify-center border border-edge-hover text-badge font-semibold text-tertiary">
          K
        </div>
        {collapsed ? null : (
          <span className="text-brand font-medium text-title">koa</span>
        )}
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-[2px] px-[10px] py-[14px]">
        <NavItem
          label="Discover"
          icon={<SearchIcon />}
          active={active === "discover"}
          collapsed={collapsed}
          onClick={actions.goDiscover}
        />
        <NavItem
          label="Installed"
          icon={<BoxIcon />}
          active={active === "installed"}
          collapsed={collapsed}
          onClick={actions.goInstalled}
        />
        <NavItem
          label="Running"
          icon={<TerminalIcon />}
          active={active === "running"}
          collapsed={collapsed}
          onClick={actions.goRunning}
          trailing={
            hasRunning ? (
              <span className="size-[5px] animate-koa-pulse rounded-full bg-sage-dot" />
            ) : null
          }
        />
        <div className="flex-1" />
        <NavItem
          label="Settings"
          icon={<GearIcon />}
          active={active === "settings"}
          collapsed={collapsed}
          onClick={actions.goSettings}
        />
      </div>

      <button
        type="button"
        title={account.signedIn ? `${account.login} — signed in` : "Sign in with GitHub"}
        onClick={account.signedIn ? actions.goSettings : actions.goSignIn}
        className={cx(
          "flex h-footer shrink-0 items-center gap-[10px] border-t border-edge text-left transition-colors hover:bg-wash-soft",
          collapsed ? "justify-center px-0" : "px-[18px]",
        )}
      >
        <Avatar url={account.avatarUrl} />
        {collapsed ? null : (
          <span className="min-w-0">
            <span className="block truncate text-control text-tertiary">
              {account.signedIn ? account.login : "Not signed in"}
            </span>
            <span className="mt-px block text-tag text-label">
              {account.signedIn ? "Signed in" : "Sign in with GitHub"}
            </span>
          </span>
        )}
      </button>
    </nav>
  );
}

function Avatar({ url }: { url: string }) {
  if (url) {
    return (
      <img
        src={url}
        alt=""
        className="size-5 shrink-0 rounded-full border border-edge-control object-cover"
      />
    );
  }
  return <span className="size-5 shrink-0 rounded-full border border-edge-control bg-wash-strong" />;
}

function NavItem({
  label,
  icon,
  active,
  collapsed,
  onClick,
  trailing,
}: {
  label: string;
  icon: React.ReactNode;
  active: boolean;
  collapsed: boolean;
  onClick: () => void;
  trailing?: React.ReactNode;
}) {
  return (
    <button
      type="button"
      aria-current={active ? "page" : undefined}
      aria-label={collapsed ? label : undefined}
      title={collapsed ? label : undefined}
      onClick={onClick}
      className={cx(
        "relative flex items-center gap-[11px] py-2 text-value transition-colors",
        collapsed ? "justify-center px-0" : "px-[10px]",
        active ? "bg-wash-strong text-title" : "text-dim hover:bg-wash-soft",
      )}
    >
      {icon}
      {collapsed ? null : <span className="flex-1 text-left">{label}</span>}
      {collapsed ? (
        trailing ? (
          <span className="absolute top-[6px] right-[9px] flex">{trailing}</span>
        ) : null
      ) : (
        trailing
      )}
    </button>
  );
}

function activeKey(view: View): NavKey | null {
  switch (view.kind) {
    case "discover":
    case "repo":
      return "discover";
    case "installed":
    case "app":
      return "installed";
    case "running":
      return "running";
    case "settings":
      return "settings";
    default:
      return null;
  }
}
