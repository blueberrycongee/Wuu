#!/usr/bin/env node
/**
 * Generates the theme surface matrix (config/desktop-theme-surface-matrix.json)
 * and the theme-author docs (docs/en/customize/theme-surface-matrix.md and
 * docs/zh-cn/customize/theme-surface-matrix.md) from the CSS dependency graph.
 * Run via vite-node from the desktop package:
 *
 *   cd desktop && npx vite-node ../scripts/generate-theme-surface-matrix.ts
 *
 * `--check` verifies the committed artifacts are current and exits non-zero
 * when stale (wired to `make theme-surface-matrix-check`).
 */

import { readFileSync, readdirSync, writeFileSync } from "node:fs";
import { relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

import {
  analyzeSurfaceMatrix,
  listProductionSources,
  parsePinnedAnchors,
  type SurfaceMatrix,
} from "../desktop/src/renderer/styles/themeCoverage";

const repoRoot = resolve(fileURLToPath(new URL("..", import.meta.url)));
const checkOnly = process.argv.includes("--check");

const stylesDir = resolve(repoRoot, "desktop/src/renderer/styles");
const rendererDir = resolve(repoRoot, "desktop/src/renderer");
const contractPath = resolve(repoRoot, "config/desktop-theme-contract.json");

const cssFiles = readdirSync(stylesDir)
  .filter((name) => name.endsWith(".css"))
  .sort()
  .map((name) => ({ name, source: readFileSync(resolve(stylesDir, name), "utf8") }));
const anchorSources = listProductionSources(rendererDir).map((path) => readFileSync(path, "utf8"));
const pinnedTest = readFileSync(
  resolve(rendererDir, "plugins/ProductionSemanticAnchors.test.ts"),
  "utf8",
);
const contract = JSON.parse(readFileSync(contractPath, "utf8"));

const matrix = analyzeSurfaceMatrix(cssFiles, anchorSources, parsePinnedAnchors(pinnedTest));
const outputs = new Map([
  [resolve(repoRoot, "config/desktop-theme-surface-matrix.json"), renderMatrixJson(matrix)],
  [resolve(repoRoot, "docs/en/customize/theme-surface-matrix.md"), renderDoc(matrix, contract, "en")],
  [resolve(repoRoot, "docs/zh-cn/customize/theme-surface-matrix.md"), renderDoc(matrix, contract, "zh")],
]);

let stale = false;
for (const [path, content] of outputs) {
  if (readOptional(path) === content) continue;
  stale = true;
  if (checkOnly) {
    console.error(`${relative(repoRoot, path)} is stale; run make generate-theme-surface-matrix`);
  } else {
    writeFileSync(path, content);
    console.log(`generated ${relative(repoRoot, path)}`);
  }
}

if (checkOnly && stale) process.exit(1);
if (checkOnly) console.log("theme surface matrix is current");

function readOptional(path) {
  try {
    return readFileSync(path, "utf8");
  } catch (error) {
    if (error?.code === "ENOENT") return undefined;
    throw error;
  }
}

/** Compact row-per-line JSON so the committed artifact stays small. */
function renderMatrixJson(matrix: SurfaceMatrix): string {
  const header = {
    schemaVersion: matrix.schemaVersion,
    generatedBy: matrix.generatedBy,
    anchors: matrix.anchors,
    tokenSet: matrix.tokenSet,
  };
  const rows = matrix.rows.map((row) => JSON.stringify(row));
  return `${JSON.stringify(header, null, 2).slice(0, -1)},\n  "rows": [\n${rows
    .map((row) => `    ${row}`)
    .join(",\n")}\n  ]\n}\n`;
}

/** Reverse view: public token -> the (anchor, state, paint property) cells it reaches. */
function tokenSurfaces(matrix: SurfaceMatrix): Map<string, string[]> {
  const cellsByToken = new Map<string, Set<string>>();
  for (const row of matrix.rows) {
    for (const token of row.tokens) {
      const cells = cellsByToken.get(token) ?? new Set<string>();
      cells.add(`${row.anchor}|${row.state}|${row.prop}`);
      cellsByToken.set(token, cells);
    }
  }
  const sorted = new Map<string, string[]>();
  for (const [token, cells] of cellsByToken) {
    sorted.set(token, [...cells].sort());
  }
  return sorted;
}

function renderDoc(matrix: SurfaceMatrix, contract, locale: "en" | "zh"): string {
  const bridged = matrix.rows.filter((row) => row.status === "bridged").length;
  const unbridged = matrix.rows.length - bridged;
  const realAnchors = matrix.anchors.filter((anchor) => !anchor.synthetic);
  const anchoredRows = matrix.rows.filter((row) => !row.anchor.startsWith("unanchored:")).length;
  const surfaces = tokenSurfaces(matrix);

  const anchorRows = matrix.anchors.map(
    (anchor) =>
      `| \`${anchor.name}\`${anchor.synthetic ? (locale === "zh" ? "（合成）" : " (synthetic)") : ""} | ${anchor.rows} |`,
  );

  const tokenNames = [...new Set([...contract.tokens.map((t) => t.name), ...contract.syntax])].sort();
  const tokenRows = tokenNames.map((token) => {
    const cells = surfaces.get(token) ?? [];
    const rendered =
      cells.length > 0
        ? cells
            .map((cell) => {
              const [anchor, state, prop] = cell.split("|");
              const count = matrix.rows.filter(
                (row) =>
                  row.anchor === anchor && row.state === state && row.prop === prop && row.tokens.includes(token),
              ).length;
              return `\`${anchor.replace("unanchored:", "")}\` · ${state} · \`${prop}\`${count > 1 ? ` (${count})` : ""}`;
            })
            .join("<br>")
        : locale === "zh"
          ? "—（未到达宿主表面）"
          : "— (no host surface)";
    return `| \`${token}\` | ${rendered} |`;
  });

  const unbridgedByFile = matrix.rows
    .filter((row) => row.status === "unbridged")
    .reduce((acc, row) => {
      acc.set(row.file, (acc.get(row.file) ?? 0) + 1);
      return acc;
    }, new Map<string, number>());
  const unbridgedSummary = [...unbridgedByFile.entries()]
    .sort((a, b) => b[1] - a[1])
    .map(([file, count]) => `${file} ${count}`)
    .join(", ");

  if (locale === "zh") {
    return `# 主题表面矩阵

本页由 \`scripts/generate-theme-surface-matrix.ts\` 从宿主样式依赖图生成（运行
\`make generate-theme-surface-matrix\`）；请勿手改。机器可读的矩阵位于
[\`config/desktop-theme-surface-matrix.json\`](../../../config/desktop-theme-surface-matrix.json)，
U1 覆盖基线位于
[\`themeCoverage.baseline.txt\`](../../../desktop/src/renderer/styles/themeCoverage.baseline.txt)。

矩阵的每一行是宿主样式表中一条颜色类绘制声明的 \`var()\` 引用。当公开 token 能沿自定义属性
依赖图到达该变量时，这一行视为**已桥接（bridged）**，否则为**未桥接（unbridged）**。
纯几何声明不在此范围。

当前总计：**${matrix.rows.length} 行**（${bridged} 已桥接、${unbridged}
未桥接），**${matrix.tokenSet.length} 个公开 token** 可达宿主表面，**${realAnchors.length} 个锚点**已发布。

## 锚点覆盖

宿主样式目前选择组件类名：**${realAnchors.length} 个锚点**中没有任何一个被宿主 CSS 选择器引用，
**${matrix.rows.length} 行中的 ${anchoredRows} 行**归属于 \`data-wuu-component\` 锚点。
其余行归入按文件划分的 \`unanchored\` 桶，直到宿主把选择器迁移到锚点。

| 锚点 | 行数 |
| --- | --- |
${anchorRows.join("\n")}

## token 可达的表面

表面以 \`文件 · 状态 · 属性\` 单元格展示（一个单元格多于一条声明时标注数量）。\`—\` 表示该
token 已在合同中声明，但未到达任何宿主绘制声明。

| token | 表面 |
| --- | --- |
${tokenRows.join("\n")}

## 未桥接的表面

未桥接行即 U1 baseline 条目：按文件归属 unanchored，优先级顺序
workspace → turns → sidebar → settings → composer/conversation-shell →
channels → environment → image-preview → 其余。当前分布：
${unbridgedSummary}。

唯一允许的桥接写法是单条声明：

\`\`\`css
prop: var(--wuu-slot, var(--private-fallback));
\`\`\`
`;
  }

  return `# Theme surface matrix

This page is generated from the host stylesheet dependency graph by
\`scripts/generate-theme-surface-matrix.ts\` (run
\`make generate-theme-surface-matrix\`); do not edit it by hand. The
machine-readable matrix lives at
[\`config/desktop-theme-surface-matrix.json\`](../../../config/desktop-theme-surface-matrix.json)
and the U1 coverage baseline at
[\`themeCoverage.baseline.txt\`](../../../desktop/src/renderer/styles/themeCoverage.baseline.txt).

Every row is one \`var()\` reference of a color-kind paint declaration in the
host stylesheets. A row is **bridged** when a public token reaches the
variable through the custom-property dependency graph, and **unbridged**
otherwise. Geometry-only declarations are excluded.

Current totals: **${matrix.rows.length} rows** (${bridged} bridged,
${unbridged} unbridged), **${matrix.tokenSet.length} public tokens** reach a
host surface, **${realAnchors.length} anchors** published.

## Anchor coverage

Host stylesheets currently target component class names: none of the
**${realAnchors.length} anchors** has a host CSS selector referencing it, and
**${anchoredRows} of ${matrix.rows.length} rows** are attributed to a
\`data-wuu-component\` anchor. The remaining rows fall into the per-file
\`unanchored\` bucket until the host migrates selectors to anchors.

| anchor | rows |
| --- | --- |
${anchorRows.join("\n")}

## Tokens and the surfaces they reach

Surfaces are shown as \`file · state · property\` cells (with the number of
declarations when a cell has more than one). \`—\` means the token is declared
in the contract but reaches no host paint declaration.

| token | surfaces |
| --- | --- |
${tokenRows.join("\n")}

## Unbridged surfaces

Unbridged rows are the U1 baseline entries: unanchored per file, priority
order workspace → turns → sidebar → settings → composer/conversation-shell →
channels → environment → image-preview → rest. Current distribution:
${unbridgedSummary}.

The only sanctioned bridge form is a single declaration:

\`\`\`css
prop: var(--wuu-slot, var(--private-fallback));
\`\`\`
`;
}
