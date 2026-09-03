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
    /** Wails' raw message channel to the shell — used directly by the edge
     * resize handles, since "resize:<edge>" has no typed runtime wrapper. */
    WailsInvoke?: (message: string) => void;
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
