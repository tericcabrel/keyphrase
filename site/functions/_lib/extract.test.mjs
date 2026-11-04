import assert from "node:assert/strict";
import test from "node:test";

import { defaultFilterSettings, isConfigured, validateExtractBody, tryPageBody } from "./extract.js";

test("isConfigured needs origin and token", () => {
  assert.equal(isConfigured({}), false);
  assert.equal(isConfigured({ EXTRACT_ORIGIN: "https://x" }), false);
  assert.equal(isConfigured({ EXTRACT_ORIGIN: "https://x", EXTRACT_TOKEN: "t" }), true);
});

test("validateExtractBody rejects bad language", () => {
  const result = validateExtractBody({ text: "hello", language: "nl" });
  assert.equal(result.status, 400);
});

test("validateExtractBody builds upstream payload", () => {
  const result = validateExtractBody({ text: "Offshore wind", language: "en", limit: 3 });
  assert.equal(result.body.language, "en");
  assert.equal(result.body.counts, true);
  assert.deepEqual(result.body.drop_lone_nouns, defaultFilterSettings.drop_lone_nouns);
});

test("tryPageBody maps keywords and counts", () => {
  const mapped = tryPageBody({
    language: "en",
    keywords: [{ text: "wind", start_byte: 0, end_byte: 4, score: 1 }],
    counts: { words: 2, extracted: 1, returned: 1, extracted_more_than_one_word: 0, returned_more_than_one_word: 0, more_than_one_word: [] },
  });
  assert.equal(mapped.keywords.length, 1);
  assert.equal(mapped.counts.words, 2);
});
