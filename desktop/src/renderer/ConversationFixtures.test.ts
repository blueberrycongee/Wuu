import { afterEach, describe, expect, it } from "vitest";
import { createConversationFixture } from "./ConversationFixtures";
import { setActiveLocale } from "./i18n";

afterEach(() => setActiveLocale("zh-CN"));

describe("localized conversation fixtures", () => {
  it("builds an English conversation sample", () => {
    setActiveLocale("en-US");
    expect(createConversationFixture("long", "/tmp/demo").preview).toBe(
      "Demo: long-form reading width",
    );
  });
});
