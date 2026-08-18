import { happy } from "blobatar/expression";
import { Blobatar } from "blobatar/react";
import "blobatar/motion.css";
import type { JSX } from "react";
import { AVATAR_HUES, avatarHueIndex } from "./DefaultAvatar";

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

export function isAgentAvatarKey(value: string): value is AgentAvatarKey {
  return AGENT_AVATAR_KEYS.some((key) => key === value);
}

export function randomAgentAvatarKey(): AgentAvatarKey {
  const value = new Uint32Array(1);
  window.crypto.getRandomValues(value);
  return AGENT_AVATAR_KEYS[value[0] % AGENT_AVATAR_KEYS.length];
}

export function AgentAvatarMark({ seed, avatarKey, avatarImage, status = "idle" }: {
  seed: string;
  avatarKey: string;
  avatarImage?: string;
  status?: AgentAvatarStatus;
}): JSX.Element {
  if (avatarImage) {
    return (
      <span className="agent-avatar-mark" aria-hidden="true">
        <img className="agent-avatar-image" src={avatarImage} alt="" draggable={false} />
      </span>
    );
  }

  const identity = seed.trim() || "agent";
  const name = `${identity}:${isAgentAvatarKey(avatarKey) ? avatarKey : AGENT_AVATAR_KEYS[0]}`;
  const hue = AVATAR_HUES[avatarHueIndex(identity)];

  if (status === "idle") {
    return (
      <span className="agent-avatar-mark" aria-hidden="true">
        <Blobatar name={name} hue={hue} background="circle" alt="" draggable={false} />
      </span>
    );
  }

  return (
    <span className="agent-avatar-mark" aria-hidden="true">
      <Blobatar
        name={name}
        hue={hue}
        background="circle"
        animate="always"
        expression={status === "sending" ? happy : undefined}
        focusable={false}
      />
    </span>
  );
}
