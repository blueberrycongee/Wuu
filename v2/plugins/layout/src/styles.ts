export const layoutStyles = `
.app-shell {
  --side-reserved-width: 686px;
  position: relative;
  display: grid;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  grid-template-columns: 326px minmax(0, 1fr) auto;
  color: var(--ink);
  background: var(--paper);
}

.app-sidebar-toggle {
  display: none;
}

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
}

.conversation-pane {
  min-width: 0;
  min-height: 0;
  overflow: hidden;
  background: var(--paper-solid);
}

.side-open-button {
  align-self: start;
  margin: 54px 10px 0;
}

@media (max-width: 760px) {
  .app-shell {
    grid-template-columns: minmax(0, 1fr) auto;
  }

  .app-sidebar {
    position: absolute;
    z-index: 4;
    inset: 0 auto 0 0;
    width: min(326px, calc(100vw - 48px));
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
    left: min(334px, calc(100vw - 40px));
  }
}
`;
