import { _layout, palette } from "blobatar";
import { happy, smug, surprised, type Expression } from "blobatar/expression";
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
import { AVATAR_HUES } from "./DefaultAvatar";
import "./styles/wuu-mascot.css";

import {
  WUU_MASCOT_ACTIVITY_PERSPECTIVES,
  WUU_MASCOT_DEFAULT_HUE,
  WUU_MASCOT_NAME,
  WUU_MASCOT_TRAITS,
} from "./wuu-mascot-spec";

const WUU_MASCOT_LAYOUT = _layout(WUU_MASCOT_NAME, {
  traits: WUU_MASCOT_TRAITS,
});

function withLongMascotEyes(expression: Expression): Expression {
  return {
    ...expression,
    p: {
      ...expression.p,
      // Expressions may move the eyes and body, but the mascot should keep its
      // long portrait eyes instead of morphing them into horizontal squints.
      esx: 1,
      esy: 1,
      esx2: 0,
      esy2: 0,
      tilt: 0,
      tilt2: 0,
    },
  };
}

export type WuuMascotAccessory =
  | "none"
  | "cap"
  | "beanie"
  | "top-hat"
  | "sprout"
  | "crown"
  | "headphones"
  | "scarf"
  | "beret"
  | "party-hat"
  | "wizard-hat"
  | "chef-hat"
  | "flower"
  | "halo"
  | "bow-tie"
  | "graduation-cap"
  | "cowboy-hat"
  | "propeller-cap"
  | "mushroom-cap"
  | "bunny-ears"
  | "cat-ears"
  | "ribbon"
  | "necktie";

export type { WuuMascotActivity } from "./wuu-mascot-spec";
import type { WuuMascotActivity } from "./wuu-mascot-spec";

const WUU_MASCOT_ACTIVITY_EXPRESSIONS: Readonly<
  Record<WuuMascotActivity, Expression | undefined>
> = {
  idle: undefined,
  // Activity still changes gaze, spacing, and body position, while this shared
  // wrapper keeps the eye silhouette consistently long everywhere Wuu appears.
  compose: withLongMascotEyes(happy),
  thinking: withLongMascotEyes(smug),
  compact: withLongMascotEyes(smug),
  search: withLongMascotEyes(surprised),
  edit: withLongMascotEyes(happy),
  command: withLongMascotEyes(happy),
  read: withLongMascotEyes(smug),
  tool: withLongMascotEyes(happy),
};

type WuuMascotRuntime = {
  provider?: string;
  providers?: readonly string[];
  model?: string;
};

const WuuMascotRuntimeContext = createContext<WuuMascotRuntime>({});

// Spread early assignments across the colour wheel, then use the remaining
// shared avatar hues before any provider colour is reused.
const PROVIDER_HUES = [
  14, 202, 96, 288, 52, 222, 150, 322, 33, 250, 182, 350,
] as const satisfies readonly (typeof AVATAR_HUES[number])[];

const MODEL_ACCESSORY_BUCKETS: readonly WuuMascotAccessory[] = [
  "sprout",
  "cap",
  "beanie",
  "crown",
  "top-hat",
  "none",
  "headphones",
  "scarf",
  "beret",
  "party-hat",
  "flower",
  "halo",
  "bow-tie",
  "wizard-hat",
  "chef-hat",
  "headphones",
  "graduation-cap",
  "cowboy-hat",
  "bunny-ears",
  "cat-ears",
  "propeller-cap",
  "mushroom-cap",
  "ribbon",
  "necktie",
  "beret",
  "party-hat",
  "flower",
  "halo",
  "bow-tie",
  "wizard-hat",
  "chef-hat",
  "headphones",
];

export function WuuMascotRuntimeProvider({
  provider,
  providers,
  model,
  children,
}: WuuMascotRuntime & { children: ReactNode }): JSX.Element {
  const value = useMemo(() => ({ provider, providers, model }), [provider, providers, model]);
  return (
    <WuuMascotRuntimeContext.Provider value={value}>
      {children}
    </WuuMascotRuntimeContext.Provider>
  );
}

function normalizedProviderIdentity(provider: string | undefined): string {
  return provider?.trim().toLocaleLowerCase() ?? "";
}

