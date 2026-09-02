// Mirrors the view models in internal/app/views.go. Go's time.Time marshals to
// an RFC 3339 string, so every timestamp is typed as a string here.

export type StatusKind =
  | "none"
  | "installed"
  | "update"
  | "incompatible"
  | "norelease";

export type ThemeName = "light" | "dark" | "system";

export interface Account {
  signedIn: boolean;
  login: string;
  name: string;
  avatarUrl: string;
  source: string;
  scopes: string;
  tokenStorage: string;
  usingPlaintextFallback: boolean;
  plaintextPath: string;
}

export interface Settings {
  theme: ThemeName;
  minimizeToTray: boolean;
  manualOrgs: string[];
  trustAcknowledged: boolean;
}

export interface PathState {
  onPath: boolean;
  persisted: boolean;
  needsRestart: boolean;
  detail: string;
}

export interface Bootstrap {
  version: string;
  platform: string;
  arch: string;
  binDir: string;
  binDirAbsolute: string;
  stateFile: string;
  account: Account;
  settings: Settings;
  path: PathState;
  assetPattern: string;
  deviceFlowReady: boolean;
  startupError: string;
}

export interface Repo {
  id: string;
  owner: string;
  name: string;
  description: string;
  visibility: string;
  htmlUrl: string;
  ownerScope: string;
  status: string;
  statusKind: StatusKind;
  action: string;
  canInstall: boolean;
  installed: boolean;
  installedVersion: string;
  incompatible: boolean;
  incompatibleReason: string;
  latestVersion: string;
  publishedAt: string;
  assetName: string;
  assetSize: number;
}

export interface ScopeError {
  scope: string;
  message: string;
  sso: boolean;
  ssoUrl: string;
}

export interface Discovery {
  repos: Repo[];
  scopes: string[];
  errors: ScopeError[];
  refreshedAt: string;
  signedIn: boolean;
}

export interface RepoDetail {
  repo: Repo;
  readmeHtml: string;
  readmeError: string;
}

export interface App {
  id: string;
  owner: string;
  name: string;
  description: string;
  visibility: string;
  version: string;
  latestVersion: string;
  hasUpdate: boolean;
  running: boolean;
  autoUpdate: boolean;
  lastChecked: string;
  installedAt: string;
  publishedAt: string;
  binaryPath: string;
  binaryPathAbsolute: string;
  command: string;
  assetName: string;
  sizeBytes: number;
  missing: boolean;
}

export interface AppDetail {
  app: App;
  latestPublishedAt: string;
  readmeHtml: string;
  readmeError: string;
}

export interface Version {
  tag: string;
  publishedAt: string;
  sizeBytes: number;
  isCurrent: boolean;
  isLatest: boolean;
  compatible: boolean;
  action: string;
}

export interface InstallProgress {
  id: string;
  owner: string;
  repo: string;
  tag: string;
  stage: string;
  done: number;
  total: number;
  error: string;
}

export interface SignInPrompt {
  userCode: string;
  verificationUri: string;
  expiresAt: string;
  browserOpened: boolean;
}

export interface AuthEvent {
  status: "signed-in" | "failed" | "cancelled";
  account: Account;
  error: string;
}

export type LogStream = "stdout" | "stderr" | "system";

export interface LogLine {
  seq: number;
  time: string;
  stream: LogStream;
  text: string;
}

export interface Process {
  id: string;
  owner: string;
  repo: string;
  command: string;
  pid: number;
  startedAt: string;
  exitedAt: string;
  running: boolean;
  exitCode: number;
  failure: string;
}

export interface Toast {
  kind: "info" | "success" | "warning" | "error";
  message: string;
}

export const EVENT = {
  auth: "koa:auth",
  install: "koa:install",
  apps: "koa:apps",
  log: "koa:log",
  process: "koa:process",
  toast: "koa:toast",
} as const;
