import type { ThreadItem } from "../shared/protocol";

export const FRONTEND_PREVIEW_TOOL_NAME = "render_frontend_preview";

const FRONTEND_PREVIEW_VERSION = 1;
const MAX_PAYLOAD_BYTES = 256 * 1024;
const MAX_TITLE_BYTES = 80;
const MAX_HTML_BYTES = 96 * 1024;
const MAX_CSS_BYTES = 96 * 1024;
const MAX_JAVASCRIPT_BYTES = 96 * 1024;
const MAX_HTML_NODES = 500;
const DEFAULT_HEIGHT = 320;
const MIN_HEIGHT = 160;
const MAX_HEIGHT = 720;

export type FrontendPreviewSpec = {
  version: 1;
  title: string;
  html: string;
  css: string;
  javascript: string;
  viewport: {
    height: number;
  };
};

export type FrontendPreviewParseResult =
  | { spec: FrontendPreviewSpec; error?: never }
  | { spec?: never; error: string };

export function isSuccessfulFrontendPreviewToolCall(item: ThreadItem): boolean {
  return (
    item.type === "tool_call" &&
    item.name === FRONTEND_PREVIEW_TOOL_NAME &&
    item.status === "completed" &&
    item.result_detail?.is_error !== true &&
    !item.error
  );
}

export function parseFrontendPreviewSpec(
  raw: string | undefined,
): FrontendPreviewParseResult {
  if (!raw || utf8ByteLength(raw) > MAX_PAYLOAD_BYTES) {
    return { error: "Preview payload is missing or too large." };
  }
  let value: unknown;
  try {
    value = JSON.parse(raw);
  } catch {
    return { error: "Preview payload is not valid JSON." };
  }
  if (!isRecord(value) || !hasOnlyKeys(value, ["version", "title", "html", "css", "javascript", "viewport"])) {
    return { error: "Preview payload contains unknown fields." };
  }
  if (value.version !== FRONTEND_PREVIEW_VERSION) {
    return { error: "This preview version is not supported." };
  }
  if (typeof value.title !== "string" || value.title.trim().length === 0 || utf8ByteLength(value.title.trim()) > MAX_TITLE_BYTES) {
    return { error: "Preview title is missing or too large." };
  }
  if (typeof value.html !== "string" || utf8ByteLength(value.html) > MAX_HTML_BYTES) {
    return { error: "Preview HTML is missing or too large." };
  }
  const css = value.css === undefined ? "" : value.css;
  const javascript = value.javascript === undefined ? "" : value.javascript;
  if (typeof css !== "string" || utf8ByteLength(css) > MAX_CSS_BYTES) {
    return { error: "Preview CSS is invalid or too large." };
  }
  if (typeof javascript !== "string" || utf8ByteLength(javascript) > MAX_JAVASCRIPT_BYTES) {
    return { error: "Preview JavaScript is invalid or too large." };
  }
  let height = DEFAULT_HEIGHT;
  if (value.viewport !== undefined) {
    if (!isRecord(value.viewport) || !hasOnlyKeys(value.viewport, ["height"])) {
      return { error: "Preview viewport contains unknown fields." };
    }
    if (value.viewport.height !== undefined) {
      if (!Number.isInteger(value.viewport.height)) {
        return { error: "Preview height must be an integer." };
      }
      height = value.viewport.height as number;
    }
  }
  if (height < MIN_HEIGHT || height > MAX_HEIGHT) {
    return { error: `Preview height must be between ${MIN_HEIGHT} and ${MAX_HEIGHT}.` };
  }
  const htmlError = validateHTMLFragment(value.html);
  if (htmlError) return { error: htmlError };
  const cssError = validateCSS(css);
  if (cssError) return { error: cssError };

  return {
    spec: {
      version: 1,
      title: value.title.trim(),
      html: value.html,
      css,
      javascript,
      viewport: { height },
    },
  };
}

const FORBIDDEN_ELEMENTS = new Set([
  "APPLET", "BASE", "BODY", "EMBED", "FRAME", "FRAMESET", "HEAD", "HTML",
  "IFRAME", "LINK", "META", "OBJECT", "SCRIPT", "STYLE", "TITLE", "WEBVIEW",
]);

const FORBIDDEN_ATTRIBUTES = new Set([
  "action", "background", "cite", "data", "formaction", "href", "longdesc",
  "manifest", "ping", "poster", "profile", "src", "srcdoc", "srcset", "usemap",
]);

