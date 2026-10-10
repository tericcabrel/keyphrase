export const maxTextBytes = 64 * 1024;

// markdownToText turns Firecrawl markdown into lines the extractor can read.
// Heading marks drop, a link keeps its visible words, and images drop.
export function markdownToText(markdown) {
  let text = String(markdown ?? "").replace(/\r\n/g, "\n");
  text = text.replace(/!\[[^\]]*]\([^)]*\)/g, "");
  text = text.replace(/!\[[^\]]*]\[[^\]]*]/g, "");
  text = text.replace(/\[([^\]]*)]\([^)]*\)/g, "$1");
  text = text.replace(/\[([^\]]*)]\[[^\]]*]/g, "$1");
  text = text.replace(/^\[[^\]]+]:\s+\S+.*$/gm, "");
  text = text.replace(/^#{1,6}[ \t]+/gm, "");
  text = text.replace(/\*\*([^*\n]+)\*\*/g, "$1");
  text = text.replace(/__([^_\n]+)__/g, "$1");
  text = text.replace(/(^|\s)\*([^*\n]+)\*(?=\s|$)/g, "$1$2");
  text = text.replace(/(^|\s)_([^_\n]+)_(?=\s|$)/g, "$1$2");
  text = text.replace(/[ \t]+\n/g, "\n");
  text = text.replace(/\n{3,}/g, "\n\n");
  return text.trim();
}

export function clipText(text) {
  const bytes = new TextEncoder().encode(text);
  if (bytes.length <= maxTextBytes) {
    return { text, truncated: false };
  }
  let end = maxTextBytes;
  while (end > 0 && (bytes[end] & 0xc0) === 0x80) {
    end--;
  }
  const cut = new TextDecoder().decode(bytes.subarray(0, end));
  return { text: cut, truncated: true };
}

// creditStatus reads a GET /v2/team/credit-usage body.
// available is false when remainingCredits is 0. A later period with credits
// left sets available back to true.
export function creditStatus(body) {
  const data = body && body.data ? body.data : {};
  const remaining = data.remainingCredits;
  return {
    available: typeof remaining === "number" && remaining > 0,
    resetsAt: data.billingPeriodEnd || null,
  };
}
