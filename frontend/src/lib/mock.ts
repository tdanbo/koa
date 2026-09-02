/**
 * Dev-only stand-in for the Wails bridge.
 *
 * It exists so `npm run dev` renders every view in a plain browser, which is
 * how the build gets compared side by side with PRD/UI/koa.dc.html. The sample
 * repos, versions and log lines are the reference's placeholder data — they are
 * never shipped and never reach the desktop build, which always talks to Go.
 */

import { installBackend, type Backend } from "./bridge";
import type {
  Account,
  App,
  Bootstrap,
  Discovery,
  LogLine,
  PathState,
  Process,
  Repo,
  SelfUpdateInfo,
  Settings,
  Version,
} from "./types";

const WINDOWS = false;

const account: Account = {
  signedIn: true,
  login: "m-halvorsen",
  name: "M Halvorsen",
  avatarUrl: "",
  source: "device",
  scopes: "repo read:org",
  tokenStorage: WINDOWS ? "Windows Credential Manager" : "the system keyring",
  usingPlaintextFallback: false,
  plaintextPath: "",
};

let settings: Settings = {
  theme: "dark",
  minimizeToTray: true,
  manualOrgs: [],
  trustAcknowledged: true,
};

const path: PathState = {
  onPath: true,
  persisted: true,
  needsRestart: false,
  detail: "on your PATH",
};

const binDir = WINDOWS ? "%LOCALAPPDATA%\\koa\\bin" : "~/.koa/bin";
const command = (name: string) => (WINDOWS ? `${name}.exe` : name);
const binary = (name: string) => `${binDir}${WINDOWS ? "\\" : "/"}${command(name)}`;
const asset = (name: string, version: string) =>
  WINDOWS
    ? `${name}-${version}-amd64-windows.zip`
    : `${name}-${version}-amd64-linux.tar.gz`;

const ago = (minutes: number) =>
  new Date(Date.now() - minutes * 60_000).toISOString();

interface Seed {
  repo: Repo;
  readme: string;
  installed?: { version: string; checkedMinutes: number; autoUpdate: boolean };
}

