// Copy contents of a target element to clipboard.
document.addEventListener("click", (e) => {
  const btn = e.target.closest("[data-copy-target]");
  if (!btn) return;
  const target = document.getElementById(btn.getAttribute("data-copy-target"));
  if (!target) return;
  const text = target.innerText;
  const done = () => {
    const prev = btn.textContent;
    btn.textContent = "Copied!";
    btn.disabled = true;
    setTimeout(() => { btn.textContent = prev; btn.disabled = false; }, 1200);
  };
  if (navigator.clipboard?.writeText) {
    navigator.clipboard.writeText(text).then(done).catch(() => {});
  } else {
    const ta = document.createElement("textarea");
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    try { document.execCommand("copy"); done(); } catch {}
    document.body.removeChild(ta);
  }
});

// Theme toggle
function toggleTheme() {
  const t = document.documentElement.dataset.theme === "light" ? "dark" : "light";
  document.documentElement.dataset.theme = t;
  localStorage.setItem("theme", t);
}

// Feature search shortcut (Cmd/Ctrl+K), local suggestions, and Escape handling.
(() => {
  const isMac = navigator.platform?.toUpperCase().includes("MAC");
  const featureNames = window.__featureNames || [];

  for (const input of document.querySelectorAll(".feature-search-input")) {
    input.placeholder = isMac ? "Search features\u2026 (\u2318K)" : "Search features\u2026 (Ctrl+K)";
  }

  const suggestionsFor = (input) => {
    const form = input.closest("[data-feature-search]");
    return form?.querySelector("[data-feature-search-suggestions]") ?? null;
  };

  const featureHref = (name) => `/features/${encodeURIComponent(name)}`;

  const fuzzyMatch = (query, text) => {
    let qi = 0;
    const indices = [];
    for (let i = 0; i < text.length && qi < query.length; i++) {
      if (text.charCodeAt(i) === query.charCodeAt(qi)) {
        indices.push(i);
        qi++;
      }
    }
    if (qi < query.length) return null;
    let score = 0;
    for (let j = 1; j < indices.length; j++) {
      score += indices[j] - indices[j - 1] - 1;
    }
    return { indices, score };
  };

  const matchFeatures = (query) => {
    const q = query.toLowerCase();
    const exact = [], prefix = [], substring = [], fuzzy = [];
    for (const name of featureNames) {
      const lower = name.toLowerCase();
      if (lower === q) exact.push(name);
      else if (lower.startsWith(q)) prefix.push(name);
      else if (lower.includes(q)) substring.push(name);
      else {
        const m = fuzzyMatch(q, lower);
        if (m) fuzzy.push({ name, score: m.score });
      }
    }
    fuzzy.sort((a, b) => a.score - b.score || a.name.localeCompare(b.name));
    return [...exact.sort(), ...prefix.sort(), ...substring.sort(), ...fuzzy.map((f) => f.name)]
      .slice(0, 8)
      .map((name) => ({ title: name, href: featureHref(name) }));
  };

  const appendHighlightedMatch = (node, text, query) => {
    const index = text.toLowerCase().indexOf(query.toLowerCase());
    if (index !== -1) {
      node.appendChild(document.createTextNode(text.slice(0, index)));
      const mark = document.createElement("mark");
      mark.textContent = text.slice(index, index + query.length);
      node.appendChild(mark);
      node.appendChild(document.createTextNode(text.slice(index + query.length)));
      return;
    }
    const m = fuzzyMatch(query.toLowerCase(), text.toLowerCase());
    if (!m) {
      node.appendChild(document.createTextNode(text));
      return;
    }
    let last = 0;
    for (const idx of m.indices) {
      if (idx > last) node.appendChild(document.createTextNode(text.slice(last, idx)));
      const mark = document.createElement("mark");
      mark.textContent = text.charAt(idx);
      node.appendChild(mark);
      last = idx + 1;
    }
    if (last < text.length) node.appendChild(document.createTextNode(text.slice(last)));
  };

  const renderSuggestions = (input, matches) => {
    const suggestions = suggestionsFor(input);
    if (!suggestions) return;
    suggestions.innerHTML = "";
    if (!matches?.length) {
      suggestions.classList.remove("visible");
      return;
    }
    const query = input.value.trim();
    for (const match of matches) {
      const link = document.createElement("a");
      link.href = match.href;
      appendHighlightedMatch(link, match.title, query);
      suggestions.appendChild(link);
    }
    suggestions.classList.add("visible");
  };

  const updateSuggestions = (input) => {
    const q = input.value.trim();
    if (q.length < 1) { renderSuggestions(input, []); return; }
    renderSuggestions(input, matchFeatures(q));
  };

  document.addEventListener("input", (e) => {
    const input = e.target.closest?.(".feature-search-input");
    if (input) updateSuggestions(input);
  });

  document.addEventListener("click", (e) => {
    if (e.target.closest?.("[data-feature-search]")) return;
    for (const s of document.querySelectorAll("[data-feature-search-suggestions]")) {
      s.classList.remove("visible");
    }
  });

  document.addEventListener("keydown", (e) => {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
      const input = document.querySelector(".landing-search-input") ?? document.querySelector(".feature-search-input");
      if (!input) return;
      e.preventDefault();
      input.focus();
      input.select();
    }
  });

  document.addEventListener("submit", (e) => {
    const form = e.target.closest?.("[data-feature-search]");
    if (!form) return;
    e.preventDefault();
    const first = form.querySelector("[data-feature-search-suggestions].visible a");
    if (first) { window.location.href = first.href; return; }
    const input = form.querySelector(".feature-search-input");
    if (!input || input.value.trim().length < 1) return;
    const matches = matchFeatures(input.value.trim());
    if (matches.length) window.location.href = matches[0].href;
  });

  document.addEventListener("keydown", (e) => {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    const input = e.target.closest?.(".feature-search-input");
    if (!input) return;
    const suggestions = suggestionsFor(input);
    if (!suggestions?.classList.contains("visible")) return;
    const links = suggestions.querySelectorAll("a");
    if (!links.length) return;
    e.preventDefault();
    links[e.key === "ArrowDown" ? 0 : links.length - 1].focus();
  });

  document.addEventListener("keydown", (e) => {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    const link = e.target.closest?.("[data-feature-search-suggestions] a");
    if (!link) return;
    const suggestions = link.closest("[data-feature-search-suggestions]");
    const links = [...suggestions.querySelectorAll("a")];
    const idx = links.indexOf(link);
    if (idx === -1) return;
    e.preventDefault();
    const next = e.key === "ArrowDown" ? idx + 1 : idx - 1;
    if (next < 0) {
      suggestions.closest("[data-feature-search]")?.querySelector(".feature-search-input")?.focus();
      return;
    }
    links[Math.min(next, links.length - 1)].focus();
  });

  document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape") return;
    const input = e.target.closest?.(".feature-search-input");
    if (!input) return;
    if (input.value !== "") {
      input.value = "";
      input.dispatchEvent(new Event("input", { bubbles: true }));
    }
    renderSuggestions(input, []);
    input.blur();
  });
})();

