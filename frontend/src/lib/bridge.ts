import type {
  Account,
  App,
  AppDetail,
  Bootstrap,
  Discovery,
  LogLine,
  PathState,
  Process,
  RepoDetail,
  SelfUpdateInfo,
  Settings,
  SignInPrompt,
  ThemeName,
  Version,
} from "./types";

/** True when running inside the Wails shell rather than a plain browser. */
export const isDesktop = (): boolean => typeof window.go?.main?.Koa === "object";

export type Backend = {
  call<T>(method: string, ...args: unknown[]): Promise<T>;
  window(method: string, ...args: unknown[]): Promise<void>;
  on(event: string, handler: (payload: unknown) => void): () => void;
  openURL(url: string): void;
};

let backend: Backend | null = null;

/** installBackend lets the dev mock take over when no shell is present. */
export function installBackend(impl: Backend): void {
  backend = impl;
}

const wailsBackend: Backend = {
  async call<T>(method: string, ...args: unknown[]): Promise<T> {
    const api = window.go?.main?.Koa;
    const fn = api?.[method];
    if (typeof fn !== "function") {
      throw new Error(`koa: the desktop bridge is missing ${method}`);
    }
    return (await fn(...args)) as T;
  },
  async window(method: string, ...args: unknown[]): Promise<void> {
    const api = window.go?.main?.Window;
    const fn = api?.[method];
    if (typeof fn !== "function") return;
    await fn(...args);
  },
  on(event, handler) {
    const runtime = window.runtime;
    if (!runtime) return () => {};
    return runtime.EventsOn(event, (...data: unknown[]) => handler(data[0]));
  },
  openURL(url) {
    window.runtime?.BrowserOpenURL(url);
  },
};

function active(): Backend {
  if (backend) return backend;
  return wailsBackend;
}

const call = <T>(method: string, ...args: unknown[]): Promise<T> =>
  active().call<T>(method, ...args);

/** koa is the typed view of the Go service bound at `window.go.main.Koa`. */
export const koa = {
  bootstrap: () => call<Bootstrap>("Bootstrap"),

  signInWithGitHub: () => call<SignInPrompt>("SignInWithGitHub"),
  cancelSignIn: () => call<void>("CancelSignIn"),
  signInWithToken: (token: string) => call<Account>("SignInWithToken", token),
  signOut: () => call<void>("SignOut"),
  refreshAccount: () => call<Account>("RefreshAccount"),

  discover: (refresh: boolean) => call<Discovery>("Discover", refresh),
  repoDetail: (owner: string, name: string) =>
    call<RepoDetail>("RepoDetail", owner, name),
  readme: (owner: string, name: string) => call<string>("Readme", owner, name),

  installed: () => call<App[]>("Installed"),
  appDetail: (owner: string, name: string) =>
    call<AppDetail>("AppDetail", owner, name),
  versions: (owner: string, name: string) =>
    call<Version[]>("Versions", owner, name),
  install: (owner: string, name: string, tag: string) =>
    call<App>("Install", owner, name, tag),
  checkForUpdates: (owner: string, name: string) =>
    call<App>("CheckForUpdates", owner, name),
  checkAllForUpdates: () => call<App[]>("CheckAllForUpdates"),
  setAutoUpdate: (owner: string, name: string, enabled: boolean) =>
    call<App>("SetAutoUpdate", owner, name, enabled),
  uninstall: (owner: string, name: string) =>
    call<void>("Uninstall", owner, name),

  launch: (owner: string, name: string) => call<Process>("Launch", owner, name),
  listProcesses: () => call<Process[]>("ListProcesses"),
  processLogs: (id: string) => call<LogLine[]>("ProcessLogs", id),
  stopProcess: (id: string) => call<void>("StopProcess", id),
  clearLogs: (id: string) => call<void>("ClearLogs", id),
  closeProcess: (id: string) => call<void>("CloseProcess", id),

  getSettings: () => call<Settings>("GetSettings"),
  setTheme: (theme: ThemeName) => call<Settings>("SetTheme", theme),
  setMinimizeToTray: (enabled: boolean) =>
    call<Settings>("SetMinimizeToTray", enabled),
  setManualOrgs: (orgs: string[]) => call<Settings>("SetManualOrgs", orgs),
  acknowledgeTrust: () => call<Settings>("AcknowledgeTrust"),
  ensurePath: () => call<PathState>("EnsurePath"),
  pathStatus: () => call<PathState>("PathStatus"),
  revealBinFolder: () => call<void>("RevealBinFolder"),
  openExternal: (url: string) => call<void>("OpenExternal", url),

  checkSelfUpdate: () => call<SelfUpdateInfo>("CheckSelfUpdate"),
  selfUpdateStatus: () => call<SelfUpdateInfo>("SelfUpdateStatus"),
  selfUpdate: () => call<void>("SelfUpdate"),
};

/** win drives koa's custom title bar, since the window is frameless. */
export const win = {
  minimise: () => active().window("Minimise"),
  toggleMaximise: () => active().window("ToggleMaximise"),
  close: () => active().window("Close"),
  syncTrayIcon: () => active().window("SyncTrayIcon"),
};

/** on subscribes to a backend event and returns an unsubscribe function. */
export function on<T>(event: string, handler: (payload: T) => void): () => void {
  return active().on(event, (payload) => handler(payload as T));
}

/** openURL sends a link to the user's browser rather than the webview. */
export function openURL(url: string): void {
  active().openURL(url);
}
