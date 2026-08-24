import "blobatar/motion.css";
import type { JSX } from "react";
import { AVATAR_HUES } from "./DefaultAvatar";
import { WuuMascot, type WuuMascotAccessory } from "./WuuMascot";
import { WUU_MASCOT_TRAITS } from "./wuu-mascot-spec";

export const AGENT_AVATAR_KEYS = [
  "abstract-1",
  "abstract-2",
  "abstract-3",
  "abstract-4",
  "abstract-5",
  "abstract-6",
  "abstract-7",
  "abstract-8",
  "abstract-9",
] as const;

export type AgentAvatarKey = (typeof AGENT_AVATAR_KEYS)[number];

export type AgentAvatarStatus = "idle" | "thinking" | "sending";

export const AGENT_AVATAR_SHAPES = [
  { id: "round", trait: 0.12 },
  { id: "organic", trait: 0.42 },
  { id: "boxy", trait: 0.65 },
  { id: "nub", trait: 0.78 },
  { id: "cloud", trait: 0.88 },
  { id: "sun", trait: 0.97 },
] as const;

export const AGENT_AVATAR_ACCESSORIES = [
  "none",
  "cap",
  "beanie",
  "top-hat",
  "sprout",
  "crown",
  "headphones",
  "scarf",
  "beret",
  "party-hat",
  "wizard-hat",
  "chef-hat",
  "flower",
  "halo",
  "bow-tie",
  "graduation-cap",
  "cowboy-hat",
  "propeller-cap",
  "mushroom-cap",
  "bunny-ears",
  "cat-ears",
  "ribbon",
  "necktie",
] as const satisfies readonly WuuMascotAccessory[];

export type AgentAvatarShape = (typeof AGENT_AVATAR_SHAPES)[number]["id"];

export type AgentAvatarConfig = {
  shape: AgentAvatarShape;
  accessory: WuuMascotAccessory;
  hue: number;
};

const AGENT_AVATAR_CONFIG_PREFIX = "mascot-v1";
const DEFAULT_AGENT_AVATAR_CONFIG: AgentAvatarConfig = { shape: "round", accessory: "none", hue: AVATAR_HUES[0] };

export function isAgentAvatarKey(value: string): value is AgentAvatarKey {
  return AGENT_AVATAR_KEYS.some((key) => key === value);
}

export function randomAgentAvatarKey(): string {
  const value = new Uint32Array(1);
  window.crypto.getRandomValues(value);
  const shape = AGENT_AVATAR_SHAPES[value[0] % AGENT_AVATAR_SHAPES.length].id;
  const hue = AVATAR_HUES[Math.floor(value[0] / AGENT_AVATAR_SHAPES.length) % AVATAR_HUES.length];
  return serializeAgentAvatarConfig({ shape, accessory: "none", hue });
}

export function parseAgentAvatarConfig(value: string): AgentAvatarConfig | null {
  const [prefix, shape, accessory, rawHue, ...rest] = value.split(":");
  if (prefix !== AGENT_AVATAR_CONFIG_PREFIX || rest.length > 0) return null;
  if (!AGENT_AVATAR_SHAPES.some((item) => item.id === shape)) return null;
  if (!AGENT_AVATAR_ACCESSORIES.some((item) => item === accessory)) return null;
  const hue = Number(rawHue);
  if (!Number.isInteger(hue) || hue < 0 || hue > 359) return null;
  return { shape: shape as AgentAvatarShape, accessory: accessory as WuuMascotAccessory, hue };
}

export function serializeAgentAvatarConfig(config: AgentAvatarConfig): string {
  return `${AGENT_AVATAR_CONFIG_PREFIX}:${config.shape}:${config.accessory}:${Math.round(config.hue)}`;
}

export function agentAvatarConfig(value: string): AgentAvatarConfig {
  const configured = parseAgentAvatarConfig(value);
  if (configured) return configured;
  if (isAgentAvatarKey(value)) {
    const index = AGENT_AVATAR_KEYS.indexOf(value);
    return {
      shape: AGENT_AVATAR_SHAPES[index % AGENT_AVATAR_SHAPES.length].id,
      accessory: "none",
      hue: AVATAR_HUES[index % AVATAR_HUES.length],
    };
  }
  return DEFAULT_AGENT_AVATAR_CONFIG;
}

export function AgentAvatarMark({ seed, avatarKey, avatarImage, status = "idle" }: {
  seed: string;
  avatarKey: string;
  avatarImage?: string;
  status?: AgentAvatarStatus;
}): JSX.Element {
  void seed;
  if (avatarImage) {
    return (
      <span className="agent-avatar-mark" aria-hidden="true">
        <img className="agent-avatar-image" src={avatarImage} alt="" draggable={false} />
      </span>
    );
  }

  const config = agentAvatarConfig(avatarKey);
  const shape = AGENT_AVATAR_SHAPES.find((item) => item.id === config.shape) ?? AGENT_AVATAR_SHAPES[0];
  return (
    <span className="agent-avatar-mark" aria-hidden="true">
      <WuuMascot
        identityName={`agent-avatar:${avatarKey}`}
        identityHue={config.hue}
        identityTraits={{ ...WUU_MASCOT_TRAITS, shape: shape.trait }}
        accessory={config.accessory}
        activity={status === "thinking" ? "thinking" : status === "sending" ? "compose" : "idle"}
      />
    </span>
  );
}
