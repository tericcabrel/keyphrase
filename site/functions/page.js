import { allowedHost } from "./_lib/host.js";
import { clipText, creditStatus, markdownToText } from "./_lib/text.js";

const scrapeURL = "https://api.firecrawl.dev/v2/scrape";
const creditURL = "https://api.firecrawl.dev/v2/team/credit-usage";

export async function onRequestGet(context) {
  const key = context.env.FIRECRAWL_API_KEY;

  if (!key || !allowedHost(context.request.url)) {
    return json({ configured: false });
  }

  const usage = await fetchCredits(key);

  if (!usage) {
    return json({ configured: true, available: true, resetsAt: null });
  }

  return json({ configured: true, ...creditStatus(usage) });
}

export async function onRequestPost(context) {
  const key = context.env.FIRECRAWL_API_KEY;

  if (!key) {
    return json({ error: "URL loading is not configured." }, 503);
  }

  if (!allowedHost(context.request.url)) {
    return json({ error: "URL loading is not available on this host." }, 403);
  }

  let payload;

  try {
    payload = await context.request.json();
  } catch {
    return json({ error: "The request body must be JSON." }, 400);
  }

  const url = payload && typeof payload.url === "string" ? payload.url.trim() : "";

  if (!isHTTP(url)) {
    return json({ error: "The URL must be http or https." }, 400);
  }

  const usage = await fetchCredits(key);
  const status = usage ? creditStatus(usage) : null;

  if (status && !status.available) {
    return json({
      error: "URL loading is unavailable.",
      unavailable: true,
      resetsAt: status.resetsAt,
    }, 402);
  }

  let scraped;
  try {
    scraped = await fetch(scrapeURL, {
      method: "POST",
      headers: {
        authorization: "Bearer " + key,
        "content-type": "application/json",
      },
      body: JSON.stringify({
        url,
        formats: ["markdown"],
        onlyMainContent: true,
      }),
      signal: AbortSignal.timeout(60000),
    });
  } catch {
    return json({ error: "The page could not be loaded." }, 502);
  }

  if (scraped.status === 402) {
    const again = await fetchCredits(key);

    return json({
      error: "URL loading is unavailable.",
      unavailable: true,
      resetsAt: again ? creditStatus(again).resetsAt : null,
    }, 402);
  }

  if (scraped.status === 429) {
    return json({ error: "URL loading is busy. Wait a moment and try again." }, 429);
  }

  if (!scraped.ok) {
    return json({ error: "The page could not be loaded." }, 502);
  }

  const body = await scraped.json();
  const markdown = (body && body.data && body.data.markdown) || (body && body.markdown) || "";
  const clipped = clipText(markdownToText(markdown));

  return json({ text: clipped.text, truncated: clipped.truncated });
}

function isHTTP(raw) {
  try {
    const url = new URL(raw);
    return url.protocol === "http:" || url.protocol === "https:";
  } catch {
    return false;
  }
}

async function fetchCredits(key) {
  try {
    const response = await fetch(creditURL, {
      headers: { authorization: "Bearer " + key },
      signal: AbortSignal.timeout(15000),
    });

    if (!response.ok) {
      return null;
    }
    
    return await response.json();
  } catch {
    return null;
  }
}

function json(body, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json; charset=utf-8" },
  });
}
