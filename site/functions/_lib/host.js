// The rate limiting rule on /page lives on the custom domain's zone, so any
// other host, such as keyphrase.pages.dev, would reach Firecrawl unthrottled.
const allowedHosts = new Set(["keyphrase.tericcabrel.com", "localhost", "127.0.0.1"]);

export function allowedHost(requestURL) {
  try {
    return allowedHosts.has(new URL(requestURL).hostname);
  } catch {
    return false;
  }
}
