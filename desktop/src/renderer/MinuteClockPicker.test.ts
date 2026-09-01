import { describe, expect, it } from "vitest";
import { minuteFromClockPoint } from "./MinuteClockPicker";

describe("minuteFromClockPoint", () => {
  it("maps the clock face to all minute quadrants", () => {
    expect(minuteFromClockPoint(0, -1)).toBe(0);
    expect(minuteFromClockPoint(1, 0)).toBe(15);
    expect(minuteFromClockPoint(0, 1)).toBe(30);
    expect(minuteFromClockPoint(-1, 0)).toBe(45);
  });

  it("supports precise non-sequential minute selection", () => {
    expect(minuteFromClockPoint(-Math.sqrt(3), -1)).toBe(50);
  });
});
