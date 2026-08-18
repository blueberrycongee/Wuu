import { _layout, palette } from "blobatar";
import { Blobatar } from "blobatar/react";
import "blobatar/motion.css";
import {
  createContext,
  useContext,
  useLayoutEffect,
  useMemo,
  useState,
  type CSSProperties,
  type ReactNode,
  type SVGProps,
} from "react";
import { createPortal } from "react-dom";
import "./styles/wuu-mascot.css";

const WUU_MASCOT_NAME = "wuu";
const WUU_MASCOT_DEFAULT_HUE = 14;
const WUU_MASCOT_TRAITS = { shape: 0.2, "body.ratio": 0.5 } as const;
const WUU_MASCOT_LAYOUT = _layout(WUU_MASCOT_NAME, {
  traits: WUU_MASCOT_TRAITS,
});

export type WuuMascotAccessory =
  | "none"
  | "cap"
  | "beanie"
  | "top-hat"
  | "sprout"
  | "crown"
  | "headphones"
  | "scarf";

type WuuMascotRuntime = {
  provider?: string;
  model?: string;
};

const WuuMascotRuntimeContext = createContext<WuuMascotRuntime>({});

// These are fixed buckets rather than `hash % presets.length`. New presets can
// replace repeated buckets later without reshuffling every existing provider or
// model. Collisions are intentional: the wardrobe is a growing set of looks,
// not a unique logo registry.
const PROVIDER_HUE_BUCKETS = [
  14, 202, 150, 288, 52, 182, 96, 322,
  14, 202, 150, 288, 52, 182, 96, 322,
] as const;

const MODEL_ACCESSORY_BUCKETS: readonly WuuMascotAccessory[] = [
  "sprout",
  "cap",
  "beanie",
  "crown",
  "top-hat",
  "none",
  "headphones",
  "scarf",
  "beanie",
  "none",
  "sprout",
  "cap",
  "top-hat",
  "crown",
  "none",
  "headphones",
];

export function WuuMascotRuntimeProvider({
  provider,
  model,
  children,
}: WuuMascotRuntime & { children: ReactNode }): JSX.Element {
  const value = useMemo(() => ({ provider, model }), [provider, model]);
  return (
    <WuuMascotRuntimeContext.Provider value={value}>
      {children}
    </WuuMascotRuntimeContext.Provider>
  );
}

export function providerMascotHue(provider: string | undefined): number {
  const identity = provider?.trim().toLocaleLowerCase();
  if (!identity) return WUU_MASCOT_DEFAULT_HUE;
  return PROVIDER_HUE_BUCKETS[stableHash(identity) % PROVIDER_HUE_BUCKETS.length];
}

export function modelMascotAccessory(model: string | undefined): WuuMascotAccessory {
  const identity = model?.trim().toLocaleLowerCase();
  if (!identity) return "none";
  return MODEL_ACCESSORY_BUCKETS[stableHash(identity) % MODEL_ACCESSORY_BUCKETS.length];
}

type WuuMascotProps = Omit<
  SVGProps<SVGSVGElement>,
  "children" | "dangerouslySetInnerHTML" | "viewBox"
> & {
  size?: number;
  provider?: string;
  model?: string;
  accessory?: WuuMascotAccessory;
};

export function WuuMascot({
  provider,
  model,
  accessory,
  style,
  ...svgProps
}: WuuMascotProps): JSX.Element {
  const runtime = useContext(WuuMascotRuntimeContext);
  const effectiveProvider = provider ?? runtime.provider;
  const effectiveModel = model ?? runtime.model;
  const hue = providerMascotHue(effectiveProvider);
  const colors = palette(hue);
  const selectedAccessory = accessory ?? modelMascotAccessory(effectiveModel);
  const [svg, setSVG] = useState<SVGSVGElement | null>(null);
  const [bodyLayer, setBodyLayer] = useState<SVGGElement | null>(null);

  useLayoutEffect(() => {
    const nextBodyLayer =
      svg?.querySelector<SVGGElement>(".mo-bob > g:not(.mo-eyes)") ?? null;
    setBodyLayer((current) => current === nextBodyLayer ? current : nextBodyLayer);
  });

  // Keep Blobatar's own hue fixed so provider changes only update inherited
  // colour variables. The SVG subtree and its seeded animation phases survive;
  // the existing fill transitions carry the mascot into the new palette.
  const mascotStyle = {
    "--mo-head": colors.head ?? WUU_MASCOT_LAYOUT.palette.head,
    "--mo-eye": colors.eye ?? WUU_MASCOT_LAYOUT.palette.eye,
    ...style,
  } as CSSProperties;

  return (
    <>
      <Blobatar
        {...svgProps}
        ref={setSVG}
        name={WUU_MASCOT_NAME}
        hue={WUU_MASCOT_DEFAULT_HUE}
        background={false}
        traits={WUU_MASCOT_TRAITS}
        animate="always"
        style={mascotStyle}
        data-wuu-mascot-provider-hue={hue}
        data-wuu-mascot-accessory={selectedAccessory}
      />
      {/* Portal into the rendered body group rather than beside it. This keeps
          the accessory under the eye layer, inside Blobatar's exact breathe and
          bob transforms, while avoiding the library's direct-child body fill
          selector from repainting the accessory. */}
      {bodyLayer && selectedAccessory !== "none"
        ? createPortal(
            <MascotAccessory
              key={selectedAccessory}
              accessory={selectedAccessory}
            />,
            bodyLayer,
          )
        : null}
    </>
  );
}

