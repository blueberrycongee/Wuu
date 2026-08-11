export const historyStyles = `
.history-sidebar {
  display: grid;
  height: 100%;
  min-height: 0;
  grid-template-rows: 48px minmax(0, 1fr) auto;
}

.history-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 14px 0 20px;
}

.history-header strong {
  font-size: 14px;
  font-weight: 650;
  letter-spacing: -0.01em;
}

.history-header button,
.history-list button,
.history-error button {
  border: 0;
  color: var(--ink, #202423);
  background: transparent;
  font: inherit;
}

.history-header button {
  display: grid;
  width: 30px;
  height: 30px;
  place-items: center;
  padding: 0;
  border-radius: 8px;
  font-size: 20px;
}

.history-header button:hover,
.history-header button:focus-visible,
.history-list button:hover,
.history-list button:focus-visible {
  background: rgba(31, 35, 40, 0.08);
  outline: none;
}

.history-list {
  min-height: 0;
  overflow-y: auto;
  padding: 6px 10px 14px;
}

.history-list button {
  display: flex;
  width: 100%;
  min-height: 36px;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 7px 10px;
  border-radius: 8px;
  text-align: left;
}

.history-list button.is-active {
  background: var(--surface-3, rgba(31, 35, 40, 0.08));
}

.history-list button span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.history-list button i {
  width: 6px;
  height: 6px;
  flex: none;
  border-radius: 999px;
  background: var(--wuu-accent, #202423);
}

.history-list p,
.history-error {
  margin: 8px 10px;
  color: var(--ink-muted, #6d7471);
  font-size: 12px;
}

.history-error {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 10px;
  border-top: 1px solid var(--hairline, rgba(31, 35, 40, 0.08));
}

.history-error button {
  text-decoration: underline;
}
`;
