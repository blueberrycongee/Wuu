import { describe, expect, it } from "vitest";

import type {
  ConversationProcessItemV1 as SDKConversationProcessItemV1,
  ConversationProcessSnapshotV1 as SDKConversationProcessSnapshotV1,
} from "../../../packages/plugin-sdk/src/index";
import type {
  ConversationProcessItemV1 as DesktopConversationProcessItemV1,
  ConversationProcessSnapshotV1 as DesktopConversationProcessSnapshotV1,
} from "./workbench";

type Extends<Left, Right> = Left extends Right ? true : false;
type Assert<T extends true> = T;

type SDKSnapshotMatchesDesktop = Assert<Extends<SDKConversationProcessSnapshotV1, DesktopConversationProcessSnapshotV1>>;
type DesktopSnapshotMatchesSDK = Assert<Extends<DesktopConversationProcessSnapshotV1, SDKConversationProcessSnapshotV1>>;
type SDKItemMatchesDesktop = Assert<Extends<SDKConversationProcessItemV1, DesktopConversationProcessItemV1>>;
type DesktopItemMatchesSDK = Assert<Extends<DesktopConversationProcessItemV1, SDKConversationProcessItemV1>>;

describe("SDK/Desktop workbench contract parity", () => {
  it("keeps the conversation process V1 snapshot assignable in both directions", () => {
    const assertions: readonly [
      SDKSnapshotMatchesDesktop,
      DesktopSnapshotMatchesSDK,
      SDKItemMatchesDesktop,
      DesktopItemMatchesSDK,
    ] = [true, true, true, true];
    expect(assertions).toEqual([true, true, true, true]);
  });
});
