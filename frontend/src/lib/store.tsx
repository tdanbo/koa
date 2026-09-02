import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  type ReactNode,
} from "react";

import { koa, on, win } from "./bridge";
import { errorMessage } from "./format";
import {
  EVENT,
  type Account,
  type App,
  type AppDetail,
  type AuthEvent,
  type Bootstrap,
  type Discovery,
  type InstallProgress,
  type LogLine,
  type PathState,
  type Process,
  type RepoDetail,
  type Settings,
  type SignInPrompt,
  type ThemeName,
  type Toast,
  type Version,
} from "./types";

export type AppTab = "overview" | "versions" | "readme";

export type View =
  | { kind: "signin" }
  | { kind: "discover" }
  | { kind: "repo"; owner: string; name: string }
  | { kind: "installed" }
  | { kind: "app"; owner: string; name: string; tab: AppTab }
  | { kind: "running" }
  | { kind: "settings" };

/** Async resource wrapper, so views can render loading and error states. */
export interface Resource<T> {
  data: T | null;
  loading: boolean;
  error: string;
}

const idle = <T,>(): Resource<T> => ({ data: null, loading: false, error: "" });

interface ToastItem extends Toast {
  id: number;
}

interface State {
  ready: boolean;
  fatal: string;
  boot: Bootstrap | null;
  account: Account;
  settings: Settings;
  path: PathState;
  systemDark: boolean;

  view: View;

  discovery: Resource<Discovery>;
  filter: string;

  repoDetail: Resource<RepoDetail>;
  appDetail: Resource<AppDetail>;
  versions: Resource<Version[]>;

  apps: App[];
  appsRefreshing: boolean;

  processes: Process[];
  logs: Record<string, LogLine[]>;
  activeProcess: string;

  /** installs is keyed by "owner/repo". */
  installs: Record<string, InstallProgress>;
  toasts: ToastItem[];

  signIn: {
    prompt: SignInPrompt | null;
    pending: boolean;
    error: string;
    tokenOpen: boolean;
  };

  /** trustPrompt holds a queued install awaiting the trust reminder (§18). */
  trustPrompt: { owner: string; name: string; tag: string } | null;
}

const emptyAccount: Account = {
  signedIn: false,
  login: "",
  name: "",
  avatarUrl: "",
  source: "",
  scopes: "",
  tokenStorage: "",
  usingPlaintextFallback: false,
  plaintextPath: "",
};

const emptySettings: Settings = {
  theme: "system",
  minimizeToTray: false,
  manualOrgs: [],
  trustAcknowledged: false,
};

const initialState: State = {
  ready: false,
  fatal: "",
  boot: null,
  account: emptyAccount,
  settings: emptySettings,
  path: { onPath: false, persisted: false, needsRestart: false, detail: "" },
  systemDark: true,
  view: { kind: "discover" },
  discovery: idle<Discovery>(),
  filter: "",
  repoDetail: idle<RepoDetail>(),
  appDetail: idle<AppDetail>(),
  versions: idle<Version[]>(),
  apps: [],
  appsRefreshing: false,
  processes: [],
  logs: {},
  activeProcess: "",
  installs: {},
  toasts: [],
  signIn: { prompt: null, pending: false, error: "", tokenOpen: false },
  trustPrompt: null,
};

type Action =
  | { type: "booted"; boot: Bootstrap }
  | { type: "fatal"; message: string }
  | { type: "account"; account: Account }
  | { type: "settings"; settings: Settings }
  | { type: "path"; path: PathState }
  | { type: "systemDark"; dark: boolean }
  | { type: "navigate"; view: View }
  | { type: "appTab"; tab: AppTab }
  | { type: "filter"; value: string }
  | { type: "discoveryLoading" }
  | { type: "discovery"; value: Resource<Discovery> }
  | { type: "repoDetail"; value: Resource<RepoDetail> }
  | { type: "appDetail"; value: Resource<AppDetail> }
  | { type: "versions"; value: Resource<Version[]> }
  | { type: "apps"; apps: App[] }
  | { type: "appsRefreshing"; value: boolean }
  | { type: "processes"; processes: Process[] }
  | { type: "process"; process: Process }
  | { type: "logs"; id: string; lines: LogLine[] }
  | { type: "logLine"; id: string; line: LogLine }
  | { type: "clearLog"; id: string }
  | { type: "activeProcess"; id: string }
  | { type: "install"; progress: InstallProgress }
  | { type: "installDone"; id: string }
  | { type: "toast"; toast: Toast }
  | { type: "dismissToast"; id: number }
  | { type: "signIn"; patch: Partial<State["signIn"]> }
  | { type: "trustPrompt"; value: State["trustPrompt"] };

