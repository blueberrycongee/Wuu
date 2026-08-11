export const slashStyles = `
.slash-command-menu {
  position: absolute;
  z-index: 20;
  inset: auto 0 8px;
  display: grid;
  max-height: min(320px, 42vh);
  overflow-y: auto;
  padding: 6px;
  border: 1px solid var(--hairline-strong, rgba(31, 35, 40, 0.12));
  border-radius: 14px;
  color: var(--ink, #202423);
  background: var(--paper-solid, #fff);
  box-shadow: 0 16px 42px rgba(20, 24, 28, 0.14);
}

.slash-command-menu button {
  display: grid;
  min-width: 0;
  grid-template-columns: minmax(88px, auto) minmax(0, 1fr);
  align-items: center;
  gap: 12px;
  padding: 8px 10px;
  border: 0;
  border-radius: 9px;
  color: inherit;
  background: transparent;
  font: inherit;
  text-align: left;
}

.slash-command-menu button[aria-selected="true"] {
  background: var(--surface-3, rgba(31, 35, 40, 0.08));
}

.slash-command-menu button:disabled {
  opacity: 0.48;
}

.slash-command-menu button span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.slash-command-menu button span:first-child {
  font-weight: 650;
}

.slash-command-menu button span:last-child {
  color: var(--ink-muted, #6d7471);
  font-size: 12px;
}

.slash-command-menu [role="alert"] {
  padding: 8px 10px 4px;
  color: var(--danger, #b1271b);
  font-size: 12px;
}
`;
