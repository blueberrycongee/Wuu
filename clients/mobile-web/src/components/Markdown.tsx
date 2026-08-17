// Minimal, safe Markdown renderer for agent messages: fenced code blocks,
// headings, quotes, lists, hr, paragraphs, and inline code/bold/italic/links.
// Renders to React elements only — no innerHTML, no injection surface.
// Covers what agents actually emit in chat replies; anything more exotic
// (tables, images) falls back to plain paragraphs, which read fine.

import type { ReactNode } from "react";

type Block =
  | { kind: "code"; text: string }
  | { kind: "heading"; level: number; text: string }
  | { kind: "quote"; text: string }
  | { kind: "list"; ordered: boolean; items: string[] }
  | { kind: "hr" }
  | { kind: "para"; text: string };

const UL_RE = /^\s*[-*•]\s+(.*)$/;
const OL_RE = /^\s*\d+[.)]\s+(.*)$/;
const QUOTE_RE = /^\s*>\s?(.*)$/;
const FENCE_RE = /^```\w*\s*$/;

function parseBlocks(text: string): Block[] {
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  const blocks: Block[] = [];
  let para: string[] = [];
  const flushPara = (): void => {
    if (para.length > 0) {
      blocks.push({ kind: "para", text: para.join("\n") });
      para = [];
    }
  };

  let i = 0;
  while (i < lines.length) {
    const line = lines[i];

    if (FENCE_RE.test(line)) {
      flushPara();
      const buf: string[] = [];
      i += 1;
      while (i < lines.length && !FENCE_RE.test(lines[i])) {
        buf.push(lines[i]);
        i += 1;
      }
      i += 1; // closing fence or EOF
      blocks.push({ kind: "code", text: buf.join("\n") });
      continue;
    }

    if (/^\s*$/.test(line)) {
      flushPara();
      i += 1;
      continue;
    }

    const heading = line.match(/^(#{1,6})\s+(.*)$/);
    if (heading) {
      flushPara();
      blocks.push({ kind: "heading", level: Math.min(heading[1].length, 4), text: heading[2] });
      i += 1;
      continue;
    }

    if (/^\s*(-{3,}|\*{3,}|_{3,})\s*$/.test(line)) {
      flushPara();
      blocks.push({ kind: "hr" });
      i += 1;
      continue;
    }

    if (QUOTE_RE.test(line)) {
      flushPara();
      const buf: string[] = [];
      while (i < lines.length && QUOTE_RE.test(lines[i])) {
        buf.push(lines[i].match(QUOTE_RE)![1]);
        i += 1;
      }
      blocks.push({ kind: "quote", text: buf.join("\n") });
      continue;
    }

    if (UL_RE.test(line) || OL_RE.test(line)) {
      flushPara();
      const ordered = OL_RE.test(line);
      const itemRe = ordered ? OL_RE : UL_RE;
      const items: string[] = [];
      while (i < lines.length) {
        const m = lines[i].match(itemRe);
        if (!m) break;
        items.push(m[1]);
        i += 1;
      }
      blocks.push({ kind: "list", ordered, items });
      continue;
    }

    para.push(line);
    i += 1;
  }
  flushPara();
  return blocks;
}

// Inline tokens: `code`, **bold**, *italic*, [link](https://…). Links are
// only rendered for http(s)/mailto targets; anything else stays literal.
const INLINE_RE =
  /(`[^`\n]+`)|(\*\*[^*\n]+\*\*)|(\*[^*\n]+\*)|(\[[^\]\n]+\]\((?:https?:\/\/|mailto:)[^)\s]+\))/g;

function renderInline(text: string, keyPrefix: string): ReactNode[] {
  const out: ReactNode[] = [];
  let last = 0;
  let n = 0;
  for (const m of text.matchAll(INLINE_RE)) {
    const idx = m.index;
    if (idx > last) out.push(text.slice(last, idx));
    const token = m[0];
    const key = `${keyPrefix}${n++}`;
    if (token.startsWith("`")) {
      out.push(
        <code key={key} className="md-code">
          {token.slice(1, -1)}
        </code>,
      );
    } else if (token.startsWith("**")) {
      out.push(<strong key={key}>{token.slice(2, -2)}</strong>);
    } else if (token.startsWith("*")) {
      out.push(<em key={key}>{token.slice(1, -1)}</em>);
    } else {
      const link = token.match(/^\[([^\]]+)\]\(([^)]+)\)$/);
      if (link) {
        out.push(
          <a key={key} href={link[2]} target="_blank" rel="noreferrer">
            {link[1]}
          </a>,
        );
      } else {
        out.push(token);
      }
    }
    last = idx + token.length;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}

/** Soft line breaks inside a paragraph/quote render as <br/>. */
function renderMultiline(text: string, keyPrefix: string): ReactNode[] {
  return text.split("\n").flatMap((part, i) => {
    const nodes = renderInline(part, `${keyPrefix}${i}-`);
    return i === 0 ? nodes : [<br key={`${keyPrefix}${i}-br`} />, ...nodes];
  });
}

export function Markdown({ text }: { text: string }): React.JSX.Element {
  const blocks = parseBlocks(text);
  return (
    <div className="md">
      {blocks.map((block, i) => {
        const key = `b${i}`;
        switch (block.kind) {
          case "code":
            return (
              <pre key={key} className="md-pre">
                <code>{block.text}</code>
              </pre>
            );
          case "heading":
            return (
              <div key={key} className={`md-h md-h${block.level}`}>
                {renderInline(block.text, key)}
              </div>
            );
          case "quote":
            return (
              <blockquote key={key} className="md-quote">
                {renderMultiline(block.text, key)}
              </blockquote>
            );
          case "list": {
            const items = block.items.map((item, j) => (
              <li key={`${key}i${j}`}>{renderInline(item, `${key}i${j}-`)}</li>
            ));
            return block.ordered ? (
              <ol key={key} className="md-list">
                {items}
              </ol>
            ) : (
              <ul key={key} className="md-list">
                {items}
              </ul>
            );
          }
          case "hr":
            return <hr key={key} className="md-hr" />;
          case "para":
            return (
              <p key={key} className="md-p">
                {renderMultiline(block.text, key)}
              </p>
            );
        }
      })}
    </div>
  );
}
