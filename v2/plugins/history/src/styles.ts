export const historyStyles = `
.history-sidebar {
  display: grid;
  height: 100%;
  min-height: 0;
  grid-template-rows: 52px 28px minmax(0, 1fr) auto;
  padding: 8px 10px 12px;
  color: var(--ink, #202423);
  background: linear-gradient(115deg, rgba(255,255,255,.16), transparent 58%);
}

.history-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 6px 0 10px;
}

.history-header strong {
  font-family: Georgia, serif;
  font-size: 21px;
  font-weight: 650;
  letter-spacing: .02em;
  color: var(--wuu-accent, #b64a32);
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
  cursor: pointer;
  transition: background-color 140ms ease, transform 140ms ease;
}

.history-new-button:active { transform: scale(.95); }

.history-section-heading {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 0 10px;
  color: var(--ink-muted, #6d7471);
  font-size: 11px;
  font-weight: 650;
  letter-spacing: .04em;
  text-transform: uppercase;
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
  padding: 4px 0 14px;
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
  box-shadow: inset 2px 0 0 var(--wuu-accent, #b64a32);
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
