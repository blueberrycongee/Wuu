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
  white-space: pre-wrap;
}

.message-user {
  width: fit-content;
  max-width: min(84%, 620px);
  margin-left: auto;
  padding: 9px 13px;
  border-radius: 16px 16px 5px 16px;
  background: var(--surface-3);
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
