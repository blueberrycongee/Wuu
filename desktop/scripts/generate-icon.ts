/**
 * Regenerates the desktop app icon from the real Wuu mascot artwork.
 *
 * The mascot is blobatar SVG (see src/renderer/WuuMascot.tsx), so the icon is
 * composed from the same renderer rather than redrawn: three "wuu" blobs in
 * different hues, each wearing a different accessory from the in-app set,
 * piled up and looking right. Personality comes from the eyes, not from extra
 * decoration: the cyan hero keeps the mascot's alert default gaze, the green
 * one stares wide-eyed (surprised) and hardest right, and the peach one in
 * back rests content with closed happy arcs tilted up. Perspective yaw is
 * positive for rightward gazes (negative looks left).
 *
 * Rasterization is per size through `sips`, so small sizes are
 * resampled from vector, not downscaled bitmaps. macOS only (sips, iconutil).
 *
 * Usage: bun scripts/generate-icon.ts   (or: npm run icon:generate)
 * Outputs: build/icon.png (1024 master), build/icon.icns, build/icon.ico,
 *          build/icon.svg (editable master), intermediates and a preview
 *          contact sheet in build/.icon-work/
 */
import { execFileSync } from "node:child_process";
import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { blobatar, _layout } from "../vendor/blobatar/src/blobatar.ts";
import { palette } from "../vendor/blobatar/src/color.ts";
import { happy, surprised, type Expression } from "../vendor/blobatar/src/expression.ts";

const DESKTOP_DIR = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const BUILD_DIR = path.join(DESKTOP_DIR, "build");
const WORK_DIR = path.join(BUILD_DIR, ".icon-work");
const CANVAS = 1024;

// The mascot's own geometry: same name and pinned traits as WuuMascot, so the
// icon blob is the app blob, not an approximation of it.
const MASCOT_NAME = "wuu";
const MASCOT_TRAITS = { shape: 0.2, "body.ratio": 0.5 } as const;

type Perspective = { yaw: number; pitch: number; strength: number };
type Accessory = "sprout" | "cap" | "crown";
type BlobSpec = {
  hue: number;
  accessory: Accessory;
  /** Omit for the mascot's alert default eyes. */
  expression?: Expression;
  perspective: Perspective;
  /** Where the body center lands on the 1024 canvas. */
  center: readonly [number, number];
  /** Canvas units per viewBox unit; body radius is ~39 * scale. */
  scale: number;
};

// Pile order matters: entries render back-to-front. The cyan one carries the
// sprout (the mascot's signature accessory) and sits on top as the hero.
// The shared rightward direction makes the pile read as one group heading
// somewhere; the per-blob eye poses keep it from reading as three copies.
const BLOBS: readonly BlobSpec[] = [
  {
    hue: 52,
    accessory: "crown",
    expression: happy,
    perspective: { yaw: 16, pitch: 26, strength: 1 },
    center: [492, 395],
    scale: 5.3,
  },
  {
    hue: 150,
    accessory: "cap",
    expression: surprised,
    perspective: { yaw: 38, pitch: 6, strength: 1 },
    center: [300, 670],
    scale: 6.0,
  },
  {
    hue: 202,
    accessory: "sprout",
    perspective: { yaw: 32, pitch: 16, strength: 1 },
    center: [715, 720],
    scale: 6.0,
  },
];

// --- OKLab mixing, mirroring CSS `color-mix(in oklab, X p%, white)` from
// wuu-mascot.css so accessory fills match the in-app accessory tint.

function hexToLinearRgb(hex: string): [number, number, number] {
  const n = parseInt(hex.slice(1), 16);
  return [n >> 16, (n >> 8) & 0xff, n & 0xff].map((v) => {
    const c = v / 255;
    return c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
  }) as [number, number, number];
}

function linearRgbToHex(rgb: readonly [number, number, number]): string {
  const srgb = rgb.map((v) => {
    const c = Math.min(1, Math.max(0, v));
    const s = c <= 0.0031308 ? 12.92 * c : 1.055 * Math.pow(c, 1 / 2.4) - 0.055;
    return Math.round(s * 255);
  });
  return `#${srgb.map((v) => v.toString(16).padStart(2, "0")).join("")}`;
}

function linearRgbToOklab([r, g, b]: readonly [number, number, number]) {
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b);
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b);
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b);
  return [
    0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s,
    1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s,
    0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s,
  ] as const;
}

function oklabToLinearRgb([L, a, b]: readonly [number, number, number]) {
  const l = (L + 0.3963377774 * a + 0.2158037573 * b) ** 3;
  const m = (L - 0.1055613458 * a - 0.0638541728 * b) ** 3;
  const s = (L - 0.0894841775 * a - 1.291485548 * b) ** 3;
  return [
    4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s,
    -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s,
    -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s,
  ] as const;
}