const seeds: Seed[] = [
  {
    repo: {
      id: "playdead/pace-cli",
      owner: "playdead",
      name: "pace-cli",
      description:
        "Command-line driver for Pace build pipelines — schedule, inspect and cancel jobs.",
      visibility: "Private",
      htmlUrl: "https://github.com/playdead/pace-cli",
      ownerScope: "playdead",
      status: "Installed v2.4.1",
      statusKind: "installed",
      action: "Open",
      canInstall: true,
      installed: true,
      installedVersion: "v2.4.1",
      incompatible: false,
      incompatibleReason: "",
      latestVersion: "v2.4.1",
      publishedAt: "2026-08-12T09:00:00Z",
      assetName: asset("pace-cli", "2.4.1"),
      assetSize: 7_100_000,
    },
    readme:
      "# pace-cli\n\npace-cli talks to the Pace scheduler over the same API the editor uses, so anything you can do from the build dashboard you can script.\n\n## Installation\n\n```sh\nkoa install pace-cli\n```\n\n## Usage\n\n- Point it at a project directory and it watches for changes.\n- Results stream to stdout; pass `--json` for machine output.\n- Exit code is non-zero when any check fails.\n",
    installed: { version: "v2.4.1", checkedMinutes: 120, autoUpdate: false },
  },
  {
    repo: {
      id: "playdead/dumpscope",
      owner: "playdead",
      name: "dumpscope",
      description: "Open, diff and watch Pace debug dumps without leaving the terminal.",
      visibility: "Private",
      htmlUrl: "https://github.com/playdead/dumpscope",
      ownerScope: "playdead",
      status: "Update available",
      statusKind: "update",
      action: "Update",
      canInstall: true,
      installed: true,
      installedVersion: "v0.9.2",
      incompatible: false,
      incompatibleReason: "",
      latestVersion: "v1.0.0",
      publishedAt: "2026-08-30T10:00:00Z",
      assetName: asset("dumpscope", "1.0.0"),
      assetSize: 6_400_000,
    },
    readme:
      "# dumpscope\n\ndumpscope reads recorded animation, movement and state-machine dumps and renders them as a readable timeline you can scrub, diff and filter.\n\n## Commands\n\n```sh\ndumpscope open <file.pacedump>\ndumpscope diff <a> <b> --json\ndumpscope watch ./Saved/Pace\n```\n\nFetched from the repository readme endpoint, not from the release archive — the same path Discover uses before anything is installed.\n",
    installed: { version: "v0.9.2", checkedMinutes: 4, autoUpdate: false },
  },
  {
    repo: {
      id: "playdead/assetlint",
      owner: "playdead",
      name: "assetlint",
      description: "Validates naming, budgets and import settings across an asset library.",
      visibility: "Public",
      htmlUrl: "https://github.com/playdead/assetlint",
      ownerScope: "playdead",
      status: "Not installed",
      statusKind: "none",
      action: "Install",
      canInstall: true,
      installed: false,
      installedVersion: "",
      incompatible: false,
      incompatibleReason: "",
      latestVersion: "v1.3.0",
      publishedAt: "2026-07-21T09:00:00Z",
      assetName: asset("assetlint", "1.3.0"),
      assetSize: 4_200_000,
    },
    readme:
      "# assetlint\n\nassetlint walks a project directory and reports every asset that breaks a rule in your ruleset file, with a non-zero exit code for CI.\n",
  },
  {
    repo: {
      id: "m-halvorsen/rigcheck",
      owner: "m-halvorsen",
      name: "rigcheck",
      description: "Skeleton and constraint sanity checks for character rigs.",
      visibility: "Private",
      htmlUrl: "https://github.com/m-halvorsen/rigcheck",
      ownerScope: "you",
      status: "Incompatible",
      statusKind: "incompatible",
      action: "",
      canInstall: false,
      installed: false,
      installedVersion: "",
      incompatible: true,
      incompatibleReason: `No release asset matches ${asset("rigcheck", "{version}").replace("{version}", "{version}")}. Latest release v0.6.1 publishes no compatible binary.`,
      latestVersion: "v0.6.1",
      publishedAt: "2026-06-02T09:00:00Z",
      assetName: "",
      assetSize: 0,
    },
    readme:
      "# rigcheck\n\nrigcheck validates joint hierarchies and constraint cycles before a rig reaches the animation team.\n",
  },
  {
    repo: {
      id: "playdead-tools/buildwatch",
      owner: "playdead-tools",
      name: "buildwatch",
      description: "Tray notifier for build and test results on the shared farm.",
      visibility: "Public",
      htmlUrl: "https://github.com/playdead-tools/buildwatch",
      ownerScope: "playdead-tools",
      status: "Not installed",
      statusKind: "none",
      action: "Install",
      canInstall: true,
      installed: false,
      installedVersion: "",
      incompatible: false,
      incompatibleReason: "",
      latestVersion: "v0.11.4",
      publishedAt: "2026-08-27T09:00:00Z",
      assetName: asset("buildwatch", "0.11.4"),
      assetSize: 3_100_000,
    },
    readme:
      "# buildwatch\n\nbuildwatch subscribes to farm events and shows a quiet notification when a build you care about changes state.\n",
  },
  {
    repo: {
      id: "m-halvorsen/shaderpack",
      owner: "m-halvorsen",
      name: "shaderpack",
      description: "Packs and hashes shader variants into a single distributable bundle.",
      visibility: "Private",
      htmlUrl: "https://github.com/m-halvorsen/shaderpack",
      ownerScope: "you",
      status: "Installed v0.4.0",
      statusKind: "installed",
      action: "Open",
      canInstall: true,
      installed: true,
      installedVersion: "v0.4.0",
      incompatible: false,
      incompatibleReason: "",
      latestVersion: "v0.4.0",
      publishedAt: "2026-08-09T09:00:00Z",
      assetName: asset("shaderpack", "0.4.0"),
      assetSize: 5_900_000,
    },
    readme:
      "# shaderpack\n\nshaderpack collapses a variant tree into one deterministic bundle so builds stay reproducible across machines.\n",
    installed: { version: "v0.4.0", checkedMinutes: 1440, autoUpdate: true },
  },
];