export function providerMascotHue(
  provider: string | undefined,
  providers?: readonly string[],
): number {
  const identity = normalizedProviderIdentity(provider);
  if (!identity) return WUU_MASCOT_DEFAULT_HUE;

  if (providers) {
    const identities = [...new Set(providers.map(normalizedProviderIdentity).filter(Boolean))];
    const index = identities.indexOf(identity);
    const allocationIndex = index >= 0 ? index : identities.length;
    return PROVIDER_HUES[allocationIndex % PROVIDER_HUES.length];
  }

  return PROVIDER_HUES[stableHash(identity) % PROVIDER_HUES.length];
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
  activity?: WuuMascotActivity;
};

export function WuuMascot({
  provider,
  model,
  accessory,
  activity = "idle",
  style,
  ...svgProps
}: WuuMascotProps): JSX.Element {
  const runtime = useContext(WuuMascotRuntimeContext);
  const effectiveProvider = provider ?? runtime.provider;
  const effectiveModel = model ?? runtime.model;
  const hue = providerMascotHue(effectiveProvider, runtime.providers);
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
        perspective={WUU_MASCOT_ACTIVITY_PERSPECTIVES[activity]}
        animate="always"
        expression={WUU_MASCOT_ACTIVITY_EXPRESSIONS[activity]}
        focusable={false}
        pointerEvents="none"
        style={mascotStyle}
        data-wuu-mascot-provider-hue={hue}
        data-wuu-mascot-accessory={selectedAccessory}
        data-wuu-mascot-activity={activity}
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
      {bodyLayer && activity !== "idle" && activity !== "compact"
        ? createPortal(
            <MascotActivityProp key={activity} activity={activity} />,
            bodyLayer,
          )
        : null}
    </>
  );
}

