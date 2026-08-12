export const historyStyles = `
.history-sidebar {
  --history-row-gap: 4px;
  --history-heading-gap: 8px;
  display: grid;
  height: 100%;
  min-height: 0;
  flex: 1 1 0;
  grid-template-rows: minmax(0, 1fr) auto;
  padding: 0 0 12px;
  color: var(--ink, #202423);
  background: linear-gradient(115deg, rgba(255,255,255,.16), transparent 58%);
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
  width: auto;
  min-width: 30px;
  height: 30px;
  place-items: center;
  padding: 0;
  border-radius: 8px;
  font-size: 20px;
  cursor: pointer;
  transition: background-color 140ms ease, transform 140ms ease;
}

.history-new-button { grid-auto-flow: column; gap: 4px; padding: 0 7px !important; font-size: 13px !important; white-space: nowrap; }

.history-new-button:active { transform: scale(.95); }

.history-section-heading {
  display: flex;
  align-items: center;
  gap: var(--history-heading-gap);
  padding: 0 10px;
  color: var(--ink-muted, #6d7471);
  font-size: 11px;
  font-weight: 650;
  letter-spacing: .04em;
  text-transform: uppercase;
}
.history-section-heading button { display: grid; width: 24px; height: 24px; margin-left: auto; place-items: center; border: 0; border-radius: 6px; color: inherit; background: transparent; cursor: pointer; }
.history-section-heading button:hover, .history-section-heading button:focus-visible { background: rgba(31,35,40,.08); outline: none; }
.history-create-error { padding: 0 10px; color: var(--danger, #b42318); font-size: 11px; }
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
  padding: var(--history-heading-gap) 0 14px;
  display: grid;
  align-content: start;
  gap: var(--history-row-gap);
  scrollbar-width: thin;
}

.history-list button {
  display: flex;
  width: 100%;
  min-height: 32px;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  padding: 6px 10px;
  border-radius: 7px;
  text-align: left;
}

.history-list button.is-active {
  background: var(--surface-3, rgba(31, 35, 40, 0.09));
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
  animation: history-pulse 1.5s ease-in-out infinite;
}

@keyframes history-pulse { 50% { opacity: .35; transform: scale(.72); } }

@media (prefers-reduced-motion: reduce) {
  .history-header button, .history-list button, .history-list button i { transition: none; animation: none; }
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