const MAX_LOG_LINES = 5000;
let toastSeq = 0;

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case "booted": {
      const view: View = action.boot.account.signedIn
        ? { kind: "discover" }
        : { kind: "signin" };
      return {
        ...state,
        ready: true,
        boot: action.boot,
        account: action.boot.account,
        settings: action.boot.settings,
        path: action.boot.path,
        view,
      };
    }
    case "fatal":
      return { ...state, ready: true, fatal: action.message };
    case "account": {
      const signedOut = !action.account.signedIn;
      return {
        ...state,
        account: action.account,
        view: signedOut ? { kind: "signin" } : state.view,
        discovery: signedOut ? idle<Discovery>() : state.discovery,
      };
    }
    case "settings":
      return { ...state, settings: action.settings };
    case "path":
      return { ...state, path: action.path };
    case "systemDark":
      return { ...state, systemDark: action.dark };
    case "navigate":
      return { ...state, view: action.view };
    case "appTab":
      return state.view.kind === "app"
        ? { ...state, view: { ...state.view, tab: action.tab } }
        : state;
    case "filter":
      return { ...state, filter: action.value };
    case "discoveryLoading":
      return { ...state, discovery: { ...state.discovery, loading: true, error: "" } };
    case "discovery":
      return { ...state, discovery: action.value };
    case "repoDetail":
      return { ...state, repoDetail: action.value };
    case "appDetail":
      return { ...state, appDetail: action.value };
    case "versions":
      return { ...state, versions: action.value };
    case "apps":
      return { ...state, apps: action.apps };
    case "appsRefreshing":
      return { ...state, appsRefreshing: action.value };
    case "processes": {
      const active =
        state.activeProcess && action.processes.some((p) => p.id === state.activeProcess)
          ? state.activeProcess
          : (action.processes.at(-1)?.id ?? "");
      return { ...state, processes: action.processes, activeProcess: active };
    }
    case "process": {
      const existing = state.processes.findIndex((p) => p.id === action.process.id);
      const processes =
        existing >= 0
          ? state.processes.map((p) => (p.id === action.process.id ? action.process : p))
          : [...state.processes, action.process];
      return {
        ...state,
        processes,
        activeProcess: state.activeProcess || action.process.id,
      };
    }
    case "logs":
      return { ...state, logs: { ...state.logs, [action.id]: action.lines } };
    case "logLine": {
      const current = state.logs[action.id] ?? [];
      if (current.some((l) => l.seq === action.line.seq)) return state;
      const next = [...current, action.line];
      if (next.length > MAX_LOG_LINES) next.splice(0, next.length - MAX_LOG_LINES);
      return { ...state, logs: { ...state.logs, [action.id]: next } };
    }
    case "clearLog":
      return { ...state, logs: { ...state.logs, [action.id]: [] } };
    case "activeProcess":
      return { ...state, activeProcess: action.id };
    case "install":
      return {
        ...state,
        installs: { ...state.installs, [action.progress.id]: action.progress },
      };
    case "installDone": {
      const installs = { ...state.installs };
      delete installs[action.id];
      return { ...state, installs };
    }
    case "toast": {
      toastSeq += 1;
      return { ...state, toasts: [...state.toasts, { ...action.toast, id: toastSeq }] };
    }
    case "dismissToast":
      return { ...state, toasts: state.toasts.filter((t) => t.id !== action.id) };
    case "signIn":
      return { ...state, signIn: { ...state.signIn, ...action.patch } };
    case "trustPrompt":
      return { ...state, trustPrompt: action.value };
    default:
      return state;
  }
}

