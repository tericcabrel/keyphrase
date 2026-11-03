import { allowedHost } from "./_lib/host.js";
import {
  defaultFilterSettings,
  isConfigured,
  tryPageBody,
  validateExtractBody,
} from "./_lib/extract.js";

export async function onRequestGet(context) {
  if (!allowedHost(context.request.url)) {
    return json({ configured: false }, 403);
  }
  const configured = isConfigured(context.env);
  const body = { configured };
  if (configured) {
    body.defaults = defaultFilterSettings;
  }

  return json(body);
}

export async function onRequestPost(context) {
  if (!isConfigured(context.env)) {
    return json({ error: "Extraction is not configured." }, 503);
  }
  if (!allowedHost(context.request.url)) {
    return json({ error: "Extraction is not available on this host." }, 403);
  }

  let payload;
  try {
    payload = await context.request.json();
  } catch {
    return json({ error: "The request body must be JSON." }, 400);
  }

  const validated = validateExtractBody(payload);
  if (validated.error) {
    return json({ error: validated.error }, validated.status);
  }

  const origin = String(context.env.EXTRACT_ORIGIN).replace(/\/$/, "");
  let upstream;
  try {
    upstream = await fetch(`${origin}/v1/keywords`, {
      method: "POST",
      headers: {
        authorization: "Bearer " + context.env.EXTRACT_TOKEN,
        "content-type": "application/json",
      },
      body: JSON.stringify(validated.body),
      signal: AbortSignal.timeout(30000),
    });
  } catch {
    return json({ error: "Extraction failed." }, 502);
  }

  let body;
  try {
    body = await upstream.json();
  } catch {
    return json({ error: "Extraction failed." }, 502);
  }
  if (!upstream.ok) {
    const message = body && body.error ? body.error : "Extraction failed.";
    return json({ error: message }, upstream.status);
  }

  return json(tryPageBody(body));
}

function json(body, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json; charset=utf-8" },
  });
}