// Sortable tables. Sort state is persisted in localStorage, keyed by
// location.pathname + table index, identifying the column by header text
// so it survives column reorderings.
const sortableTables = () => [...document.querySelectorAll("table.sortable")];

const sortableStorageKey = (table) => {
  if (table.dataset?.sortKey) return `sort:${table.dataset.sortKey}`;
  const tables = sortableTables();
  const idx = tables.indexOf(table);
  return idx < 0 ? null : `sort:${location.pathname}:${idx}`;
};

function applySort(th, dir) {
  const table = th.closest("table");
  const tbody = table.querySelector("tbody");
  if (!tbody) return;
  const idx = [...th.parentElement.children].indexOf(th);
  const rows = [...tbody.querySelectorAll("tr")];
  const asc = dir === "asc";
  for (const h of table.querySelectorAll("th")) { delete h.dataset.sort; }
  th.dataset.sort = asc ? "asc" : "desc";

  const cellSortValue = (row) => {
    const cell = row.children[idx];
    if (!cell) return "";
    return typeof cell.dataset?.sortValue === "string" ? cell.dataset.sortValue : cell.textContent.trim();
  };

  rows.sort((a, b) => {
    const av = cellSortValue(a);
    const bv = cellSortValue(b);
    const an = parseFloat(av);
    const bn = parseFloat(bv);
    if (!isNaN(an) && !isNaN(bn) && /^-?\d+(\.\d+)?$/.test(av) && /^-?\d+(\.\d+)?$/.test(bv)) {
      return asc ? an - bn : bn - an;
    }
    return asc ? av.localeCompare(bv) : bv.localeCompare(av);
  });
  for (const r of rows) { tbody.appendChild(r); }
}

