type MdastPosition = {
  start: { offset?: number };
  end: { offset?: number };
};

type MdastNode = {
  type: string;
  value?: string;
  children?: MdastNode[];
  position?: MdastPosition;
};

type MdastFile = {
  value?: unknown;
};

const CJK_CHARACTER = /[\p{Script=Han}\p{Script=Hiragana}\p{Script=Katakana}\p{Script=Hangul}]/u;
const PUNCTUATION_OR_SYMBOL = /[\p{P}\p{S}]/u;
const STRONG_MARKERS = /\*\*([^*\n]+?)\*\*/g;

function hasBrokenCjkBoundary(value: string, start: number, end: number, content: string): boolean {
  if (!CJK_CHARACTER.test(content)) {
    return false;
  }
  const before = value[start - 1] ?? "";
  const after = value[end] ?? "";
  const first = content[0] ?? "";
  const last = content.at(-1) ?? "";
  const openerIsBlocked = Boolean(before)
    && !/\s/u.test(before)
    && !PUNCTUATION_OR_SYMBOL.test(before)
    && PUNCTUATION_OR_SYMBOL.test(first);
  const closerIsBlocked = Boolean(after)
    && !/\s/u.test(after)
    && !PUNCTUATION_OR_SYMBOL.test(after)
    && PUNCTUATION_OR_SYMBOL.test(last);
  return openerIsBlocked || closerIsBlocked;
}

function repairTextNode(node: MdastNode, source: string): MdastNode[] | undefined {
  if (node.type !== "text" || typeof node.value !== "string") {
    return undefined;
  }
  const startOffset = node.position?.start.offset;
  const endOffset = node.position?.end.offset;
  if (
    typeof startOffset !== "number"
    || typeof endOffset !== "number"
    || source.slice(startOffset, endOffset) !== node.value
  ) {
    return undefined;
  }

  const replacements: MdastNode[] = [];
  let cursor = 0;
  STRONG_MARKERS.lastIndex = 0;
  for (let match = STRONG_MARKERS.exec(node.value); match; match = STRONG_MARKERS.exec(node.value)) {
    const content = match[1];
    const matchEnd = match.index + match[0].length;
    if (!hasBrokenCjkBoundary(node.value, match.index, matchEnd, content)) {
      continue;
    }
    if (match.index > cursor) {
      replacements.push({ type: "text", value: node.value.slice(cursor, match.index) });
    }
    replacements.push({ type: "strong", children: [{ type: "text", value: content }] });
    cursor = matchEnd;
  }
  if (cursor === 0) {
    return undefined;
  }
  if (cursor < node.value.length) {
    replacements.push({ type: "text", value: node.value.slice(cursor) });
  }
  return replacements;
}

function visit(parent: MdastNode, source: string): void {
  const children = parent.children;
  if (!children) {
    return;
  }
  for (let index = 0; index < children.length; index += 1) {
    const replacements = repairTextNode(children[index], source);
    if (replacements) {
      children.splice(index, 1, ...replacements);
      index += replacements.length - 1;
      continue;
    }
    visit(children[index], source);
  }
}

/** Repairs otherwise-literal `**` markers blocked by CJK punctuation boundaries. */
export function remarkCjkStrongBoundary() {
  return (tree: MdastNode, file: MdastFile): void => {
    visit(tree, typeof file?.value === "string" ? file.value : "");
  };
}

export default remarkCjkStrongBoundary;