function mixWithWhite(hex: string, ratio: number): string {
  const lab = linearRgbToOklab(hexToLinearRgb(hex));
  // White is (1, 0, 0) in OKLab.
  const mixed: [number, number, number] = [
    lab[0] * ratio + (1 - ratio),
    lab[1] * ratio,
    lab[2] * ratio,
  ];
  return linearRgbToHex(oklabToLinearRgb(mixed));
}

// --- Accessory geometry, adapted from MascotAccessory in WuuMascot.tsx and
// pinned to the same body layout so the icon and the app never drift apart.

function accessorySvg(accessory: Accessory, head: string, eye: string): string {
  const { body } = _layout(MASCOT_NAME, { traits: { ...MASCOT_TRAITS } });
  const cx = body.cx;
  const top = body.cy - body.ry;
  const fill = mixWithWhite(head, 0.48);
  const style = `fill="${fill}" stroke="${eye}" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"`;
  const line = `fill="none" stroke="${eye}" stroke-width="1.8" stroke-linecap="round"`;
  switch (accessory) {
    case "sprout":
      return (
        `<path ${line} d="M ${cx} ${top + 3} Q ${cx - 2} ${top - 6} ${cx + 1} ${top - 13}"/>` +
        `<path ${style} d="M ${cx} ${top - 9} Q ${cx - 13} ${top - 17} ${cx - 16} ${top - 7} Q ${cx - 9} ${top - 2} ${cx} ${top - 9} Z"/>` +
        `<path ${style} d="M ${cx + 1} ${top - 11} Q ${cx + 12} ${top - 16} ${cx + 17} ${top - 9} Q ${cx + 12} ${top - 3} ${cx + 1} ${top - 11} Z"/>`
      );
    case "cap":
      return (
        `<path ${style} d="M ${cx - 22} ${top + 8} Q ${cx - 18} ${top - 8} ${cx + 2} ${top - 9} Q ${cx + 20} ${top - 8} ${cx + 23} ${top + 9} Z"/>` +
        `<path ${style} d="M ${cx - 24} ${top + 8} Q ${cx + 1} ${top + 4} ${cx + 29} ${top + 10} Q ${cx + 17} ${top + 17} ${cx - 22} ${top + 13} Z"/>`
      );
    case "crown":
      return `<path ${style} d="M ${cx - 23} ${top + 11} L ${cx - 20} ${top - 7} L ${cx - 8} ${top + 1} L ${cx} ${top - 12} L ${cx + 9} ${top + 1} L ${cx + 21} ${top - 7} L ${cx + 24} ${top + 11} Q ${cx} ${top + 16} ${cx - 23} ${top + 11} Z"/>`;
  }
}

function blobSvg(spec: BlobSpec): string {
  const colors = palette(spec.hue);
  const head = colors.head ?? "#8ae2e9";
  const eye = colors.eye ?? "#051213";
  const raw = blobatar(MASCOT_NAME, {
    hue: spec.hue,
    traits: { ...MASCOT_TRAITS },
    perspective: { ...spec.perspective },
    expression: spec.expression,
    background: false,
    contrast: false,
  });
  let inner = raw.replace(/^<svg[^>]*>/, "").replace(/<\/svg>\s*$/, "");
  // blobatar emits body first (a flat <g> with no nested groups) and eyes
  // second; the accessory rides between them, exactly like the in-app portal.
  const bodyClose = inner.indexOf("</g>");
  if (bodyClose < 0) throw new Error("unexpected blobatar output shape");
  inner =
    inner.slice(0, bodyClose + "</g>".length) +
    accessorySvg(spec.accessory, head, eye) +
    inner.slice(bodyClose + "</g>".length);

  const { body } = _layout(MASCOT_NAME, { traits: { ...MASCOT_TRAITS } });
  const dx = spec.center[0] - body.cx * spec.scale;
  const dy = spec.center[1] - body.cy * spec.scale;
  return `<g transform="translate(${dx.toFixed(2)} ${dy.toFixed(2)}) scale(${spec.scale})">${inner}</g>`;
}

function composeIconSvg(): string {
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 ${CANVAS} ${CANVAS}">` +
    BLOBS.map(blobSvg).join("") +
    `</svg>`
  );
}

// --- Rasterization: sips honors the root width/height, so each size gets its
// own sized copy of the master and is rendered straight from vector.

