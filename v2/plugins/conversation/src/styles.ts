export const conversationStyles = `
.conversation-shell {
  display: grid;
  width: min(100%, 776px);
  height: 100%;
  min-height: 0;
  margin: 0 auto;
  grid-template-rows: minmax(0, 1fr) auto;
}

.conversation-scroll {
  min-height: 0;
  overflow-y: auto;
  padding: 72px 48px 32px;
  overscroll-behavior: contain;
}

.message {
  margin: 0 0 22px;
  color: var(--ink);
  font-size: 15px;
  line-height: 1.65;
  overflow-wrap: anywhere;
}

.message-markdown > :first-child { margin-top: 0; }
.message-markdown > :last-child { margin-bottom: 0; }

.message-markdown p,
.message-markdown h1,
.message-markdown h2,
.message-markdown h3,
.message-markdown h4,
.message-markdown h5,
.message-markdown h6,
.message-markdown ul,
.message-markdown ol,
.message-markdown blockquote,
.message-markdown pre {
  margin: 0 0 12px;
}

.message-markdown h1 { font-size: 1.35em; }
.message-markdown h2 { font-size: 1.24em; }
.message-markdown h3 { font-size: 1.14em; }
.message-markdown h4,
.message-markdown h5,
.message-markdown h6 { font-size: 1em; }

.message-markdown ul,
.message-markdown ol {
  padding-left: 24px;
}

.message-markdown blockquote {
  padding-left: 12px;
  border-left: 2px solid var(--hairline-strong);
  color: var(--ink-muted);
}

.message-markdown code {
  border-radius: 5px;
  background: var(--surface-3);
  font: 0.9em/1.45 ui-monospace, SFMono-Regular, Menlo, monospace;
}

.message-markdown :not(pre) > code {
  padding: 0.12em 0.35em;
}

.message-markdown pre {
  overflow-x: auto;
  padding: 12px;
  border: 1px solid var(--hairline);
  border-radius: 10px;
  background: var(--surface-2);
}

.message-markdown pre code {
  background: transparent;
}

.message-markdown a {
  color: inherit;
  text-decoration-thickness: 1px;
  text-underline-offset: 3px;
}

.message-table-wrap {
  max-width: 100%;
  margin: 0 0 12px;
  overflow-x: auto;
}

.message-markdown table {
  width: 100%;
  border-spacing: 0;
  border-collapse: collapse;
  font-size: 0.92em;
}

.message-markdown th,
.message-markdown td {
  padding: 6px 9px;
  border: 1px solid var(--hairline-strong);
  text-align: left;
  vertical-align: top;
}

.message-terminal,
.conversation-status {
  color: var(--danger);
  font-size: 12px;
}

.message-terminal {
  display: block;
  margin-top: 6px;
}

.conversation-status {
  margin: 0 0 18px;
  padding: 8px 10px;
  border: 1px solid color-mix(in srgb, var(--danger) 24%, transparent);
  border-radius: 8px;
  background: color-mix(in srgb, var(--danger) 5%, transparent);
}

.message-user {
  width: fit-content;
  max-width: min(84%, 620px);
  margin-left: auto;
  padding: 9px 13px;
  border-radius: 16px 16px 5px 16px;
  background: var(--surface-3);
}

.tool-activity {
  margin: 0 0 16px;
  overflow: hidden;
  border: 1px solid var(--hairline, rgba(31, 35, 40, 0.08));
  border-radius: 10px;
  color: var(--ink-muted, #5f6663);
  background: var(--surface-2, #f7f7f5);
  font-size: 12px;
}

.tool-activity[data-status="error"],
.tool-activity[data-status="failed"] {
  border-color: color-mix(in srgb, var(--danger, #b1271b) 35%, transparent);
}

.tool-activity-heading {
  display: flex;
  min-height: 34px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 0 11px;
}

.tool-activity-heading code {
  color: var(--ink, #202423);
  font: inherit;
  font-weight: 600;
}

.tool-activity details,
.tool-activity-result {
  margin: 0;
  border-top: 1px solid var(--hairline, rgba(31, 35, 40, 0.08));
}

.tool-activity summary {
  padding: 8px 11px;
  cursor: pointer;
}

.tool-activity pre {
  max-height: 180px;
  margin: 0;
  overflow: auto;
  padding: 10px 11px;
  color: var(--ink, #202423);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.message-assistant[data-status="streaming"]::after {
  content: "";
  display: inline-block;
  width: 5px;
  height: 1em;
  margin-left: 3px;
  vertical-align: -0.12em;
  border-radius: 2px;
  background: var(--ink-muted);
  animation: wuu-caret 900ms steps(1, end) infinite;
}

.conversation-shell > .wuu-composer-stack {
  box-sizing: border-box;
  padding: 0 48px 16px;
}

.conversation-empty {
  display: grid;
  height: 100%;
  place-items: center;
  color: var(--ink-muted);
}

.conversation-empty-state {
  margin: 18vh 0 0;
  color: var(--ink-muted);
  text-align: center;
}

@keyframes wuu-caret {
  50% { opacity: 0; }
}

@media (max-width: 760px) {
  .conversation-scroll {
    padding-right: 20px;
    padding-left: 20px;
  }

  .conversation-shell > .wuu-composer-stack {
    padding-right: 16px;
    padding-left: 16px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .message-assistant[data-status="streaming"]::after {
    animation: none;
  }
}
`;
