export const layoutStyles = `
.app-shell {
  --side-reserved-width: 686px;
  position: relative;
  display: grid;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  grid-template-columns: var(--sidebar-width, 326px) minmax(0, 1fr) auto;
  transition: grid-template-columns 180ms cubic-bezier(.22,.8,.25,1);
  color: var(--ink);
  background: var(--paper);
}

.app-sidebar-toggle {
  position: absolute;
  z-index: 5;
  top: 10px;
  left: 72px;
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  padding: 0;
  border: 0;
  border-radius: 8px;
  color: var(--ink);
  background: transparent;
  cursor: pointer;
  -webkit-app-region: no-drag;
  transition: background-color 140ms ease, left 180ms ease, transform 140ms ease;
}
.app-sidebar-toggle:hover, .app-sidebar-toggle:focus-visible { background: rgba(31,35,40,.08); outline: none; }

.app-shell.is-sidebar-empty {
  --side-reserved-width: 360px;
  grid-template-columns: minmax(0, 1fr) auto;
}

@media (max-width: 1080px) {
  .app-shell {
    --side-overlay-width: calc(100vw - 48px);
  }
}

.app-shell::before {
  content: "";
  position: absolute;
  z-index: 2;
  inset: 0 0 auto;
  height: 48px;
  -webkit-app-region: drag;
}

.app-sidebar {
  position: relative;
  min-width: 0;
  padding: 48px 0 0;
  border-right: 1px solid var(--hairline);
  background: var(--surface-2);
  box-shadow: inset -1px 0 rgba(255,255,255,.18), 10px 0 28px rgba(31,35,40,.035);
  transition: opacity 180ms ease, transform 180ms ease;
}

.app-sidebar-resizer {
  position: absolute;
  z-index: 6;
  inset: 0 -5px 0 auto;
  width: 10px;
  cursor: col-resize;
  -webkit-app-region: no-drag;
}
.app-sidebar-resizer:hover::after,
.app-sidebar-resizer:focus-visible::after {
  content: "";
  position: absolute;
  inset: 0 4px;
  background: var(--hairline-strong, rgba(31,35,40,.18));
}
body.is-sidebar-resizing,
body.is-sidebar-resizing * { cursor: col-resize !important; user-select: none !important; }

.app-shell.is-sidebar-collapsed { grid-template-columns: 0 minmax(0, 1fr) auto; }
.app-shell.is-sidebar-collapsed .app-sidebar { opacity: 0; pointer-events: none; transform: translateX(-12px); }
.app-shell.is-sidebar-collapsed .app-sidebar-toggle { left: 72px; }

@media (prefers-reduced-motion: reduce) {
  .app-shell, .app-sidebar, .app-sidebar-toggle { transition: none !important; }
}

.conversation-pane {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  background: var(--paper-solid);
}

.app-work-surface {
  position: relative;
  display: block;
  min-width: 0;
  min-height: 0;
  overflow: visible;
}

@media (max-width: 760px) {
  .app-shell {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .app-sidebar {
    position: absolute;
    z-index: 4;
    inset: 0 auto 0 0;
    width: min(var(--sidebar-width, 326px), calc(100vw - 48px));
    visibility: hidden;
    transform: translateX(-100%);
    transition: transform 140ms ease, visibility 140ms step-end;
    box-shadow: 16px 0 36px rgba(31, 35, 40, 0.16);
  }

  .app-shell.is-sidebar-open .app-sidebar {
    visibility: visible;
    transform: translateX(0);
    transition: transform 140ms ease;
  }

  .app-sidebar-toggle {
    position: absolute;
    z-index: 5;
    top: 52px;
    left: 10px;
    display: grid;
    width: 32px;
    height: 32px;
    place-items: center;
    padding: 0;
    border: 1px solid var(--hairline);
    border-radius: 8px;
    color: var(--ink);
    background: var(--paper-solid);
    transition: left 140ms ease;
  }

  .app-shell.is-sidebar-open .app-sidebar-toggle {
    left: min(calc(var(--sidebar-width, 326px) + 8px), calc(100vw - 40px));
  }
}
`;