function rasterize(
  svgBody: string,
  rootViewBox: string,
  width: number,
  height: number,
  outPath: string,
): void {
  const svgPath = `${outPath}.svg`;
  writeFileSync(
    svgPath,
    svgBody.replace(
      `viewBox="${rootViewBox}"`,
      `viewBox="${rootViewBox}" width="${width}" height="${height}"`,
    ),
  );
  execFileSync("sips", ["-s", "format", "png", svgPath, "--out", outPath], { stdio: "pipe" });
}

// --- Preview contact sheet so the result can be judged at real sizes, as
// nested SVG (sips does not render HTML, and <text> support is unreliable —
// the descending row order is the label).

function previewSvg(iconSvg: string): { svg: string; width: number; height: number } {
  const inner = iconSvg.replace(/^<svg[^>]*>/, "").replace(/<\/svg>\s*$/, "");
  const sizes = [256, 128, 64, 32, 16];
  const row = (bg: string, top: number) => {
    let x = 32;
    const baseline = top + 288;
    const cells = sizes
      .map((s) => {
        const cell = `<svg x="${x}" y="${baseline - s}" width="${s}" height="${s}" viewBox="0 0 ${CANVAS} ${CANVAS}">${inner}</svg>`;
        x += s + 20;
        return cell;
      })
      .join("");
    return `<rect x="0" y="${top}" width="640" height="320" fill="${bg}"/>${cells}`;
  };
  return {
    svg:
      `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 640 640">` +
      row("#ffffff", 0) +
      row("#2b2b2e", 320) +
      `</svg>`,
    width: 640,
    height: 640,
  };
}

// --- .ico container: PNG-compressed entries, the standard since Vista.

function buildIco(pngPaths: readonly string[], outPath: string): void {
  const images = pngPaths.map((p) => ({
    png: readFileSync(p),
    size: Number(p.match(/(\d+)\.png$/)?.[1]),
  }));
  const header = Buffer.alloc(6);
  header.writeUInt16LE(1, 2); // type: icon
  header.writeUInt16LE(images.length, 4);
  const entries = Buffer.alloc(16 * images.length);
  let offset = 6 + entries.length;
  images.forEach(({ png, size }, i) => {
    entries.writeUInt8(size >= 256 ? 0 : size, i * 16); // 0 means 256
    entries.writeUInt8(size >= 256 ? 0 : size, i * 16 + 1);
    entries.writeUInt16LE(1, i * 16 + 4); // planes
    entries.writeUInt16LE(32, i * 16 + 6); // bpp
    entries.writeUInt32LE(png.length, i * 16 + 8);
    entries.writeUInt32LE(offset, i * 16 + 12);
    offset += png.length;
  });
  writeFileSync(outPath, Buffer.concat([header, entries, ...images.map((i) => i.png)]));
}

function main(): void {
  rmSync(WORK_DIR, { recursive: true, force: true });
  mkdirSync(WORK_DIR, { recursive: true });

  const iconSvg = composeIconSvg();
  writeFileSync(path.join(BUILD_DIR, "icon.svg"), iconSvg);

  const sizes = [16, 24, 32, 48, 64, 128, 256, 512, 1024];
  for (const size of sizes) {
    rasterize(iconSvg, `0 0 ${CANVAS} ${CANVAS}`, size, size, path.join(WORK_DIR, `${size}.png`));
  }
  const preview = previewSvg(iconSvg);
  rasterize(preview.svg, "0 0 640 640", preview.width, preview.height, path.join(WORK_DIR, "preview.png"));

  const iconset = path.join(WORK_DIR, "icon.iconset");
  mkdirSync(iconset);
  const iconsetMap: ReadonlyArray<readonly [string, number]> = [
    ["icon_16x16.png", 16],
    ["icon_16x16@2x.png", 32],
    ["icon_32x32.png", 32],
    ["icon_32x32@2x.png", 64],
    ["icon_128x128.png", 128],
    ["icon_128x128@2x.png", 256],
    ["icon_256x256.png", 256],
    ["icon_256x256@2x.png", 512],
    ["icon_512x512.png", 512],
    ["icon_512x512@2x.png", 1024],
  ];
  for (const [name, size] of iconsetMap) {
    cpSync(path.join(WORK_DIR, `${size}.png`), path.join(iconset, name));
  }
  execFileSync("iconutil", ["-c", "icns", iconset, "-o", path.join(BUILD_DIR, "icon.icns")]);

  buildIco(
    [16, 24, 32, 48, 64, 128, 256].map((s) => path.join(WORK_DIR, `${s}.png`)),
    path.join(BUILD_DIR, "icon.ico"),
  );

  cpSync(path.join(WORK_DIR, "1024.png"), path.join(BUILD_DIR, "icon.png"));

  console.log(`icon assets written to ${BUILD_DIR}`);
  console.log(`preview: ${path.join(WORK_DIR, "preview.png")}`);
}

main();
