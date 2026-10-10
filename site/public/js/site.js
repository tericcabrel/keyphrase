(function () {
  const root = document.documentElement;
  const toggle = document.querySelector(".theme-toggle");

  function highlightGo() {
    const blocks = document.querySelectorAll("pre code.language-go");
    if (blocks.length === 0) {
      return;
    }
    import("/js/lib/highlight.js").then(function (mod) {
      blocks.forEach(function (block) {
        block.innerHTML = mod.highlight(block.textContent);
      });
    }).catch(function () {});
  }

  function themeButton() {
    const dark = root.getAttribute("data-theme") === "dark";
    toggle.setAttribute("aria-label", dark ? "Switch to light theme" : "Switch to dark theme");
  }

  themeButton();
  toggle.addEventListener("click", function () {
    const next = root.getAttribute("data-theme") === "dark" ? "light" : "dark";
    root.setAttribute("data-theme", next);
    localStorage.setItem("theme", next);
    themeButton();
  });

  highlightGo();

  const form = document.querySelector("#try");
  if (!form) {
    return;
  }

  const language = document.querySelector("#language");
  const urlInput = document.querySelector("#url");
  const urlError = document.querySelector("#url-error");
  const loadButton = document.querySelector("#load");
  const text = document.querySelector("#text");
  const limitInput = document.querySelector("#limit");
  const extractButton = document.querySelector("#extract");
  const wasmNote = document.querySelector("#wasm-note");
  const quotaNote = document.querySelector("#quota-note");
  const status = document.querySelector("#status");
  const results = document.querySelector("#results");
  const keywordList = document.querySelector("#keywords");
  const passage = document.querySelector("#passage");
  const countsList = document.querySelector("#counts");
  const more = document.querySelector("#more");
  const tabs = Array.from(document.querySelectorAll("#results [role='tab']"));

  tabs.forEach(function (tab) {
    tab.addEventListener("click", function () {
      selectTab(tab);
    });
    tab.addEventListener("keydown", function (event) {
      const current = tabs.indexOf(tab);
      let next = -1;
      if (event.key === "ArrowRight") {
        next = (current + 1) % tabs.length;
      } else if (event.key === "ArrowLeft") {
        next = (current - 1 + tabs.length) % tabs.length;
      } else if (event.key === "Home") {
        next = 0;
      } else if (event.key === "End") {
        next = tabs.length - 1;
      }
      if (next < 0) {
        return;
      }
      event.preventDefault();
      selectTab(tabs[next]);
      tabs[next].focus();
    });
  });

  let wasmReady = false;

  extractButton.disabled = true;
  wasmNote.hidden = false;
  wasmNote.textContent = "The extractor is still loading.";

  loadWasm()
    .then(function () {
      wasmReady = true;
      extractButton.disabled = false;
      wasmNote.hidden = true;
    })
    .catch(function () {
      wasmReady = false;
      extractButton.disabled = true;
      wasmNote.hidden = false;
      wasmNote.textContent = "The extractor could not be loaded.";
    });

  refreshQuota();

  loadButton.addEventListener("click", function () {
    loadPage();
  });

  urlInput.addEventListener("input", function () {
    if (urlInput.value.trim() !== "") {
      setUrlError("");
    }
  });

  form.addEventListener("submit", function (event) {
    event.preventDefault();
    extract();
  });

  function loadWasm() {
    return new Promise(function (resolve, reject) {
      const ready = new Promise(function (readyResolve) {
        addEventListener("keyphrase-ready", function () {
          readyResolve();
        }, { once: true });
      });
      const script = document.createElement("script");
      script.src = "/js/lib/wasm_exec.js";
      script.onload = function () {
        const go = new Go();
        WebAssembly.instantiateStreaming(fetch("/assets/extract.wasm"), go.importObject)
          .then(function (result) {
            go.run(result.instance);
            return ready;
          })
          .then(resolve)
          .catch(function () {
            return fetch("/assets/extract.wasm")
              .then(function (response) { return response.arrayBuffer(); })
              .then(function (bytes) { return WebAssembly.instantiate(bytes, go.importObject); })
              .then(function (result) {
                go.run(result.instance);
                return ready;
              })
              .then(resolve)
              .catch(reject);
          });
      };
      script.onerror = reject;
      document.head.appendChild(script);
    });
  }

  function refreshQuota() {
    fetch("/page")
      .then(function (response) { return response.json(); })
      .then(applyQuota)
      .catch(function () {});
  }

  function applyQuota(body) {
    if (!body || body.configured === false) {
      return;
    }
    if (body.available) {
      urlInput.disabled = false;
      loadButton.disabled = false;
      quotaNote.hidden = true;
      return;
    }
    urlInput.disabled = true;
    loadButton.disabled = true;
    quotaNote.hidden = false;
    quotaNote.textContent = quotaMessage(body.resetsAt);
  }

  function quotaMessage(resetsAt) {
    if (!resetsAt) {
      return "URL loading is unavailable. Paste the article text.";
    }
    const date = new Date(resetsAt);
    const label = Number.isNaN(date.getTime())
      ? resetsAt
      : date.toLocaleDateString(undefined, { day: "numeric", month: "long", year: "numeric" });
    return "URL loading is unavailable until " + label + ". Paste the article text.";
  }

  function loadPage() {
    const url = urlInput.value.trim();
    if (url === "") {
      setUrlError("Enter a URL to load.");
      return Promise.resolve(false);
    }
    setUrlError("");
    setStatus("");
    loadButton.disabled = true;
    return fetch("/page", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ url: url }),
    })
      .then(function (response) {
        // The Cloudflare rate limiting rule answers 429 with an HTML page.
        if (response.status === 429) {
          return { status: 429, body: {} };
        }
        return response.json().then(function (body) {
          return { status: response.status, body: body };
        });
      })
      .then(function (result) {
        if (result.status === 402 || result.body.unavailable) {
          applyQuota({ configured: true, available: false, resetsAt: result.body.resetsAt });
          setStatus(quotaMessage(result.body.resetsAt));
          return false;
        }
        if (result.status === 429) {
          setStatus("URL loading is busy. Wait a moment and try again.");
          return false;
        }
        if (!result.body || result.body.error) {
          setStatus((result.body && result.body.error) || "The page could not be loaded.");
          return false;
        }
        text.value = result.body.text || "";
        if (result.body.truncated) {
          setStatus("The page was longer than 64 KiB. The text was cut to fit.");
        }
        return true;
      })
      .catch(function () {
        setStatus("The page could not be loaded.");
        return false;
      })
      .finally(function () {
        if (!urlInput.disabled) {
          loadButton.disabled = false;
        }
      });
  }

  function extract() {
    if (!wasmReady) {
      return;
    }
    const pasted = text.value;
    if (pasted.trim() !== "") {
      runExtract(pasted);
      return;
    }
    if (urlInput.disabled) {
      setStatus(quotaNote.textContent || "URL loading is unavailable. Paste the article text.");
      return;
    }
    if (urlInput.value.trim() === "") {
      setStatus("Paste article text or enter a URL.");
      return;
    }
    loadPage().then(function (ok) {
      if (ok) {
        runExtract(text.value);
      }
    });
  }

  function runExtract(source) {
    setStatus("");
    const limit = Number(limitInput.value);
    let raw;
    try {
      raw = keyphraseExtract(source, language.value, limit);
    } catch (err) {
      setStatus("The extractor could not be loaded.");
      return;
    }
    const body = JSON.parse(raw);
    if (body.error) {
      results.hidden = true;
      setStatus(body.error);
      return;
    }
    render(source, body);
  }

  function render(source, body) {
    results.hidden = false;
    keywordList.replaceChildren();
    body.keywords.forEach(function (keyword, index) {
      const item = document.createElement("li");
      const button = document.createElement("button");
      button.type = "button";
      const phrase = document.createElement("span");
      phrase.className = "phrase";
      phrase.textContent = keyword.text;
      const score = document.createElement("span");
      score.className = "mono";
      score.textContent = Number(keyword.score).toFixed(4);
      const bytes = document.createElement("span");
      bytes.className = "mono";
      bytes.textContent = keyword.start_byte + "–" + keyword.end_byte;
      button.append(phrase, score, bytes);
      button.addEventListener("mouseenter", function () {
        revealMark(index, false);
      });
      button.addEventListener("focus", function () {
        revealMark(index, false);
      });
      button.addEventListener("click", function () {
        revealMark(index, true);
      });
      item.append(button);
      keywordList.append(item);
    });
    if (body.keywords.length === 0) {
      const empty = document.createElement("li");
      empty.textContent = "No phrases.";
      keywordList.append(empty);
    }
    passage.innerHTML = highlight(source, body.keywords);
    fillCounts(body.counts);
    selectTab(document.getElementById("tab-phrases"));
  }

  function selectTab(tab) {
    tabs.forEach(function (item) {
      const selected = item === tab;
      item.setAttribute("aria-selected", selected ? "true" : "false");
      item.tabIndex = selected ? 0 : -1;
      document.getElementById(item.getAttribute("aria-controls")).hidden = !selected;
    });
  }

  function revealMark(index, open) {
    if (open) {
      selectTab(document.getElementById("tab-text"));
    }
    if (document.getElementById("panel-text").hidden) {
      return;
    }
    const mark = document.getElementById("mark-" + index);
    if (mark) {
      mark.scrollIntoView({ block: "nearest" });
    }
  }

  function fillCounts(counts) {
    countsList.replaceChildren();
    const rows = [
      ["Words", counts.words],
      ["Extracted", counts.extracted],
      ["Ranked", counts.ranked],
      ["Extracted, more than one word", counts.extracted_more_than_one_word],
      ["Ranked, more than one word", counts.ranked_more_than_one_word],
    ];
    rows.forEach(function (row) {
      const wrap = document.createElement("div");
      const term = document.createElement("dt");
      term.textContent = row[0];
      const value = document.createElement("dd");
      value.textContent = String(row[1]);
      wrap.append(term, value);
      countsList.append(wrap);
    });
    more.replaceChildren();
    (counts.more_than_one_word || []).forEach(function (phrase) {
      const item = document.createElement("li");
      item.textContent = phrase;
      more.append(item);
    });
  }

  function highlight(source, keywords) {
    const atByte = byteIndexes(source);
    const marks = keywords.map(function (keyword, index) {
      return { keyword: keyword, index: index };
    }).sort(function (a, b) {
      return a.keyword.start_byte - b.keyword.start_byte;
    });
    let html = "";
    let cursor = 0;
    marks.forEach(function (keyword, index) {
      const start = atByte.get(keyword.keyword.start_byte);
      const end = atByte.get(keyword.keyword.end_byte);
      if (start == null || end == null || start < cursor || end < start) {
        return;
      }
      html += escapeHTML(source.slice(cursor, start));
      html += '<mark id="mark-' + keyword.index + '">' + escapeHTML(source.slice(start, end)) + "</mark>";
      cursor = end;
    });
    html += escapeHTML(source.slice(cursor));
    return html;
  }

  function byteIndexes(source) {
    const encoder = new TextEncoder();
    const atByte = new Map();
    let byte = 0;
    let index = 0;
    atByte.set(0, 0);
    for (const ch of source) {
      byte += encoder.encode(ch).length;
      index += ch.length;
      atByte.set(byte, index);
    }
    return atByte;
  }

  function escapeHTML(value) {
    return value.replace(/[&<>"']/g, function (ch) {
      switch (ch) {
        case "&":
          return "&amp;";
        case "<":
          return "&lt;";
        case ">":
          return "&gt;";
        case '"':
          return "&quot;";
        default:
          return "&#39;";
      }
    });
  }

  function setUrlError(message) {
    urlError.hidden = message === "";
    urlError.textContent = message;
  }

  function setStatus(message) {
    status.hidden = message === "";
    status.textContent = message;
  }
})();
