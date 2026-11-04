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
  const filters = document.querySelector("#filters");
  const dropLoneNouns = document.querySelector("#drop-lone-nouns");
  const loneNounMinRarity = document.querySelector("#lone-noun-min-rarity");
  const dropOneOff = document.querySelector("#drop-one-off");
  const oneOffMinOthers = document.querySelector("#one-off-min-others");
  const resetFilters = document.querySelector("#reset-filters");
  let defaultSettings = null;
  const extractButton = document.querySelector("#extract");
  const extractNote = document.querySelector("#extract-note");
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

  let extractReady = false;

  extractButton.disabled = true;
  filters.disabled = true;

  loadExtractConfig()
    .then(function (body) {
      if (!body || !body.configured) {
        extractNote.hidden = false;
        extractNote.textContent = "Extraction is not configured on this host.";
        return;
      }
      extractReady = true;
      extractButton.disabled = false;
      filters.disabled = false;
      defaultSettings = body.defaults;
      applySettings(defaultSettings);
    })
    .catch(function () {
      extractNote.hidden = false;
      extractNote.textContent = "Extraction could not be reached.";
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

  dropLoneNouns.addEventListener("change", syncFilterInputs);
  dropOneOff.addEventListener("change", syncFilterInputs);
  resetFilters.addEventListener("click", function () {
    applySettings(defaultSettings);
  });

  function loadExtractConfig() {
    return fetch("/extract").then(function (response) {
      return response.json();
    });
  }

  function applySettings(settings) {
    if (!settings) {
      return;
    }
    dropLoneNouns.checked = settings.drop_lone_nouns;
    loneNounMinRarity.value = String(settings.lone_noun_min_rarity);
    dropOneOff.checked = settings.drop_one_off_phrases;
    oneOffMinOthers.value = String(settings.one_off_min_others);
    syncFilterInputs();
  }

  function syncFilterInputs() {
    loneNounMinRarity.disabled = !dropLoneNouns.checked;
    oneOffMinOthers.disabled = !dropOneOff.checked;
  }

  function readSettings() {
    const rarity = Number(loneNounMinRarity.value);
    if (loneNounMinRarity.value.trim() === "" || !(rarity >= 0 && rarity <= 1)) {
      setStatus("Rarity must be a number from 0 to 1.");
      return null;
    }
    const minOthers = Number(oneOffMinOthers.value);
    if (oneOffMinOthers.value.trim() === "" || !Number.isInteger(minOthers) || minOthers < 0) {
      setStatus("The number of other phrases must be a whole number, 0 or more.");
      return null;
    }
    return {
      drop_lone_nouns: dropLoneNouns.checked,
      lone_noun_min_rarity: rarity,
      drop_one_off_phrases: dropOneOff.checked,
      one_off_min_others: minOthers,
    };
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
    if (!extractReady) {
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
    const settings = readSettings();
    if (!settings) {
      results.hidden = true;
      return;
    }
    extractButton.disabled = true;
    fetch("/extract", {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({
        text: source,
        language: language.value,
        limit: limit,
        drop_lone_nouns: settings.drop_lone_nouns,
        lone_noun_min_rarity: settings.lone_noun_min_rarity,
        drop_one_off_phrases: settings.drop_one_off_phrases,
        one_off_min_others: settings.one_off_min_others,
      }),
    })
      .then(function (response) {
        return response.json().then(function (body) {
          return { status: response.status, body: body };
        });
      })
      .then(function (result) {
        if (result.body.error) {
          results.hidden = true;
          setStatus(result.body.error);
          return;
        }
        render(source, result.body);
      })
      .catch(function () {
        results.hidden = true;
        setStatus("Extraction failed.");
      })
      .finally(function () {
        extractButton.disabled = false;
      });
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
      ["Returned", counts.returned],
      ["Extracted, more than one word", counts.extracted_more_than_one_word],
      ["Returned, more than one word", counts.returned_more_than_one_word],
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
    marks.forEach(function (keyword) {
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
