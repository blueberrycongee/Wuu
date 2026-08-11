import { Fragment, type ReactNode } from "react";

function safeHref(value: string): string | undefined {
  try {
    const url = new URL(value);
    return url.protocol === "https:" || url.protocol === "http:" || url.protocol === "mailto:"
      ? url.href
      : undefined;
  } catch {
    return undefined;
  }
}

function inline(text: string): ReactNode[] {
  const pattern = /(`[^`\n]+`|\*\*[^*\n]+\*\*|\*[^*\n]+\*|\[[^\]\n]+\]\([^\s)]+\))/g;
  const nodes: ReactNode[] = [];
  let cursor = 0;
  for (const match of text.matchAll(pattern)) {
    const index = match.index;
    if (index > cursor) nodes.push(text.slice(cursor, index));
    const token = match[0];
    const key = `${index}:${token}`;
    if (token.startsWith("`")) {
      nodes.push(<code key={key}>{token.slice(1, -1)}</code>);
    } else if (token.startsWith("**")) {
      nodes.push(<strong key={key}>{token.slice(2, -2)}</strong>);
    } else if (token.startsWith("*")) {
      nodes.push(<em key={key}>{token.slice(1, -1)}</em>);
    } else {
      const parts = /^\[([^\]]+)\]\(([^)]+)\)$/.exec(token);
      const href = parts ? safeHref(parts[2]!) : undefined;
      nodes.push(href
        ? <a key={key} href={href} target="_blank" rel="noreferrer">{parts![1]}</a>
        : parts?.[1] ?? token);
    }
    cursor = index + token.length;
  }
  if (cursor < text.length) nodes.push(text.slice(cursor));
  return nodes;
}

function lines(text: string, key: string): ReactNode {
  return text.split("\n").map((line, index) => (
    <Fragment key={`${key}:${index}`}>
      {index ? <br /> : null}
      {inline(line)}
    </Fragment>
  ));
}

function tableCells(row: string): string[] {
  const value = row.trim().replace(/^\|/, "").replace(/\|$/, "");
  return value.split("|").map((cell) => cell.trim());
}

export function MarkdownText({ text }: { text: string }) {
  const source = text.replace(/\r\n?/g, "\n");
  const rows = source.split("\n");
  const blocks: ReactNode[] = [];
  let index = 0;
  while (index < rows.length) {
    const row = rows[index]!;
    if (!row.trim()) {
      index += 1;
      continue;
    }
    if (row.startsWith("```")) {
      const language = row.slice(3).trim();
      const body: string[] = [];
      index += 1;
      while (index < rows.length && !rows[index]!.startsWith("```")) {
        body.push(rows[index]!);
        index += 1;
      }
      if (index < rows.length) index += 1;
      blocks.push(
        <pre key={`code:${index}`}>
          <code data-language={language || undefined}>{body.join("\n")}</code>
        </pre>,
      );
      continue;
    }
    const heading = /^(#{1,6})\s+(.+)$/.exec(row);
    if (heading) {
      const level = heading[1]!.length;
      const Tag = `h${level}` as "h1" | "h2" | "h3" | "h4" | "h5" | "h6";
      blocks.push(<Tag key={`heading:${index}`}>{inline(heading[2]!)}</Tag>);
      index += 1;
      continue;
    }
    const header = tableCells(row);
    const divider = rows[index + 1] ? tableCells(rows[index + 1]!) : [];
    if (
      row.includes("|") &&
      header.length > 1 &&
      divider.length === header.length &&
      divider.every((cell) => /^:?-{3,}:?$/.test(cell))
    ) {
      index += 2;
      const body: string[][] = [];
      while (index < rows.length && rows[index]!.includes("|") && rows[index]!.trim()) {
        body.push(tableCells(rows[index]!));
        index += 1;
      }
      blocks.push(
        <div className="message-table-wrap" key={`table:${index}`}>
          <table>
            <thead><tr>{header.map((cell, cellIndex) => (
              <th key={`header:${cellIndex}`}>{inline(cell)}</th>
            ))}</tr></thead>
            <tbody>{body.map((cells, rowIndex) => (
              <tr key={`row:${rowIndex}`}>{header.map((_, cellIndex) => (
                <td key={`cell:${cellIndex}`}>{inline(cells[cellIndex] ?? "")}</td>
              ))}</tr>
            ))}</tbody>
          </table>
        </div>,
      );
      continue;
    }
    const unordered = /^[-*]\s+(.+)$/.exec(row);
    const ordered = /^\d+[.)]\s+(.+)$/.exec(row);
    if (unordered || ordered) {
      const orderedList = Boolean(ordered);
      const items: ReactNode[] = [];
      while (index < rows.length) {
        const item = orderedList
          ? /^\d+[.)]\s+(.+)$/.exec(rows[index]!)
          : /^[-*]\s+(.+)$/.exec(rows[index]!);
        if (!item) break;
        items.push(<li key={`item:${index}`}>{inline(item[1]!)}</li>);
        index += 1;
      }
      blocks.push(orderedList
        ? <ol key={`list:${index}`}>{items}</ol>
        : <ul key={`list:${index}`}>{items}</ul>);
      continue;
    }
    if (row.startsWith("> ")) {
      const quoted: string[] = [];
      while (index < rows.length && rows[index]!.startsWith("> ")) {
        quoted.push(rows[index]!.slice(2));
        index += 1;
      }
      blocks.push(<blockquote key={`quote:${index}`}>{lines(quoted.join("\n"), "quote")}</blockquote>);
      continue;
    }
    const paragraph: string[] = [];
    while (index < rows.length && rows[index]!.trim()) {
      if (index !== 0 && rows[index]!.startsWith("```")) break;
      paragraph.push(rows[index]!);
      index += 1;
    }
    blocks.push(<p key={`paragraph:${index}`}>{lines(paragraph.join("\n"), "line")}</p>);
  }
  return <div className="message-markdown">{blocks}</div>;
}
