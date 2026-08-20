import { renderToStaticMarkup } from "react-dom/server";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AgentAvatarMark } from "./AgentAvatarMark";

const blobatarProps = vi.hoisted(() => vi.fn());

vi.mock("blobatar/react", () => ({
  Blobatar: (props: unknown) => {
    blobatarProps(props);
    return null;
  },
}));

describe("AgentAvatarMark", () => {
  beforeEach(() => blobatarProps.mockClear());

  it.each(["idle", "thinking", "sending"] as const)(
    "renders the %s blob without a backdrop",
    (status) => {
      renderToStaticMarkup(
        <AgentAvatarMark seed="agent-1" avatarKey="abstract-1" status={status} />,
      );

      expect(blobatarProps).toHaveBeenCalledOnce();
      expect(blobatarProps.mock.calls[0][0]).toEqual(
        expect.objectContaining({ background: false }),
      );
    },
  );
});