function MascotAccessory({
  accessory,
}: {
  accessory: Exclude<WuuMascotAccessory, "none">;
}): JSX.Element {
  const { body } = WUU_MASCOT_LAYOUT;
  const top = body.cy - body.ry;
  const bottom = body.cy + body.ry;
  const centerX = body.cx;

  return (
    <g
      className={`wuu-mascot-accessory wuu-mascot-accessory-${accessory}`}
      aria-hidden="true"
    >
      {accessory === "cap" ? (
        <>
          <path className="wuu-mascot-fill" d={`M ${centerX - 22} ${top + 8} Q ${centerX - 18} ${top - 8} ${centerX + 2} ${top - 9} Q ${centerX + 20} ${top - 8} ${centerX + 23} ${top + 9} Z`} />
          <path className="wuu-mascot-fill" d={`M ${centerX - 24} ${top + 8} Q ${centerX + 1} ${top + 4} ${centerX + 29} ${top + 10} Q ${centerX + 17} ${top + 17} ${centerX - 22} ${top + 13} Z`} />
        </>
      ) : null}
      {accessory === "beanie" ? (
        <>
          <circle className="wuu-mascot-fill" cx={centerX} cy={top - 10} r="4.8" />
          <path className="wuu-mascot-fill" d={`M ${centerX - 22} ${top + 7} Q ${centerX - 19} ${top - 10} ${centerX} ${top - 11} Q ${centerX + 20} ${top - 10} ${centerX + 23} ${top + 7} Z`} />
          <rect className="wuu-mascot-fill" x={centerX - 24} y={top + 4} width="48" height="10" rx="5" />
        </>
      ) : null}
      {accessory === "top-hat" ? (
        <>
          <rect className="wuu-mascot-fill" x={centerX - 16} y={top - 14} width="32" height="27" rx="4.5" />
          <rect className="wuu-mascot-band" x={centerX - 16} y={top + 5} width="32" height="7" rx="2" />
          <rect className="wuu-mascot-fill" x={centerX - 25} y={top + 10} width="50" height="8" rx="4" />
        </>
      ) : null}
      {accessory === "sprout" ? (
        <>
          <path className="wuu-mascot-line" d={`M ${centerX} ${top + 3} Q ${centerX - 2} ${top - 6} ${centerX + 1} ${top - 13}`} />
          <path className="wuu-mascot-fill" d={`M ${centerX} ${top - 9} Q ${centerX - 13} ${top - 17} ${centerX - 16} ${top - 7} Q ${centerX - 9} ${top - 2} ${centerX} ${top - 9} Z`} />
          <path className="wuu-mascot-fill" d={`M ${centerX + 1} ${top - 11} Q ${centerX + 12} ${top - 16} ${centerX + 17} ${top - 9} Q ${centerX + 12} ${top - 3} ${centerX + 1} ${top - 11} Z`} />
        </>
      ) : null}
      {accessory === "crown" ? (
        <path className="wuu-mascot-fill" d={`M ${centerX - 23} ${top + 11} L ${centerX - 20} ${top - 7} L ${centerX - 8} ${top + 1} L ${centerX} ${top - 12} L ${centerX + 9} ${top + 1} L ${centerX + 21} ${top - 7} L ${centerX + 24} ${top + 11} Q ${centerX} ${top + 16} ${centerX - 23} ${top + 11} Z`} />
      ) : null}
      {accessory === "headphones" ? (
        <>
          <path className="wuu-mascot-line wuu-mascot-headphone-band" d={`M ${body.cx - body.rx * 0.86} ${body.cy + 5} Q ${body.cx - body.rx * 0.9} ${top - 5} ${centerX} ${top - 8} Q ${body.cx + body.rx * 0.9} ${top - 5} ${body.cx + body.rx * 0.86} ${body.cy + 5}`} />
          <rect className="wuu-mascot-fill" x={body.cx - body.rx - 3} y={body.cy - 8} width="11" height="23" rx="5.5" />
          <rect className="wuu-mascot-fill" x={body.cx + body.rx - 8} y={body.cy - 8} width="11" height="23" rx="5.5" />
        </>
      ) : null}
      {accessory === "scarf" ? (
        <>
          <path className="wuu-mascot-fill" d={`M ${body.cx - body.rx * 0.76} ${bottom - 15} Q ${centerX} ${bottom - 7} ${body.cx + body.rx * 0.76} ${bottom - 15} L ${body.cx + body.rx * 0.7} ${bottom - 5} Q ${centerX} ${bottom + 1} ${body.cx - body.rx * 0.7} ${bottom - 5} Z`} />
          <path className="wuu-mascot-fill" d={`M ${centerX + 8} ${bottom - 7} Q ${centerX + 19} ${bottom + 2} ${centerX + 14} ${bottom + 16} L ${centerX + 3} ${bottom + 8} L ${centerX + 2} ${bottom - 5} Z`} />
        </>
      ) : null}
    </g>
  );
}

function stableHash(value: string): number {
  let hash = 0x811c9dc5;
  for (let index = 0; index < value.length; index++) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 0x01000193);
  }
  return hash >>> 0;
}