const seedFor = (owner: string, name: string) =>
  seeds.find(
    (s) =>
      s.repo.owner.toLowerCase() === owner.toLowerCase() &&
      s.repo.name.toLowerCase() === name.toLowerCase(),
  );

function appFor(seed: Seed): App {
  const installed = seed.installed!;
  return {
    id: seed.repo.id,
    owner: seed.repo.owner,
    name: seed.repo.name,
    description: seed.repo.description,
    visibility: seed.repo.visibility,
    version: installed.version,
    latestVersion: seed.repo.latestVersion,
    hasUpdate: seed.repo.latestVersion !== installed.version,
    running: runningRepos().has(seed.repo.name),
    autoUpdate: installed.autoUpdate,
    lastChecked: ago(installed.checkedMinutes),
    installedAt: ago(installed.checkedMinutes + 600),
    publishedAt: seed.repo.publishedAt,
    binaryPath: binary(seed.repo.name),
    binaryPathAbsolute: binary(seed.repo.name),
    command: command(seed.repo.name),
    assetName: asset(seed.repo.name, installed.version.replace(/^v/, "")),
    sizeBytes: seed.repo.assetSize || 6_200_000,
    missing: false,
  };
}

const versionsFor = (name: string): Version[] => {
  const seed = seeds.find((s) => s.repo.name === name);
  const current = seed?.installed?.version ?? "";
  const rows = [
    { tag: "v1.0.0", publishedAt: "2026-08-30T10:00:00Z", sizeBytes: 6_400_000 },
    { tag: "v0.9.2", publishedAt: "2026-08-11T10:00:00Z", sizeBytes: 6_200_000 },
    { tag: "v0.9.1", publishedAt: "2026-07-28T10:00:00Z", sizeBytes: 6_200_000 },
    { tag: "v0.9.0", publishedAt: "2026-07-14T10:00:00Z", sizeBytes: 6_100_000 },
    { tag: "v0.8.3", publishedAt: "2026-06-02T10:00:00Z", sizeBytes: 5_900_000 },
  ];
  return rows.map((row, index) => ({
    ...row,
    isCurrent: row.tag === current,
    isLatest: index === 0,
    compatible: true,
    action: row.tag === current ? "Reinstall" : index === 0 ? "Update" : "Roll back",
  }));
};

const seedLogs: Record<string, Array<[string, LogLine["stream"], string]>> = {
  "pace-cli": [
    ["10:04:02", "system", "pace-cli 2.4.1 (linux/amd64)"],
    ["10:04:02", "stdout", "reading config from ~/.config/pace/cli.toml"],
    ["10:04:03", "stdout", "connected to scheduler pace-farm-01"],
    ["10:04:03", "stdout", 'watching queue "animation" (14 jobs)'],
    ["10:04:19", "stdout", "job 8841 → running   rig-bake/full"],
    ["10:04:41", "stdout", "job 8841 → complete  38.2s"],
    ["10:05:12", "stdout", "job 8842 → running   anim-export/batch"],
    ["10:06:03", "stderr", "warn: shard 3 responded in 4.1s (threshold 2.0s)"],
    ["10:06:44", "stdout", "job 8842 → complete  91.7s"],
    ["10:07:10", "stdout", "idle — 0 jobs queued"],
    ["10:08:13", "system", "heartbeat ok"],
  ],
  dumpscope: [
    ["09:52:41", "system", "dumpscope 0.9.2 (linux/amd64)"],
    ["09:52:41", "stdout", "open ~/Saved/Pace/2026-09-02-0941.pacedump"],
    ["09:52:42", "stdout", "parsed 4,812 frames · 61 tracks"],
    ["09:52:42", "stderr", "warn: 3 IK solver oscillations detected"],
    ["09:53:07", "stdout", "exported report to ./dumpscope-report.json"],
    ["09:53:07", "system", "process exited with code 0"],
  ],
};