function MascotActivityProp({
  activity,
}: {
  activity: Exclude<WuuMascotActivity, "idle">;
}): JSX.Element {
  return (
    <g
      className={`wuu-mascot-activity-prop wuu-mascot-activity-prop-${activity}`}
      aria-hidden="true"
    >
      <g className="wuu-mascot-activity-motion">
        {activity === "thinking" ? (
          <>
            <circle
              className="wuu-mascot-thinking-bubble"
              cx="79"
              cy="24"
              r="14"
            />
            <path
              className="wuu-mascot-activity-line"
              d="M 73 19 C 73.5 12 84.5 12 85.5 18.5 C 86.5 24 79 24 79 29"
            />
            <circle
              className="wuu-mascot-activity-solid"
              cx="79"
              cy="34.5"
              r="3"
            />
          </>
        ) : null}
        {activity === "search" ? (
          <>
            <circle
              className="wuu-mascot-activity-fill"
              cx="78"
              cy="53"
              r="10"
            />
            <path
              className="wuu-mascot-activity-line"
              d="M 85 61 L 95 72"
            />
          </>
        ) : null}
        {activity === "edit" ? (
          <>
            <path
              className="wuu-mascot-activity-fill"
              d="M 68 71 L 87 50 L 94 57 L 74 77 L 66 79 Z"
            />
            <path
              className="wuu-mascot-activity-line"
              d="M 84 54 L 91 61 M 68 72 L 74 77"
            />
          </>
        ) : null}
        {activity === "command" ? (
          <>
            <rect
              className="wuu-mascot-activity-fill"
              x="67"
              y="51"
              width="28"
              height="23"
              rx="6"
            />
            <path
              className="wuu-mascot-activity-line"
              d="M 73 58 L 78 62.5 L 73 67 M 82 67 L 89 67"
            />
          </>
        ) : null}
        {activity === "read" ? (
          <>
            <path
              className="wuu-mascot-activity-fill"
              d="M 68 50 Q 77 47 82 52 Q 87 47 96 50 L 96 73 Q 87 70 82 75 Q 77 70 68 73 Z"
            />
            <path
              className="wuu-mascot-activity-line"
              d="M 82 52 L 82 75"
            />
          </>
        ) : null}
        {activity === "tool" ? (
          <>
            <path
              className="wuu-mascot-activity-line"
              d="M 82 49 L 82 73 M 70 61 L 94 61 M 73.5 52.5 L 90.5 69.5 M 90.5 52.5 L 73.5 69.5"
            />
            <circle
              className="wuu-mascot-activity-fill"
              cx="82"
              cy="61"
              r="5.5"
            />
          </>
        ) : null}
      </g>
    </g>
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
      {accessory === "beret" ? (
        <path className="wuu-mascot-fill" d={`M ${centerX - 25} ${top + 8} Q ${centerX - 22} ${top - 5} ${centerX - 7} ${top - 10} Q ${centerX + 13} ${top - 14} ${centerX + 24} ${top + 1} Q ${centerX + 12} ${top + 12} ${centerX - 25} ${top + 8} Z`} />
      ) : null}
      {accessory === "party-hat" ? (
        <>
          <path className="wuu-mascot-fill" d={`M ${centerX - 20} ${top + 11} L ${centerX + 3} ${top - 13} L ${centerX + 20} ${top + 11} Q ${centerX} ${top + 16} ${centerX - 20} ${top + 11} Z`} />
          <circle className="wuu-mascot-fill" cx={centerX + 3} cy={top - 13} r="4" />
        </>
      ) : null}
      {accessory === "wizard-hat" ? (
        <>
          <path className="wuu-mascot-fill" d={`M ${centerX - 18} ${top + 11} Q ${centerX - 3} ${top - 3} ${centerX + 4} ${top - 16} Q ${centerX + 10} ${top - 4} ${centerX + 23} ${top + 10} Q ${centerX + 4} ${top + 15} ${centerX - 18} ${top + 11} Z`} />
          <path className="wuu-mascot-fill" d={`M ${centerX - 25} ${top + 10} Q ${centerX} ${top + 5} ${centerX + 27} ${top + 11} Q ${centerX + 8} ${top + 19} ${centerX - 25} ${top + 14} Z`} />
        </>
      ) : null}
      {accessory === "chef-hat" ? (
        <path className="wuu-mascot-fill" d={`M ${centerX - 23} ${top + 13} L ${centerX - 23} ${top + 4} Q ${centerX - 26} ${top - 5} ${centerX - 15} ${top - 7} Q ${centerX - 10} ${top - 16} ${centerX} ${top - 12} Q ${centerX + 10} ${top - 17} ${centerX + 16} ${top - 7} Q ${centerX + 27} ${top - 5} ${centerX + 23} ${top + 4} L ${centerX + 23} ${top + 13} Q ${centerX} ${top + 17} ${centerX - 23} ${top + 13} Z`} />
      ) : null}
      {accessory === "flower" ? (
        <g className="wuu-mascot-flower" transform={`translate(${body.cx - body.rx * 0.7} ${top + 3})`}>
          <circle className="wuu-mascot-petal" cx="0" cy="-6" r="5" />
          <circle className="wuu-mascot-petal" cx="6" cy="0" r="5" />
          <circle className="wuu-mascot-petal" cx="0" cy="6" r="5" />
          <circle className="wuu-mascot-petal" cx="-6" cy="0" r="5" />
          <circle className="wuu-mascot-flower-center" r="4" />
        </g>
      ) : null}
      {accessory === "halo" ? (
        <ellipse className="wuu-mascot-halo" cx={centerX} cy={top - 8} rx="22" ry="5.5" />
      ) : null}
      {accessory === "bow-tie" ? (
        <>
          <path className="wuu-mascot-fill" d={`M ${centerX - 3} ${bottom - 9} Q ${centerX - 17} ${bottom - 20} ${centerX - 23} ${bottom - 8} Q ${centerX - 18} ${bottom + 3} ${centerX - 3} ${bottom - 5} Z`} />
          <path className="wuu-mascot-fill" d={`M ${centerX + 3} ${bottom - 9} Q ${centerX + 17} ${bottom - 20} ${centerX + 23} ${bottom - 8} Q ${centerX + 18} ${bottom + 3} ${centerX + 3} ${bottom - 5} Z`} />
          <circle className="wuu-mascot-band" cx={centerX} cy={bottom - 7} r="5" />
        </>
      ) : null}
      {accessory === "graduation-cap" ? (
        <>
          <path className="wuu-mascot-fill" d={`M ${centerX - 28} ${top - 1} L ${centerX} ${top - 13} L ${centerX + 28} ${top - 1} L ${centerX} ${top + 10} Z`} />
          <path className="wuu-mascot-line" d={`M ${centerX + 20} ${top + 2} L ${centerX + 22} ${top + 14}`} />
          <circle className="wuu-mascot-band" cx={centerX + 22} cy={top + 16} r="3" />
        </>
      ) : null}
      {accessory === "cowboy-hat" ? (
        <>
          <path className="wuu-mascot-fill" d={`M ${centerX - 17} ${top + 9} Q ${centerX - 14} ${top - 9} ${centerX} ${top - 8} Q ${centerX + 15} ${top - 9} ${centerX + 18} ${top + 9} Z`} />
          <path className="wuu-mascot-fill" d={`M ${centerX - 30} ${top + 8} Q ${centerX - 17} ${top + 16} ${centerX} ${top + 10} Q ${centerX + 18} ${top + 16} ${centerX + 30} ${top + 8} Q ${centerX + 16} ${top + 22} ${centerX} ${top + 15} Q ${centerX - 17} ${top + 22} ${centerX - 30} ${top + 8} Z`} />
        </>
      ) : null}
      {accessory === "propeller-cap" ? (
        <>
          <path className="wuu-mascot-fill" d={`M ${centerX - 23} ${top + 10} Q ${centerX - 18} ${top - 8} ${centerX} ${top - 9} Q ${centerX + 19} ${top - 8} ${centerX + 23} ${top + 10} Z`} />
          <path className="wuu-mascot-line" d={`M ${centerX} ${top - 9} L ${centerX} ${top - 16}`} />
          <path className="wuu-mascot-fill" d={`M ${centerX} ${top - 16} Q ${centerX - 11} ${top - 18} ${centerX - 15} ${top - 12} Q ${centerX - 8} ${top - 8} ${centerX} ${top - 16} Z`} />
          <path className="wuu-mascot-fill" d={`M ${centerX} ${top - 16} Q ${centerX + 11} ${top - 18} ${centerX + 15} ${top - 12} Q ${centerX + 8} ${top - 8} ${centerX} ${top - 16} Z`} />
        </>
      ) : null}
      {accessory === "mushroom-cap" ? (
        <path className="wuu-mascot-fill" d={`M ${centerX - 29} ${top + 12} Q ${centerX - 23} ${top - 12} ${centerX} ${top - 14} Q ${centerX + 23} ${top - 12} ${centerX + 29} ${top + 12} Q ${centerX + 11} ${top + 5} ${centerX} ${top + 12} Q ${centerX - 11} ${top + 5} ${centerX - 29} ${top + 12} Z`} />
      ) : null}
      {accessory === "bunny-ears" ? (
        <>
          <path className="wuu-mascot-fill" d={`M ${centerX - 18} ${top + 7} Q ${centerX - 29} ${top - 13} ${centerX - 17} ${top - 17} Q ${centerX - 6} ${top - 12} ${centerX - 10} ${top + 8} Z`} />
          <path className="wuu-mascot-fill" d={`M ${centerX + 10} ${top + 8} Q ${centerX + 6} ${top - 12} ${centerX + 17} ${top - 17} Q ${centerX + 29} ${top - 13} ${centerX + 18} ${top + 7} Z`} />
        </>
      ) : null}
      {accessory === "cat-ears" ? (
        <>
          <path className="wuu-mascot-fill" d={`M ${centerX - 25} ${top + 10} L ${centerX - 20} ${top - 11} Q ${centerX - 8} ${top - 4} ${centerX - 4} ${top + 9} Z`} />
          <path className="wuu-mascot-fill" d={`M ${centerX + 4} ${top + 9} Q ${centerX + 8} ${top - 4} ${centerX + 20} ${top - 11} L ${centerX + 25} ${top + 10} Z`} />
        </>
      ) : null}
      {accessory === "ribbon" ? (
        <g transform={`translate(${body.cx + body.rx * 0.72} ${top + 5})`}>
          <path className="wuu-mascot-fill" d="M -2 0 Q -15 -11 -17 1 Q -14 12 -2 4 Z" />
          <path className="wuu-mascot-fill" d="M 2 0 Q 15 -11 17 1 Q 14 12 2 4 Z" />
          <circle className="wuu-mascot-band" cy="2" r="4.5" />
        </g>
      ) : null}
      {accessory === "necktie" ? (
        <>
          <path className="wuu-mascot-fill" d={`M ${centerX} ${bottom - 19} L ${centerX + 7} ${bottom - 14} L ${centerX} ${bottom - 9} L ${centerX - 7} ${bottom - 14} Z`} />
          <path className="wuu-mascot-fill" d={`M ${centerX} ${bottom - 9} L ${centerX - 8} ${bottom + 4} L ${centerX} ${bottom + 10} L ${centerX + 8} ${bottom + 4} Z`} />
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
