import { describe, expect, it } from "vitest";
import {
  OPTIMISTIC_TURN_ID_PREFIX,
  threadHasAcceptedComposerMessage,
} from "./ComposerMessages";
import type { Turn } from "../shared/protocol";

function turnWithUserText(id: string, text: string): Turn {
  return {
    id,
    status: "in_progress",
    items_view: "full",
    items: [
      {
        id: `${id}-user`,
        type: "user_message",
        status: "completed",
        text,
      },
    ],
  };
}

describe("threadHasAcceptedComposerMessage", () => {
  it("ignores the local optimistic placeholder for the same text", () => {
    expect(
      threadHasAcceptedComposerMessage(
        {
          turns: [
            turnWithUserText(`${OPTIMISTIC_TURN_ID_PREFIX}local`, "keep this sent"),
          ],
        },
        { text: "keep this sent" },
        `${OPTIMISTIC_TURN_ID_PREFIX}local`,
      ),
    ).toBe(false);
  });

  it("recognizes a later server turn that already carries the sent text", () => {
    expect(
      threadHasAcceptedComposerMessage(
        {
          turns: [
            turnWithUserText(`${OPTIMISTIC_TURN_ID_PREFIX}local`, "keep this sent"),
            turnWithUserText("turn-follow-up", "keep this sent"),
          ],
        },
        { text: "keep this sent" },
        `${OPTIMISTIC_TURN_ID_PREFIX}local`,
      ),
    ).toBe(true);
  });
});
