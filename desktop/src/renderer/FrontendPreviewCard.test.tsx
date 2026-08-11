import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it } from "vitest";
import type { ThreadItem } from "../shared/protocol";
import { FrontendPreviewCard } from "./FrontendPreviewCard";
import { buildFrontendPreviewDocument, parseFrontendPreviewSpec } from "./FrontendPreviewSpec";

let root: Root | undefined;
let container: HTMLDivElement | undefined;

function previewItem(argumentsJSON: string): ThreadItem {
  return {
    id: "preview-1",
    type: "tool_call",
    status: "completed",
    name: "render_frontend_preview",
    arguments: argumentsJSON,
    result: "Frontend preview ready.",
  };
}

function renderCard(item: ThreadItem): HTMLDivElement {
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => root?.render(<FrontendPreviewCard item={item} />));
  return container;
}

afterEach(() => {
  act(() => root?.unmount());
  container?.remove();
  root = undefined;
  container = undefined;
});

describe("FrontendPreviewCard", () => {
  const valid = JSON.stringify({
    version: 1,
    title: "Loading button",
    html: '<button id="save">Save</button>',
    css: "button:hover { transform: scale(1.05); }",
    javascript: "document.querySelector('#save').textContent = 'Ready';",
    viewport: { height: 280 },
  });

  it("stays collapsed by default and destroys the runtime when collapsed or viewing source", () => {
    const view = renderCard(previewItem(valid));
    expect(view.querySelector("iframe")).toBeNull();

    act(() => view.querySelector<HTMLButtonElement>(".frontend-preview-summary")?.click());
    const frame = view.querySelector<HTMLIFrameElement>("iframe");
    expect(frame).not.toBeNull();
    expect(frame?.getAttribute("sandbox")).toBe("allow-scripts");
    expect(frame?.getAttribute("sandbox")).not.toContain("allow-same-origin");
    expect(frame?.srcdoc).toContain("default-src 'none'");
    expect(frame?.srcdoc).toContain("connect-src 'none'");
    expect(frame?.style.height).toBe("280px");

    const sourceTab = view.querySelectorAll<HTMLButtonElement>('[role="tab"]')[1];
    act(() => sourceTab?.click());
    expect(view.querySelector("iframe")).toBeNull();
    expect(view.querySelector("pre")?.textContent).toContain("<button");

    act(() => view.querySelector<HTMLButtonElement>(".frontend-preview-summary")?.click());
    expect(view.querySelector("iframe")).toBeNull();
  });

  it("shows an inert error instead of mounting invalid history", () => {
    const view = renderCard(previewItem('{"version":1,"title":"Unsafe","html":"<script>alert(1)</script>"}'));
    act(() => view.querySelector<HTMLButtonElement>(".frontend-preview-summary")?.click());
    expect(view.querySelector("iframe")).toBeNull();
    expect(view.querySelector('[role="alert"]')).not.toBeNull();
  });
});

describe("frontend preview document", () => {
  it("rejects unknown fields and external-resource markup", () => {
    expect(parseFrontendPreviewSpec('{"version":1,"title":"x","html":"","extra":true}').error).toContain("unknown");
    expect(parseFrontendPreviewSpec('{"version":1,"title":"x","html":"<img src=\\"https://example.com/x.png\\">"}').error).toContain("attribute");
  });

  it("escapes style and script terminators while installing runtime guards", () => {
    const parsed = parseFrontendPreviewSpec(JSON.stringify({
      version: 1,
      title: "Safe",
      html: "<button>ok</button>",
      css: "/* </style> */ button { color: red; }",
      javascript: "const marker = '</script>';",
    }));
    expect(parsed.error).toBeUndefined();
    const document = buildFrontendPreviewDocument(parsed.spec!);
    expect(document).toContain("<\\/style>");
    expect(document).toContain("<\\/script>");
    expect(document).toContain('Object.defineProperty(globalThis, name');
    expect(document).toContain("form-action 'none'");
  });
});