let seq = 0;
const processes: Process[] = [
  {
    id: "pace-cli-1",
    owner: "playdead",
    repo: "pace-cli",
    command: binary("pace-cli"),
    pid: 48213,
    startedAt: ago(4),
    exitedAt: "0001-01-01T00:00:00Z",
    running: true,
    exitCode: 0,
    failure: "",
  },
  {
    id: "dumpscope-1",
    owner: "playdead",
    repo: "dumpscope",
    command: binary("dumpscope"),
    pid: 47120,
    startedAt: ago(16),
    exitedAt: ago(15),
    running: false,
    exitCode: 0,
    failure: "",
  },
];

const logs: Record<string, LogLine[]> = Object.fromEntries(
  processes.map((process) => [
    process.id,
    (seedLogs[process.repo] ?? []).map(([clock, stream, text]) => {
      seq += 1;
      const [h, m, s] = clock.split(":").map(Number);
      const when = new Date();
      when.setHours(h, m, s, 0);
      return { seq, time: when.toISOString(), stream, text };
    }),
  ]),
);

function runningRepos(): Set<string> {
  return new Set(processes.filter((p) => p.running).map((p) => p.repo));
}

const listeners = new Map<string, Set<(payload: unknown) => void>>();

function emit(event: string, payload: unknown) {
  listeners.get(event)?.forEach((handler) => handler(payload));
}

const delay = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

const bootstrap: Bootstrap = {
  version: "0.1.0-dev",
  platform: WINDOWS ? "windows" : "linux",
  arch: "amd64",
  binDir,
  binDirAbsolute: binDir,
  stateFile: WINDOWS ? "%APPDATA%\\koa\\state.json" : "~/.config/koa/state.json",
  account,
  settings,
  path,
  assetPattern: asset("{repo}", "{version}"),
  deviceFlowReady: true,
  startupError: "",
};

// koa's own update banner, illustrated the same way the reference illustrates
// a per-app update: sample data only, never shipped.
let selfUpdate: SelfUpdateInfo = {
  available: true,
  configured: true,
  current: bootstrap.version,
  latest: "0.2.0",
  publishedAt: ago(60 * 24),
  url: "https://github.com/tdanbo/koa/releases/tag/v0.2.0",
};

