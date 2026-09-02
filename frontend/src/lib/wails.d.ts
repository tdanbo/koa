// The Wails runtime and Go bindings are injected into the page by the desktop
// shell. koa declares the slice it uses rather than depending on the generated
// `wailsjs` folder, so the frontend type-checks and builds on its own.

export {};

declare global {
  interface Window {
    go?: {
      main?: {
        Koa?: Record<string, (...args: unknown[]) => Promise<unknown>>;
        Window?: Record<string, (...args: unknown[]) => Promise<unknown>>;
      };
    };
    runtime?: {
      EventsOn(event: string, callback: (...data: unknown[]) => void): () => void;
      EventsOff(event: string): void;
      WindowMinimise(): void;
      WindowToggleMaximise(): void;
      WindowIsMaximised(): Promise<boolean>;
      BrowserOpenURL(url: string): void;
      Quit(): void;
    };
  }
}
