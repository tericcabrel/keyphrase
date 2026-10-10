import assert from "node:assert/strict";
import test from "node:test";

import { allowedHost } from "./host.js";

test("URL loading is allowed only on the custom domain and local hosts", () => {
  const cases = [
    { url: "https://keyphrase.tericcabrel.com/page", want: true },
    { url: "http://localhost:8788/page", want: true },
    { url: "http://127.0.0.1:8788/page", want: true },
    { url: "https://keyphrase.pages.dev/page", want: false },
    { url: "https://3f2a1b.keyphrase.pages.dev/page", want: false },
    { url: "https://keyphrase.tericcabrel.com.evil.example/page", want: false },
    { url: "https://tericcabrel.com/page", want: false },
    { url: "not a url", want: false },
  ];

  for (const { url, want } of cases) {
    assert.equal(allowedHost(url), want, url);
  }
});