const handlers: Record<string, (...args: unknown[]) => Promise<unknown>> = {
  Bootstrap: async () => ({ ...bootstrap, settings }),

  Discover: async (): Promise<Discovery> => {
    await delay(120);
    return {
      repos: seeds.map((s) => ({ ...s.repo })),
      scopes: ["user:m-halvorsen", "org:playdead", "org:playdead-tools"],
      errors: [],
      refreshedAt: ago(4),
      signedIn: true,
    };
  },

  RepoDetail: async (owner, name) => {
    await delay(90);
    const seed = seedFor(String(owner), String(name));
    if (!seed) throw new Error("repository not found");
    return { repo: { ...seed.repo }, readmeHtml: renderMarkdown(seed.readme), readmeError: "" };
  },

  Readme: async (owner, name) => {
    await delay(90);
    return renderMarkdown(seedFor(String(owner), String(name))?.readme ?? "");
  },

  Installed: async () => seeds.filter((s) => s.installed).map(appFor),

  AppDetail: async (owner, name) => {
    const seed = seedFor(String(owner), String(name));
    if (!seed?.installed) throw new Error("not installed");
    return {
      app: appFor(seed),
      latestPublishedAt: seed.repo.publishedAt,
      readmeHtml: "",
      readmeError: "",
    };
  },

  Versions: async (_owner, name) => {
    await delay(120);
    return versionsFor(String(name));
  },

  Install: async (owner, name, tag) => {
    const seed = seedFor(String(owner), String(name));
    if (!seed) throw new Error("repository not found");
    const id = seed.repo.id;
    const total = seed.repo.assetSize || 6_000_000;
    for (const stage of ["resolving", "downloading", "extracting", "installing", "done"]) {
      if (stage === "downloading") {
        for (const fraction of [0.25, 0.6, 0.9, 1]) {
          emit("koa:install", {
            id, owner, repo: name, tag, stage,
            done: Math.round(total * fraction), total, error: "",
          });
          await delay(120);
        }
        continue;
      }
      emit("koa:install", { id, owner, repo: name, tag, stage, done: 0, total: 0, error: "" });
      await delay(140);
    }
    const version = String(tag || seed.repo.latestVersion);
    seed.installed = { version, checkedMinutes: 0, autoUpdate: seed.installed?.autoUpdate ?? false };
    seed.repo.installed = true;
    seed.repo.installedVersion = version;
    seed.repo.status = version === seed.repo.latestVersion ? `Installed ${version}` : "Update available";
    seed.repo.statusKind = version === seed.repo.latestVersion ? "installed" : "update";
    seed.repo.action = version === seed.repo.latestVersion ? "Open" : "Update";
    emit("koa:apps", seeds.filter((s) => s.installed).map(appFor));
    return appFor(seed);
  },

  CheckForUpdates: async (owner, name) => {
    await delay(200);
    const seed = seedFor(String(owner), String(name));
    if (!seed?.installed) throw new Error("not installed");
    seed.installed.checkedMinutes = 0;
    return appFor(seed);
  },

  CheckAllForUpdates: async () => {
    await delay(320);
    seeds.forEach((s) => {
      if (s.installed) s.installed.checkedMinutes = 0;
    });
    return seeds.filter((s) => s.installed).map(appFor);
  },

  SetAutoUpdate: async (owner, name, enabled) => {
    const seed = seedFor(String(owner), String(name));
    if (!seed?.installed) throw new Error("not installed");
    seed.installed.autoUpdate = Boolean(enabled);
    return appFor(seed);
  },

  Uninstall: async (owner, name) => {
    const seed = seedFor(String(owner), String(name));
    if (!seed) return;
    seed.installed = undefined;
    seed.repo.installed = false;
    seed.repo.installedVersion = "";
    seed.repo.status = "Not installed";
    seed.repo.statusKind = "none";
    seed.repo.action = "Install";
    emit("koa:apps", seeds.filter((s) => s.installed).map(appFor));
  },

  Launch: async (owner, name) => {
    const process: Process = {
      id: `${name}-${processes.length + 1}`,
      owner: String(owner),
      repo: String(name),
      command: binary(String(name)),
      pid: 40000 + processes.length * 137,
      startedAt: new Date().toISOString(),
      exitedAt: "0001-01-01T00:00:00Z",
      running: true,
      exitCode: 0,
      failure: "",
    };
    processes.push(process);
    logs[process.id] = [];
    emit("koa:process", process);

    void (async () => {
      for (const text of [
        `started ${process.command} (pid ${process.pid})`,
        "reading configuration",
        "ready",
      ]) {
        await delay(400);
        seq += 1;
        const line: LogLine = {
          seq,
          time: new Date().toISOString(),
          stream: text.startsWith("started") ? "system" : "stdout",
          text,
        };
        logs[process.id].push(line);
        emit("koa:log", { id: process.id, line });
      }
    })();

    return process;
  },

  ListProcesses: async () => processes.map((p) => ({ ...p })),
  ProcessLogs: async (id) => [...(logs[String(id)] ?? [])],

  StopProcess: async (id) => {
    const process = processes.find((p) => p.id === String(id));
    if (!process) return;
    process.running = false;
    process.exitedAt = new Date().toISOString();
    emit("koa:process", { ...process });
  },

  ClearLogs: async (id) => {
    logs[String(id)] = [];
  },

  CloseProcess: async (id) => {
    const index = processes.findIndex((p) => p.id === String(id));
    if (index >= 0) processes.splice(index, 1);
    delete logs[String(id)];
  },

  GetSettings: async () => settings,
  SetTheme: async (theme) => {
    settings = { ...settings, theme: theme as Settings["theme"] };
    return settings;
  },
  SetMinimizeToTray: async (enabled) => {
    settings = { ...settings, minimizeToTray: Boolean(enabled) };
    return settings;
  },
  SetManualOrgs: async (orgs) => {
    settings = { ...settings, manualOrgs: (orgs as string[]) ?? [] };
    return settings;
  },
  AcknowledgeTrust: async () => {
    settings = { ...settings, trustAcknowledged: true };
    return settings;
  },
  EnsurePath: async () => path,
  PathStatus: async () => path,
  RevealBinFolder: async () => undefined,
  OpenExternal: async (url) => {
    window.open(String(url), "_blank", "noopener");
  },

  SignInWithGitHub: async () => {
    await delay(200);
    return {
      userCode: "WQBD-2F4C",
      verificationUri: "https://github.com/login/device",
      expiresAt: new Date(Date.now() + 900_000).toISOString(),
      browserOpened: false,
    };
  },
  CancelSignIn: async () => undefined,
  SignInWithToken: async () => account,
  SignOut: async () => undefined,
  RefreshAccount: async () => account,

  CheckSelfUpdate: async () => {
    await delay(150);
    return selfUpdate;
  },
  SelfUpdateStatus: async () => selfUpdate,
  SelfUpdate: async () => {
    const total = 9_500_000;
    for (const stage of ["resolving", "downloading", "extracting", "installing", "relaunching"]) {
      if (stage === "downloading") {
        for (const fraction of [0.3, 0.7, 1]) {
          emit("koa:selfupdate", { stage, done: Math.round(total * fraction), total, error: "" });
          await delay(150);
        }
        continue;
      }
      emit("koa:selfupdate", { stage, done: 0, total: 0, error: "" });
      await delay(200);
    }
    selfUpdate = { ...selfUpdate, available: false, current: selfUpdate.latest };
  },
};