function replaySavedSort(table) {
  const key = sortableStorageKey(table);
  if (!key) return;
  let raw;
  try { raw = localStorage.getItem(key); } catch { return; }
  if (!raw) return;
  let state;
  try { state = JSON.parse(raw); } catch { return; }
  if (!state?.col || !state?.dir) return;
  const th = [...table.querySelectorAll("thead th")].find(
    (h) => h.dataset?.noSort === undefined && h.textContent.trim() === state.col
  );
  if (th) applySort(th, state.dir);
}
window.initSortable = replaySavedSort;

document.addEventListener("click", (e) => {
  const th = e.target.closest(".sortable th");
  if (!th || th.dataset?.noSort !== undefined) return;
  const dir = th.dataset.sort === "asc" ? "desc" : "asc";
  applySort(th, dir);
  const key = sortableStorageKey(th.closest("table"));
  if (key) {
    try { localStorage.setItem(key, JSON.stringify({ col: th.textContent.trim(), dir })); } catch {}
  }
});

// Replay saved sort on load.
document.addEventListener("DOMContentLoaded", () => {
  sortableTables().forEach(replaySavedSort);
});

// Kebab menu toggle
document.addEventListener("click", (e) => {
  const btn = e.target.closest("[data-kebab-toggle]");
  if (btn) {
    const menu = document.getElementById(btn.getAttribute("data-kebab-toggle"));
    if (menu) {
      const wasOpen = menu.classList.contains("open");
      for (const m of document.querySelectorAll(".kebab-menu.open")) { m.classList.remove("open"); }
      if (!wasOpen) {
        menu.classList.add("open");
        dismissNavHint();
      }
    }
    e.stopPropagation();
    return;
  }
  for (const m of document.querySelectorAll(".kebab-menu.open")) { m.classList.remove("open"); }
});

// Expand/collapse all <details> matching [data-expand-all="<class>"]
document.addEventListener("click", (e) => {
  const btn = e.target.closest("[data-expand-all]");
  if (!btn) return;
  const details = document.querySelectorAll(`details.${btn.getAttribute("data-expand-all")}`);
  if (!details.length) return;
  const anyClosed = [...details].some((d) => !d.open);
  for (const d of details) { d.open = anyClosed; }
  btn.textContent = anyClosed ? "Collapse all" : "Expand all";
});

// JSON / RAW textarea mode toggle. Radios with [data-mode-toggle] inside
// the same form as a [data-mode-target] textarea reformat its contents.
document.addEventListener("change", (e) => {
  const radio = e.target.closest("input[data-mode-toggle][type=radio]");
  if (!radio) return;
  const ta = radio.form?.querySelector("textarea[data-mode-target]");
  if (!ta) return;
  try {
    const parsed = JSON.parse(ta.value);
    ta.value = radio.value === "json" ? JSON.stringify(parsed, null, 2) : JSON.stringify(parsed);
  } catch {}
});

// Table text filter: [data-filter-table="<id>"] filters rows of table#<id>
document.addEventListener("input", (e) => {
  const input = e.target.closest("[data-filter-table]");
  if (!input) return;
  const table = document.getElementById(input.getAttribute("data-filter-table"));
  if (!table) return;
  const query = input.value.toLowerCase().trim();
  for (const row of table.querySelectorAll("tbody tr")) {
    row.style.display = !query || row.textContent.toLowerCase().includes(query) ? "" : "none";
  }
});

