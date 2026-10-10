import assert from "node:assert/strict";
import test from "node:test";

import { clipText, creditStatus, markdownToText, maxTextBytes } from "./text.js";

test("markdown becomes plain lines", () => {
  const markdown = [
    "# Offshore wind",
    "",
    "Farms are the [backbone](https://example.com/a) of the grid.",
    "",
    "![turbine](https://example.com/t.jpg)",
    "",
    "See [National Grid][ng] and **steel** prices.",
    "",
    "[ng]: https://example.com/ng",
  ].join("\n");

  assert.equal(markdownToText(markdown), [
    "Offshore wind",
    "",
    "Farms are the backbone of the grid.",
    "",
    "See National Grid and steel prices.",
  ].join("\n"));
});

test("text at 64 KiB is kept", () => {
  const text = "a".repeat(maxTextBytes);
  assert.deepEqual(clipText(text), { text, truncated: false });
});

test("text past 64 KiB is clipped on a UTF-8 boundary", () => {
  const head = "a".repeat(maxTextBytes - 1);
  const text = head + "é" + "zzz";
  assert.deepEqual(clipText(text), { text: head, truncated: true });
});

test("a zero balance disables URL loading", () => {
  assert.deepEqual(creditStatus({
    success: true,
    data: {
      remainingCredits: 0,
      planCredits: 1000,
      billingPeriodEnd: "2026-11-01T00:00:00.000Z",
    },
  }), {
    available: false,
    resetsAt: "2026-11-01T00:00:00.000Z",
  });
});

test("credits in a later period enable URL loading", () => {
  assert.deepEqual(creditStatus({
    success: true,
    data: {
      remainingCredits: 1000,
      planCredits: 1000,
      billingPeriodEnd: "2026-12-01T00:00:00.000Z",
    },
  }), {
    available: true,
    resetsAt: "2026-12-01T00:00:00.000Z",
  });
});