function validateHTMLFragment(fragment: string): string | undefined {
  if (/<\/?(?:html|head|body|script|style|iframe|link|meta|base|object|embed|webview)\b/i.test(fragment)) {
    return "Preview HTML contains a forbidden element.";
  }
  if (typeof DOMParser === "undefined") {
    return "Preview HTML validation is unavailable.";
  }
  const document = new DOMParser().parseFromString(`<main>${fragment}</main>`, "text/html");
  const root = document.body.firstElementChild;
  if (!root) {
    return "Preview HTML validation failed.";
  }
  const walker = document.createTreeWalker(root, NodeFilter.SHOW_ALL);
  let nodeCount = 0;
  while (walker.nextNode()) nodeCount++;
  if (nodeCount > MAX_HTML_NODES) {
    return `Preview HTML exceeds ${MAX_HTML_NODES} nodes.`;
  }
  const elements = Array.from(root.querySelectorAll("*"));
  for (const element of elements) {
    if (FORBIDDEN_ELEMENTS.has(element.tagName)) {
      return `Preview HTML element <${element.tagName.toLowerCase()}> is not allowed.`;
    }
    for (const attribute of Array.from(element.attributes)) {
      const name = attribute.name.toLowerCase();
      if (name.startsWith("on") || FORBIDDEN_ATTRIBUTES.has(name)) {
        return `Preview HTML attribute ${name} is not allowed.`;
      }
    }
  }
  return undefined;
}

function validateCSS(css: string): string | undefined {
  const normalized = css.toLowerCase();
  for (const token of ["@import", "url(", "expression(", "-moz-binding"]) {
    if (normalized.includes(token)) {
      return `Preview CSS contains forbidden ${token}.`;
    }
  }
  return undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function hasOnlyKeys(value: Record<string, unknown>, allowed: readonly string[]): boolean {
  const keys = new Set(allowed);
  return Object.keys(value).every((key) => keys.has(key));
}

function utf8ByteLength(value: string): number {
  return new TextEncoder().encode(value).byteLength;
}

function escapeStyleEnd(value: string): string {
  return value.replace(/<\/style/gi, "<\\/style");
}

function escapeScriptEnd(value: string): string {
  return value.replace(/<\/script/gi, "<\\/script");
}

const PREVIEW_CSP = [
  "default-src 'none'",
  "script-src 'unsafe-inline'",
  "style-src 'unsafe-inline'",
  "img-src data:",
  "connect-src 'none'",
  "font-src 'none'",
  "media-src 'none'",
  "object-src 'none'",
  "frame-src 'none'",
  "worker-src 'none'",
  "form-action 'none'",
  "base-uri 'none'",
  "navigate-to 'none'",
].join("; ");

const PREVIEW_RUNTIME_GUARD = `
(() => {
  const blocked = (name) => () => { throw new Error(name + " is disabled in Wuu previews"); };
  for (const name of ["fetch", "XMLHttpRequest", "WebSocket", "EventSource", "Worker", "SharedWorker", "open"]) {
    try {
      Object.defineProperty(globalThis, name, { value: blocked(name), writable: false, configurable: false });
    } catch {}
  }
  try {
    Object.defineProperty(navigator, "sendBeacon", { value: blocked("sendBeacon"), writable: false, configurable: false });
  } catch {}
  document.addEventListener("submit", (event) => event.preventDefault(), true);
  document.addEventListener("click", (event) => {
    if (event.target instanceof Element && event.target.closest("a")) event.preventDefault();
  }, true);
})();`;

export function buildFrontendPreviewDocument(spec: FrontendPreviewSpec): string {
  return `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<meta http-equiv="Content-Security-Policy" content="${PREVIEW_CSP}">
<meta name="viewport" content="width=device-width, initial-scale=1">
<style>
:root { color-scheme: light dark; font-family: Inter, ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
* { box-sizing: border-box; }
html, body { min-height: 100%; margin: 0; }
body { padding: 20px; background: #ffffff; color: #202124; }
button, input, select, textarea { font: inherit; }
@media (prefers-color-scheme: dark) { body { background: #171918; color: #f1f3f2; } }
@media (prefers-reduced-motion: reduce) { *, *::before, *::after { animation-duration: 0.001ms !important; animation-iteration-count: 1 !important; transition-duration: 0.001ms !important; } }
</style>
<style>${escapeStyleEnd(spec.css)}</style>
</head>
<body>
${spec.html}
<script>${PREVIEW_RUNTIME_GUARD}</script>
<script>${escapeScriptEnd(spec.javascript)}</script>
</body>
</html>`;
}