export interface Actions {
  goDiscover(): void;
  goInstalled(): void;
  goRunning(): void;
  goSettings(): void;
  goSignIn(): void;
  openRepo(owner: string, name: string): void;
  openApp(owner: string, name: string, tab?: AppTab): void;
  setAppTab(tab: AppTab): void;
  setFilter(value: string): void;

  refreshDiscovery(force: boolean): Promise<void>;
  refreshApps(): Promise<void>;
  checkAllForUpdates(): Promise<void>;

  install(owner: string, name: string, tag?: string): Promise<void>;
  confirmTrust(): Promise<void>;
  dismissTrust(): void;
  checkForUpdates(owner: string, name: string): Promise<void>;
  setAutoUpdate(owner: string, name: string, enabled: boolean): Promise<void>;
  uninstall(owner: string, name: string): Promise<void>;

  launch(owner: string, name: string): Promise<void>;
  selectProcess(id: string): void;
  stopProcess(id: string): Promise<void>;
  clearLog(id: string): Promise<void>;
  closeProcess(id: string): Promise<void>;

  signInWithGitHub(): Promise<void>;
  cancelSignIn(): Promise<void>;
  signInWithToken(token: string): Promise<void>;
  signOut(): Promise<void>;
  openTokenEntry(open: boolean): void;

  setTheme(theme: ThemeName): Promise<void>;
  setMinimizeToTray(enabled: boolean): Promise<void>;
  setManualOrgs(orgs: string[]): Promise<void>;
  ensurePath(): Promise<void>;
  revealBinFolder(): Promise<void>;
  openExternal(url: string): void;

  notify(toast: Toast): void;
  dismissToast(id: number): void;
}

const StateContext = createContext<State>(initialState);
const ActionsContext = createContext<Actions | null>(null);

/** installKey matches the id the backend stamps on install progress events. */
const installKey = (owner: string, name: string) =>
  `${owner.toLowerCase()}/${name.toLowerCase()}`;

