import { describe, expect, it } from "vitest";
import { type Command, cycle, matchCommands } from "./palette";

const cmd = (id: string, label: string, english: string): Command => ({ id, label, english, group: "", run: () => {} });
const commands = [cmd("pause", "Jeda semua", "Pause all"), cmd("limit", "Nyalakan pembatas kecepatan", "Turn the speed limiter on"), cmd("video", "Video", "Video")];

describe("palette", () => {
  it("matches every word, in either language, ignoring case and accents", () => {
    expect(matchCommands(commands, "").map((c) => c.id)).toEqual(["pause", "limit", "video"]);
    expect(matchCommands(commands, "jeda").map((c) => c.id)).toEqual(["pause"]);
    expect(matchCommands(commands, "PAUSE").map((c) => c.id)).toEqual(["pause"]);
    expect(matchCommands(commands, "speed on").map((c) => c.id)).toEqual(["limit"]);
    expect(matchCommands(commands, "vídeo").map((c) => c.id)).toEqual(["video"]);
    expect(matchCommands(commands, "ubuntu")).toEqual([]);
  });

  it("wraps the highlight", () => {
    expect(cycle(0, 1, 3)).toBe(1);
    expect(cycle(2, 1, 3)).toBe(0);
    expect(cycle(0, -1, 3)).toBe(2);
    expect(cycle(0, 1, 0)).toBe(-1);
  });
});
