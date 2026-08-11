export const layoutStyles = `
.app-shell {
  display: grid;
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  grid-template-columns: 326px minmax(0, 1fr) auto;
  color: var(--ink);
  background: var(--paper);
}

.app-sidebar {
  position: relative;
  min-width: 0;
  padding: 52px 12px 12px;
  border-right: 1px solid var(--hairline);
  background: var(--surface-2);
}

.app-sidebar::before {
  content: "Wuu";
  display: block;
  padding: 8px 10px;
  color: var(--ink);
  font-size: 14px;
  font-weight: 650;
  letter-spacing: -0.01em;
}

.app-sidebar::after {
  content: "";
  position: absolute;
  z-index: 1;
  inset: 0 0 auto;
  height: 48px;
  -webkit-app-region: drag;
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
    display: none;
  }
}
`;