/** renderMarkdown is a deliberately small stand-in for the Go renderer. */
function renderMarkdown(md: string): string {
  const escape = (s: string) =>
    s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;");
  const inline = (s: string) =>
    escape(s)
      .replace(/\*\*([^*]+)\*\*/g, "<strong>$1</strong>")
      .replace(/`([^`]+)`/g, "<code>$1</code>");

  const out: string[] = [];
  const lines = md.split("\n");
  let index = 0;
  while (index < lines.length) {
    const line = lines[index];
    if (line.startsWith("```")) {
      const body: string[] = [];
      index += 1;
      while (index < lines.length && !lines[index].startsWith("```")) {
        body.push(escape(lines[index]));
        index += 1;
      }
      index += 1;
      out.push(`<pre><code>${body.join("\n")}</code></pre>`);
      continue;
    }
    const heading = /^(#{1,6})\s+(.*)$/.exec(line);
    if (heading) {
      const level = heading[1].length;
      out.push(`<h${level}>${inline(heading[2])}</h${level}>`);
      index += 1;
      continue;
    }
    if (line.startsWith("- ")) {
      const items: string[] = [];
      while (index < lines.length && lines[index].startsWith("- ")) {
        items.push(`<li>${inline(lines[index].slice(2))}</li>`);
        index += 1;
      }
      out.push(`<ul>${items.join("")}</ul>`);
      continue;
    }
    if (line.trim() === "") {
      index += 1;
      continue;
    }
    const paragraph: string[] = [];
    while (index < lines.length && lines[index].trim() !== "" && !lines[index].startsWith("#")) {
      paragraph.push(lines[index]);
      index += 1;
    }
    out.push(`<p>${inline(paragraph.join(" "))}</p>`);
  }
  return out.join("\n");
}

const mockBackend: Backend = {
  async call<T>(method: string, ...args: unknown[]): Promise<T> {
    const handler = handlers[method];
    if (!handler) throw new Error(`mock backend has no ${method}`);
    return (await handler(...args)) as T;
  },
  async window() {
    // The browser has no window chrome to drive.
  },
  on(event, handler) {
    const set = listeners.get(event) ?? new Set();
    set.add(handler);
    listeners.set(event, set);
    return () => set.delete(handler);
  },
  openURL(url) {
    window.open(url, "_blank", "noopener");
  },
};

export function installMockBackend(): void {
  installBackend(mockBackend);
}
