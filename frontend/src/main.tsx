import { StrictMode } from "react";
import { createRoot } from "react-dom/client";

import { App } from "./App";
import { isDesktop } from "./lib/bridge";
import { StoreProvider } from "./lib/store";
import "./styles.css";

async function boot() {
  // Running `npm run dev` in a plain browser has no Wails bridge. A dev-only
  // mock stands in so the UI can be worked on and reviewed side by side with
  // PRD/UI/koa.dc.html without launching the desktop shell.
  if (import.meta.env.DEV && !isDesktop()) {
    const { installMockBackend } = await import("./lib/mock");
    installMockBackend();
  }

  const root = document.getElementById("root");
  if (!root) throw new Error("koa: #root is missing from index.html");

  createRoot(root).render(
    <StrictMode>
      <StoreProvider>
        <App />
      </StoreProvider>
    </StrictMode>,
  );
}

void boot();