// URL-controlled filter: [data-url-filter="<param>"] updates ?param= on input
// Filters client-side immediately, then navigates on Enter for shareable URL.
(() => {
  document.addEventListener("input", (e) => {
    const input = e.target.closest("[data-url-filter]");
    if (!input) return;
    const cardBody = input.closest(".card-body");
    const tbody = cardBody?.querySelector("table tbody");
    if (!tbody) return;
    const query = input.value.toLowerCase().trim();
    const terms = query.split(/\s+/).filter(Boolean);
    for (const row of tbody.querySelectorAll("tr")) {
      const text = row.textContent.toLowerCase().replace(/:\s+/g, ":");
      row.style.display = !terms.length || terms.every((t) => text.includes(t)) ? "" : "none";
    }
    const param = input.getAttribute("data-url-filter");
    const url = new URL(window.location);
    query ? url.searchParams.set(param, query) : url.searchParams.delete(param);
    history.replaceState(null, "", url);
  });

  document.addEventListener("keydown", (e) => {
    if (e.key !== "Enter") return;
    const input = e.target.closest("[data-url-filter]");
    if (!input) return;
    e.preventDefault();
    const param = input.getAttribute("data-url-filter");
    const url = new URL(window.location);
    const val = input.value.trim();
    val ? url.searchParams.set(param, val) : url.searchParams.delete(param);
    window.location = url;
  });
})();

// Nav hotdog hint
function dismissNavHint() {
  document.getElementById("nav-hotdog-hint")?.classList.remove("visible");
  document.querySelector(".nav-hotdog")?.classList.remove("highlighted");
  localStorage.setItem("nav-hotdog-seen", "1");
}
(() => {
  if (localStorage.getItem("nav-hotdog-seen")) return;
  document.getElementById("nav-hotdog-hint")?.classList.add("visible");
  document.querySelector(".nav-hotdog")?.classList.add("highlighted");
})();

// Lazy modal: [data-lazy-modal="<url>"] fetches HTML and shows it in a modal.
(() => {
  const modal = document.createElement("div");
  modal.className = "lazy-modal-backdrop";
  modal.innerHTML = '<div class="lazy-modal"><button class="lazy-modal-close" type="button">&times;</button><div class="lazy-modal-content"></div></div>';
  document.body.appendChild(modal);

  const content = modal.querySelector(".lazy-modal-content");
  const closeBtn = modal.querySelector(".lazy-modal-close");

  function close() { modal.classList.remove("open"); }
  closeBtn.addEventListener("click", close);
  modal.addEventListener("click", (e) => { if (e.target === modal) close(); });
  document.addEventListener("keydown", (e) => { if (e.key === "Escape" && modal.classList.contains("open")) close(); });

  document.addEventListener("click", (e) => {
    const btn = e.target.closest("[data-lazy-modal]");
    if (!btn) return;
    e.preventDefault();
    const url = btn.getAttribute("data-lazy-modal");
    content.innerHTML = '<p class="text-muted">Loading…</p>';
    modal.classList.add("open");
    modal.dataset.url = url;
    fetch(url)
      .then((r) => { if (!r.ok) throw new Error(r.statusText); return r.text(); })
      .then((html) => { content.innerHTML = html; })
      .catch((err) => { content.innerHTML = '<p class="text-error">Failed to load: ' + err.message + '</p>'; });
  });

  // Shift+Enter submits the template tester form
  document.addEventListener("keydown", (e) => {
    if (e.shiftKey && e.key === "Enter" && e.target.id === "template-input") {
      e.preventDefault();
      e.target.closest("form").requestSubmit();
    }
  });

  // Keep URL in sync with template tester form state
  const envSelect = document.getElementById("env-select");
  const templateInput = document.getElementById("template-input");
  if (envSelect && templateInput) {
    const syncURL = () => {
      const url = new URL(window.location);
      const env = envSelect.value;
      if (env) { url.searchParams.set("env", env); } else { url.searchParams.delete("env"); }
      const tpl = templateInput.value;
      if (tpl) { url.searchParams.set("template", tpl); } else { url.searchParams.delete("template"); }
      history.replaceState(null, "", url);
    };
    envSelect.addEventListener("change", syncURL);
    templateInput.addEventListener("input", syncURL);
  }
})();
