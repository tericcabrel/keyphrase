import { maxTextBytes } from "./text.js";

export const defaultFilterSettings = {
  drop_lone_nouns: true,
  lone_noun_min_rarity: 0.6,
  drop_one_off_phrases: true,
  one_off_min_others: 10,
};

const languages = new Set(["en", "fr", "de", "es", "it", "pt"]);

export function isConfigured(env) {
  return Boolean(env.EXTRACT_ORIGIN && env.EXTRACT_TOKEN);
}

export function validateExtractBody(payload) {
  if (!payload || typeof payload !== "object" || Array.isArray(payload)) {
    return { status: 400, error: "The request body must be JSON." };
  }
  const text = typeof payload.text === "string" ? payload.text : null;
  if (text === null) {
    return { status: 400, error: "text is required" };
  }
  const language = typeof payload.language === "string" ? payload.language.trim().toLowerCase() : "";
  if (!languages.has(language)) {
    return { status: 400, error: `unsupported language ${JSON.stringify(payload.language)}` };
  }
  const limit = payload.limit == null ? 10 : payload.limit;
  if (typeof limit !== "number" || !Number.isInteger(limit) || limit < 1) {
    return { status: 400, error: "limit must be at least 1" };
  }
  const bytes = new TextEncoder().encode(text);
  if (bytes.length > maxTextBytes) {
    return { status: 413, error: `text exceeds ${maxTextBytes} bytes` };
  }
  const settings = readFilterSettings(payload);
  if (settings.error) {
    return { status: 400, error: settings.error };
  }

  return {
    body: {
      text,
      language,
      limit,
      counts: true,
      ...settings.value,
    },
  };
}

function readFilterSettings(payload) {
  const value = { ...defaultFilterSettings };
  if (payload.drop_lone_nouns != null) {
    if (typeof payload.drop_lone_nouns !== "boolean") {
      return { error: 'field "drop_lone_nouns" has the wrong type' };
    }
    value.drop_lone_nouns = payload.drop_lone_nouns;
  }
  if (payload.lone_noun_min_rarity != null) {
    const rarity = payload.lone_noun_min_rarity;
    if (typeof rarity !== "number" || !(rarity >= 0 && rarity <= 1)) {
      return { error: "Rarity must be a number from 0 to 1." };
    }
    value.lone_noun_min_rarity = rarity;
  }
  if (payload.drop_one_off_phrases != null) {
    if (typeof payload.drop_one_off_phrases !== "boolean") {
      return { error: 'field "drop_one_off_phrases" has the wrong type' };
    }
    value.drop_one_off_phrases = payload.drop_one_off_phrases;
  }
  if (payload.one_off_min_others != null) {
    const minOthers = payload.one_off_min_others;
    if (typeof minOthers !== "number" || !Number.isInteger(minOthers) || minOthers < 0) {
      return { error: "The number of other phrases must be a whole number, 0 or more." };
    }
    value.one_off_min_others = minOthers;
  }

  return { value };
}

export function tryPageBody(upstream) {
  if (upstream.error) {
    return { error: upstream.error };
  }
  return {
    keywords: upstream.keywords ?? [],
    counts: upstream.counts ?? emptyCounts(),
  };
}

function emptyCounts() {
  return {
    words: 0,
    extracted: 0,
    returned: 0,
    extracted_more_than_one_word: 0,
    returned_more_than_one_word: 0,
    more_than_one_word: [],
  };
}
