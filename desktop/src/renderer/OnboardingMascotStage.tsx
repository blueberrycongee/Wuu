import { _layout } from "blobatar";
import type { CSSProperties, ReactNode } from "react";
import { ENGINE_ICON_PATHS } from "./EngineIcons";
import { WuuMascot } from "./WuuMascot";
import { WUU_MASCOT_NAME, WUU_MASCOT_TRAITS } from "./wuu-mascot-spec";

export const ONBOARDING_PLUGIN_ORDER = [
  "ask-user", "todo", "automation", "subagent", "memory", "dream", "note-compaction",
] as const;

export type OnboardingPluginID = (typeof ONBOARDING_PLUGIN_ORDER)[number];

const COMPANIONS = ["coral", "blue", "sage"] as const;
const { body } = _layout(WUU_MASCOT_NAME, { traits: WUU_MASCOT_TRAITS });
// Equipment is authored around a radius-40 body, then fitted to Wuu's actual
// identity geometry. The face and equipment share one moving parent.
const equipmentFit = `translate(${body.cx} ${body.cy}) scale(${body.rx / 40} ${body.ry / 40}) translate(-50 -50)`;

export function OnboardingMascotStage({
  pluginIDs,
  engineID,
}: {
  pluginIDs?: readonly string[];
  engineID?: string;
}): JSX.Element {
  const worn = new Set(pluginIDs ?? []);
  const split = worn.has("subagent");
  const engineMark = engineID && engineID !== "wuu" ? ENGINE_ICON_PATHS[engineID] : undefined;

  return (
    <div
      className={`onboarding-plugin-mascot-stage${split ? " is-split" : ""}`}
      data-testid="onboarding-mascot-stage"
      data-onboarding-split={split ? "" : undefined}
      data-onboarding-engine={engineID || undefined}
      aria-hidden="true"
    >
      <div className="onboarding-mascot-pack">
        {COMPANIONS.map((color, index) => (
          <div
            key={color}
            className={`onboarding-mascot-clone onboarding-mascot-clone-${index}`}
            data-onboarding-companion={color}
            hidden={index !== 0 && !split}
          >
            <div className="onboarding-mascot-body">
              <div className="onboarding-mascot-pose">
                <WuuMascot
                  className="onboarding-mascot"
                  size={200}
                  activity="compose"
                  accessory="none"
                  animate="hover"
                  followPointer
                  style={{ "--mo-head": `var(--companion-${color})`, "--mo-eye": "var(--equipment-ink)" } as CSSProperties}
                />
                {index === 0 ? <CompanionEquipment worn={worn} engineMark={engineMark} engineID={engineID} /> : null}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function CompanionEquipment({
  worn,
  engineMark,
  engineID,
}: {
  worn: ReadonlySet<string>;
  engineMark?: string;
  engineID?: string;
}): JSX.Element {
  const hasBelt = ["todo", "automation", "memory", "dream", "note-compaction"].some((id) => worn.has(id));
  const hasPocket = worn.has("memory") || worn.has("dream");
  function capability(id: OnboardingPluginID, content: ReactNode): ReactNode {
    if (!worn.has(id)) return null;
    return (
      <g key={id} className="onboarding-equipment-module" data-onboarding-capability={id}>
        {content}
      </g>
    );
  }

  return (
    <svg className="onboarding-mascot-equipment" viewBox="0 0 100 100" aria-hidden="true">
      <g transform={equipmentFit}>
        {engineMark ? (
          <g className="onboarding-equipment-module" data-onboarding-engine-mark={engineID}>
            <g className="onboarding-engine-mark" transform="translate(68 18) scale(0.72)">
              <path d={engineMark} />
            </g>
          </g>
        ) : null}
        {hasBelt ? (
          <g className="onboarding-equipment-belt">
            <path className="equipment-edge" d="M14 65 Q49 84 86 64 L83 74 Q51 94 18 76 Z" />
            <path className="equipment-fabric" d="M14 63 Q49 82 86 62 L84 71 Q51 91 17 73 Z" />
            <path className="equipment-seam" d="M20 72 Q48 85 78 73" />
          </g>
        ) : null}
        {capability("ask-user", <>
          <path className="equipment-edge" d="M13 40 Q9 49 13 58 L17 57 Q14 48 18 41 Z" />
          <rect className="equipment-shell" x="9" y="43" width="9" height="14" rx="4.5" transform="rotate(8 13 50)" />
          <path className="equipment-detail" d="M13 47 V52" />
          <path className="equipment-listen" d="M5 44 Q2 49 5 54" />
        </>)}
        {capability("todo", <>
          <path className="equipment-edge" d="M28 72 Q40 76 51 76 L51 84 Q38 83 26 79 Z" />
          <path className="equipment-progress-done" d="M31 76 L34 77" />
          <path className="equipment-progress-current" d="M39 78 L42 78.5" />
          <path className="equipment-progress-next" d="M47 79 L49 79" />
        </>)}
        {capability("automation", <>
          <circle className="equipment-edge" cx="22" cy="70" r="7.5" />
          <circle className="equipment-shell" cx="22" cy="69" r="6" />
          <path className="equipment-seam" d="M22 64 V65 M27 69 H26 M22 74 V73 M17 69 H18" />
          <path className="equipment-detail equipment-clock-hand" d="M22 65.5 V69 L24.5 70" />
        </>)}
        {hasPocket ? <path className="equipment-edge" d="M65 61 L82 59 Q87 59 86 65 L84 79 Q82 84 70 84 Q65 83 65 79 Z" /> : null}
        {capability("memory", <g className="equipment-notebook">
          <rect className="equipment-binding" x="66" y="55" width="15" height="20" rx="2.5" transform="rotate(-7 73 65)" />
          <path className="equipment-paper" d="M69 55 L79 54 V72 L69 74 Z" />
          <path className="equipment-seam" d="M72 59 L77 58.5 M72 62 L77 61.5" />
          <path className="equipment-bookmark" d="M74 54 L77 53.5 V59 L75.5 58 L74 59 Z" />
        </g>)}
        {hasPocket ? <>
          <path className="equipment-fabric" d="M63 66 Q75 70 86 64 L84 78 Q82 83 70 82 Q64 82 64 77 Z" />
          <path className="equipment-seam" d="M68 77 Q75 80 81 77" />
        </> : null}
        {capability("dream", <>
          <g className="equipment-sort-sheet equipment-sort-sheet-back"><rect className="equipment-paper" x="77" y="58" width="7" height="10" rx="1.5" /></g>
          <g className="equipment-sort-sheet"><rect className="equipment-shell" x="77" y="59" width="7" height="10" rx="1.5" /></g>
          <path className="equipment-detail" d="M80 62 V68 Q80 71 82 70 L83 69" />
        </>)}
        {capability("note-compaction", <>
          <path className="equipment-edge" d="M42 85 L61 84 L62 92 L42 93 Z" />
          <g className="equipment-folded-note">
            <path className="equipment-paper" d="M43 85 L60 84 L61 90 L44 91 Z" />
            <path className="equipment-seam" d="M46 87 L58 86 M46 89 L54 88.5" />
          </g>
          <path className="equipment-fabric" d="M48 84 L52 84 L53 92 L49 92 Z" />
          <path className="equipment-bookmark" d="M56 89 L59 89 L59 95 L57.5 94 L56 95 Z" />
        </>)}
      </g>
    </svg>
  );
}