export function StoreProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(reducer, initialState);

  // The reducer's latest state, for callbacks that must read without becoming
  // dependent on every change.
  const latest = useRef(state);
  latest.current = state;

  const notify = useCallback((toast: Toast) => {
    dispatch({ type: "toast", toast });
  }, []);

  const loadApps = useCallback(async () => {
    try {
      dispatch({ type: "apps", apps: await koa.installed() });
    } catch (err) {
      notify({ kind: "error", message: errorMessage(err) });
    }
  }, [notify]);

  const loadProcesses = useCallback(async () => {
    try {
      dispatch({ type: "processes", processes: await koa.listProcesses() });
    } catch {
      // The Running view simply stays empty if the shell is not ready yet.
    }
  }, []);

  const refreshDiscovery = useCallback(
    async (force: boolean) => {
      const current = latest.current;
      if (!current.account.signedIn) return;
      dispatch({ type: "discoveryLoading" });
      try {
        const data = await koa.discover(force);
        dispatch({ type: "discovery", value: { data, loading: false, error: "" } });
      } catch (err) {
        dispatch({
          type: "discovery",
          value: { data: current.discovery.data, loading: false, error: errorMessage(err) },
        });
      }
    },
    [],
  );

  const loadRepoDetail = useCallback(async (owner: string, name: string) => {
    dispatch({ type: "repoDetail", value: { data: null, loading: true, error: "" } });
    try {
      const data = await koa.repoDetail(owner, name);
      dispatch({ type: "repoDetail", value: { data, loading: false, error: "" } });
    } catch (err) {
      dispatch({
        type: "repoDetail",
        value: { data: null, loading: false, error: errorMessage(err) },
      });
    }
  }, []);

  const loadAppDetail = useCallback(async (owner: string, name: string) => {
    dispatch({ type: "appDetail", value: { data: null, loading: true, error: "" } });
    try {
      const data = await koa.appDetail(owner, name);
      dispatch({ type: "appDetail", value: { data, loading: false, error: "" } });
    } catch (err) {
      dispatch({
        type: "appDetail",
        value: { data: null, loading: false, error: errorMessage(err) },
      });
    }
  }, []);

  const loadVersions = useCallback(async (owner: string, name: string) => {
    dispatch({ type: "versions", value: { data: null, loading: true, error: "" } });
    try {
      const data = await koa.versions(owner, name);
      dispatch({ type: "versions", value: { data, loading: false, error: "" } });
    } catch (err) {
      dispatch({
        type: "versions",
        value: { data: null, loading: false, error: errorMessage(err) },
      });
    }
  }, []);

  const runInstall = useCallback(
    async (owner: string, name: string, tag: string) => {
      const key = installKey(owner, name);
      dispatch({
        type: "install",
        progress: { id: key, owner, repo: name, tag, stage: "resolving", done: 0, total: 0, error: "" },
      });
      try {
        const app = await koa.install(owner, name, tag);
        notify({
          kind: "success",
          message: `${app.name} ${app.version} installed to your koa bin folder.`,
        });
        await loadApps();
        await refreshDiscovery(false);
        const view = latest.current.view;
        if (view.kind === "app" && view.name === name) {
          await loadAppDetail(owner, name);
          if (view.tab === "versions") await loadVersions(owner, name);
        }
      } catch (err) {
        notify({ kind: "error", message: errorMessage(err) });
      } finally {
        dispatch({ type: "installDone", id: key });
      }
    },
    [loadAppDetail, loadApps, loadVersions, notify, refreshDiscovery],
  );

  const actions = useMemo<Actions>(() => {
    const navigate = (view: View) => dispatch({ type: "navigate", view });

    return {
      goDiscover() {
        navigate({ kind: "discover" });
        void refreshDiscovery(false);
      },
      goInstalled() {
        navigate({ kind: "installed" });
        void loadApps();
      },
      goRunning() {
        navigate({ kind: "running" });
        void loadProcesses();
      },
      goSettings() {
        navigate({ kind: "settings" });
      },
      goSignIn() {
        navigate({ kind: "signin" });
      },
      openRepo(owner, name) {
        navigate({ kind: "repo", owner, name });
        void loadRepoDetail(owner, name);
      },
      openApp(owner, name, tab = "overview") {
        navigate({ kind: "app", owner, name, tab });
        void loadAppDetail(owner, name);
        if (tab === "versions") void loadVersions(owner, name);
      },
      setAppTab(tab) {
        dispatch({ type: "appTab", tab });
        const view = latest.current.view;
        if (view.kind !== "app") return;
        if (tab === "versions") void loadVersions(view.owner, view.name);
      },
      setFilter(value) {
        dispatch({ type: "filter", value });
      },

      refreshDiscovery,
      refreshApps: loadApps,

      async checkAllForUpdates() {
        dispatch({ type: "appsRefreshing", value: true });
        try {
          const apps = await koa.checkAllForUpdates();
          dispatch({ type: "apps", apps });
          const pending = apps.filter((a) => a.hasUpdate).length;
          notify({
            kind: pending > 0 ? "info" : "success",
            message:
              pending === 0
                ? "Everything is up to date."
                : `${pending} ${pending === 1 ? "update" : "updates"} available.`,
          });
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        } finally {
          dispatch({ type: "appsRefreshing", value: false });
        }
      },

      async install(owner, name, tag = "") {
        // The trust reminder is shown once, before the first install (§18).
        if (!latest.current.settings.trustAcknowledged) {
          dispatch({ type: "trustPrompt", value: { owner, name, tag } });
          return;
        }
        await runInstall(owner, name, tag);
      },

      async confirmTrust() {
        const queued = latest.current.trustPrompt;
        dispatch({ type: "trustPrompt", value: null });
        try {
          dispatch({ type: "settings", settings: await koa.acknowledgeTrust() });
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
        if (queued) await runInstall(queued.owner, queued.name, queued.tag);
      },

      dismissTrust() {
        dispatch({ type: "trustPrompt", value: null });
      },

      async checkForUpdates(owner, name) {
        try {
          const app = await koa.checkForUpdates(owner, name);
          await loadApps();
          const view = latest.current.view;
          if (view.kind === "app" && view.name === name) {
            await loadAppDetail(owner, name);
          }
          notify({
            kind: app.hasUpdate ? "info" : "success",
            message: app.hasUpdate
              ? `${app.name} ${app.latestVersion} is available.`
              : `${app.name} is up to date.`,
          });
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
      },

      async setAutoUpdate(owner, name, enabled) {
        try {
          await koa.setAutoUpdate(owner, name, enabled);
          await loadApps();
          await loadAppDetail(owner, name);
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
      },

      async uninstall(owner, name) {
        try {
          await koa.uninstall(owner, name);
          notify({ kind: "info", message: `${name} uninstalled.` });
          await loadApps();
          await refreshDiscovery(false);
          navigate({ kind: "installed" });
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
      },

      async launch(owner, name) {
        try {
          const proc = await koa.launch(owner, name);
          dispatch({ type: "process", process: proc });
          dispatch({ type: "activeProcess", id: proc.id });
          dispatch({ type: "logs", id: proc.id, lines: await koa.processLogs(proc.id) });
          navigate({ kind: "running" });
          void loadApps();
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
      },

      selectProcess(id) {
        dispatch({ type: "activeProcess", id });
        void koa
          .processLogs(id)
          .then((lines) => dispatch({ type: "logs", id, lines }))
          .catch(() => {});
      },

      async stopProcess(id) {
        try {
          await koa.stopProcess(id);
          await loadProcesses();
          void loadApps();
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
      },

      async clearLog(id) {
        try {
          await koa.clearLogs(id);
          dispatch({ type: "clearLog", id });
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
      },

      async closeProcess(id) {
        try {
          await koa.closeProcess(id);
          await loadProcesses();
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
      },

      async signInWithGitHub() {
        dispatch({ type: "signIn", patch: { pending: true, error: "", prompt: null } });
        try {
          const prompt = await koa.signInWithGitHub();
          dispatch({ type: "signIn", patch: { prompt, pending: true } });
        } catch (err) {
          dispatch({
            type: "signIn",
            patch: { pending: false, error: errorMessage(err), prompt: null },
          });
        }
      },

      async cancelSignIn() {
        try {
          await koa.cancelSignIn();
        } finally {
          dispatch({ type: "signIn", patch: { pending: false, prompt: null } });
        }
      },

      async signInWithToken(token) {
        dispatch({ type: "signIn", patch: { pending: true, error: "" } });
        try {
          const account = await koa.signInWithToken(token);
          dispatch({ type: "account", account });
          dispatch({
            type: "signIn",
            patch: { pending: false, prompt: null, tokenOpen: false, error: "" },
          });
          navigate({ kind: "discover" });
          await refreshDiscovery(true);
          await loadApps();
        } catch (err) {
          dispatch({ type: "signIn", patch: { pending: false, error: errorMessage(err) } });
        }
      },

      async signOut() {
        try {
          await koa.signOut();
          dispatch({ type: "account", account: emptyAccount });
          dispatch({ type: "discovery", value: idle<Discovery>() });
          navigate({ kind: "signin" });
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
      },

      openTokenEntry(open) {
        dispatch({ type: "signIn", patch: { tokenOpen: open, error: "" } });
      },

      async setTheme(theme) {
        try {
          dispatch({ type: "settings", settings: await koa.setTheme(theme) });
          void win.syncTrayIcon();
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
      },

      async setMinimizeToTray(enabled) {
        try {
          dispatch({ type: "settings", settings: await koa.setMinimizeToTray(enabled) });
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
      },

      async setManualOrgs(orgs) {
        try {
          dispatch({ type: "settings", settings: await koa.setManualOrgs(orgs) });
          await refreshDiscovery(true);
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
      },

      async ensurePath() {
        try {
          const path = await koa.ensurePath();
          dispatch({ type: "path", path });
          notify({ kind: path.onPath || path.persisted ? "success" : "warning", message: `koa bin folder ${path.detail}.` });
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
      },

      async revealBinFolder() {
        try {
          await koa.revealBinFolder();
        } catch (err) {
          notify({ kind: "error", message: errorMessage(err) });
        }
      },

      openExternal(url) {
        void koa.openExternal(url).catch(() => {});
      },

      notify,
      dismissToast(id) {
        dispatch({ type: "dismissToast", id });
      },
    };
  }, [
    loadAppDetail,
    loadApps,
    loadProcesses,
    loadRepoDetail,
    loadVersions,
    notify,
    refreshDiscovery,
    runInstall,
  ]);

  // Bootstrap.
  useEffect(() => {
    let cancelled = false;
    void (async () => {
      try {
        const boot = await koa.bootstrap();
        if (cancelled) return;
        dispatch({ type: "booted", boot });
        if (boot.startupError) {
          dispatch({ type: "toast", toast: { kind: "warning", message: boot.startupError } });
        }
        if (boot.account.signedIn) {
          await Promise.all([refreshDiscovery(true), loadApps(), loadProcesses()]);
        }
      } catch (err) {
        if (!cancelled) dispatch({ type: "fatal", message: errorMessage(err) });
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [loadApps, loadProcesses, refreshDiscovery]);

  // Backend events.
  useEffect(() => {
    const offs = [
      on<AuthEvent>(EVENT.auth, (event) => {
        if (event.status === "signed-in") {
          dispatch({ type: "account", account: event.account });
          dispatch({ type: "signIn", patch: { pending: false, prompt: null, error: "" } });
          dispatch({ type: "navigate", view: { kind: "discover" } });
          void refreshDiscovery(true);
          void loadApps();
          return;
        }
        dispatch({
          type: "signIn",
          patch: { pending: false, prompt: null, error: event.error },
        });
      }),
      on<InstallProgress>(EVENT.install, (progress) => {
        if (progress.stage === "failed") {
          dispatch({ type: "installDone", id: progress.id });
          return;
        }
        dispatch({ type: "install", progress });
      }),
      on<App[]>(EVENT.apps, (apps) => dispatch({ type: "apps", apps: apps ?? [] })),
      on<Process>(EVENT.process, (process) => dispatch({ type: "process", process })),
      on<{ id: string; line: LogLine }>(EVENT.log, ({ id, line }) =>
        dispatch({ type: "logLine", id, line }),
      ),
      on<Toast>(EVENT.toast, (toast) => dispatch({ type: "toast", toast })),
    ];
    return () => offs.forEach((off) => off());
  }, [loadApps, refreshDiscovery]);

  // Follow the OS appearance when the theme is set to System (PRD §15).
  useEffect(() => {
    const query = window.matchMedia("(prefers-color-scheme: dark)");
    const apply = () => dispatch({ type: "systemDark", dark: query.matches });
    apply();
    query.addEventListener("change", apply);
    return () => query.removeEventListener("change", apply);
  }, []);

  const dark =
    state.settings.theme === "dark" ||
    (state.settings.theme === "system" && state.systemDark);

  useEffect(() => {
    document.documentElement.classList.toggle("dark", dark);
    document.documentElement.style.colorScheme = dark ? "dark" : "light";
  }, [dark]);

  return (
    <StateContext.Provider value={state}>
      <ActionsContext.Provider value={actions}>{children}</ActionsContext.Provider>
    </StateContext.Provider>
  );
}

export function useStore(): State {
  return useContext(StateContext);
}

export function useActions(): Actions {
  const actions = useContext(ActionsContext);
  if (!actions) throw new Error("useActions must be used inside StoreProvider");
  return actions;
}

/** useDark reports the resolved appearance, for anything that needs it in JS. */
export function useDark(): boolean {
  const { settings, systemDark } = useStore();
  return settings.theme === "dark" || (settings.theme === "system" && systemDark);
}
