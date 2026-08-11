export const sideStyles = `
.side-panel {
  position: relative;
  display: grid;
  height: 100%;
  min-width: 0;
  min-height: 0;
  grid-template-rows: 48px minmax(0, 1fr) auto;
  border-left: 1px solid var(--hairline-strong, rgba(31, 35, 40, 0.12));
  color: var(--ink, #202423);
  background: var(--paper, #fff);
}

.side-resizer {
  position: absolute;
  inset: 0 auto 0 -5px;
  z-index: 3;
  width: 10px;
  padding: 0;
  border: 0;
  background: transparent;
  cursor: col-resize;
}

.side-resizer:focus-visible::after {
  content: "";
  position: absolute;
  inset: 0 auto 0 4px;
  width: 2px;
  background: var(--ink-muted, #6d7471);
}

.side-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 12px;
  border-bottom: 1px solid var(--hairline, rgba(31, 35, 40, 0.08));
  font-size: 13px;
}

.side-header button,
.side-open-button {
  min-height: 28px;
  padding: 0 10px;
  border: 0;
  border-radius: 7px;
  color: var(--ink-muted, #6d7471);
  background: transparent;
}

.side-header button:hover,
.side-open-button:hover {
  color: var(--ink, #202423);
  background: rgba(31, 35, 40, 0.08);
}

.side-conversation {
  min-height: 0;
  overflow-y: auto;
  padding: 12px;
  overscroll-behavior: contain;
}

.side-message {
  margin: 0 0 12px;
  white-space: pre-wrap;
}

.side-composer-host {
  min-width: 0;
  padding: 12px;
  background: var(--paper, #fff);
}
`;
