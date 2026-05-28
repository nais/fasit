// Copy contents of a target element to clipboard.
document.addEventListener("click", function (e) {
  var btn = e.target.closest("[data-copy-target]");
  if (!btn) return;
  var id = btn.getAttribute("data-copy-target");
  var target = document.getElementById(id);
  if (!target) return;
  var text = target.innerText;
  var done = function () {
    var prev = btn.textContent;
    btn.textContent = "Copied!";
    btn.disabled = true;
    setTimeout(function () { btn.textContent = prev; btn.disabled = false; }, 1200);
  };
  if (navigator.clipboard && navigator.clipboard.writeText) {
    navigator.clipboard.writeText(text).then(done).catch(function () {});
  } else {
    var ta = document.createElement("textarea");
    ta.value = text;
    document.body.appendChild(ta);
    ta.select();
    try { document.execCommand("copy"); done(); } catch (_) {}
    document.body.removeChild(ta);
  }
});

// Theme toggle
function toggleTheme() {
  var t = document.documentElement.dataset.theme === "light" ? "dark" : "light";
  document.documentElement.dataset.theme = t;
  localStorage.setItem("theme", t);
}

// Feature search shortcut (Cmd/Ctrl+K), local suggestions, and Escape handling.
(function () {
  var isMac = navigator.platform && navigator.platform.toUpperCase().includes("MAC");
  var featureNames = window.__featureNames || [];

  document.querySelectorAll(".feature-search-input").forEach(function (input) {
    input.placeholder = isMac ? "Search features\u2026 (\u2318K)" : "Search features\u2026 (Ctrl+K)";
  });

  function suggestionsFor(input) {
    var form = input.closest("[data-feature-search]");
    if (!form) return null;
    return form.querySelector("[data-feature-search-suggestions]");
  }

  function featureHref(name) {
    return "/features/" + encodeURIComponent(name);
  }

  function matchFeatures(query) {
    var q = query.toLowerCase();
    var exact = [];
    var prefix = [];
    var substring = [];
    featureNames.forEach(function (name) {
      var lower = name.toLowerCase();
      if (lower === q) {
        exact.push(name);
      } else if (lower.startsWith(q)) {
        prefix.push(name);
      } else if (lower.includes(q)) {
        substring.push(name);
      }
    });
    var names = exact.sort().concat(prefix.sort(), substring.sort());
    return names.slice(0, 8).map(function (name) {
      return { title: name, href: featureHref(name) };
    });
  }

  function appendHighlightedMatch(node, text, query) {
    var index = text.toLowerCase().indexOf(query.toLowerCase());
    if (index === -1) {
      node.appendChild(document.createTextNode(text));
      return;
    }

    node.appendChild(document.createTextNode(text.slice(0, index)));
    var mark = document.createElement("mark");
    mark.textContent = text.slice(index, index + query.length);
    node.appendChild(mark);
    node.appendChild(document.createTextNode(text.slice(index + query.length)));
  }

  function renderSuggestions(input, matches) {
    var suggestions = suggestionsFor(input);
    if (!suggestions) return;
    suggestions.innerHTML = "";
    if (!matches || !matches.length) {
      suggestions.classList.remove("visible");
      return;
    }
    var query = input.value.trim();
    matches.forEach(function (match) {
      var link = document.createElement("a");
      link.href = match.href;
      appendHighlightedMatch(link, match.title, query);
      suggestions.appendChild(link);
    });
    suggestions.classList.add("visible");
  }

  function updateSuggestions(input) {
    var q = input.value.trim();
    if (q.length < 1) {
      renderSuggestions(input, []);
      return;
    }
    renderSuggestions(input, matchFeatures(q));
  }

  document.addEventListener("input", function (e) {
    var input = e.target.closest && e.target.closest(".feature-search-input");
    if (!input) return;
    updateSuggestions(input);
  });

  document.addEventListener("click", function (e) {
    if (e.target.closest && e.target.closest("[data-feature-search]")) return;
    document.querySelectorAll("[data-feature-search-suggestions]").forEach(function (suggestions) {
      suggestions.classList.remove("visible");
    });
  });

  document.addEventListener("keydown", function (e) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
      var input = document.querySelector(".landing-search-input") || document.querySelector(".feature-search-input");
      if (!input) return;
      e.preventDefault();
      input.focus();
      input.select();
    }
  });
  document.addEventListener("submit", function (e) {
    var form = e.target.closest && e.target.closest("[data-feature-search]");
    if (!form) return;
    e.preventDefault();
    var first = form.querySelector("[data-feature-search-suggestions].visible a");
    if (first) {
      window.location.href = first.href;
      return;
    }
    var input = form.querySelector(".feature-search-input");
    if (!input || input.value.trim().length < 1) return;
    var matches = matchFeatures(input.value.trim());
    if (matches.length) window.location.href = matches[0].href;
  });

  document.addEventListener("keydown", function (e) {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    var input = e.target.closest && e.target.closest(".feature-search-input");
    if (!input) return;
    var suggestions = suggestionsFor(input);
    if (!suggestions || !suggestions.classList.contains("visible")) return;
    var links = suggestions.querySelectorAll("a");
    if (!links.length) return;
    e.preventDefault();
    links[e.key === "ArrowDown" ? 0 : links.length - 1].focus();
  });

  document.addEventListener("keydown", function (e) {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    var link = e.target.closest && e.target.closest("[data-feature-search-suggestions] a");
    if (!link) return;
    var suggestions = link.closest("[data-feature-search-suggestions]");
    var links = Array.from(suggestions.querySelectorAll("a"));
    var idx = links.indexOf(link);
    if (idx === -1) return;
    e.preventDefault();
    var next = e.key === "ArrowDown" ? idx + 1 : idx - 1;
    if (next < 0) {
      var input = suggestions.closest("[data-feature-search]").querySelector(".feature-search-input");
      if (input) input.focus();
      return;
    }
    links[Math.min(next, links.length - 1)].focus();
  });
  document.addEventListener("keydown", function (e) {
    if (e.key !== "Escape") return;
    var input = e.target.closest && e.target.closest(".feature-search-input");
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
function sortableTables() {
  return Array.from(document.querySelectorAll("table.sortable"));
}

function sortableStorageKey(table) {
  if (table.dataset && table.dataset.sortKey) {
    return "sort:" + table.dataset.sortKey;
  }
  var tables = sortableTables();
  var idx = tables.indexOf(table);
  if (idx < 0) return null;
  return "sort:" + location.pathname + ":" + idx;
}

function applySort(th, dir) {
  var table = th.closest("table");
  var tbody = table.querySelector("tbody");
  if (!tbody) return;
  var idx = Array.from(th.parentElement.children).indexOf(th);
  var rows = Array.from(tbody.querySelectorAll("tr"));
  var asc = dir === "asc";
  table.querySelectorAll("th").forEach(function (h) { delete h.dataset.sort; });
  th.dataset.sort = asc ? "asc" : "desc";

  function cellSortValue(row) {
    var cell = row.children[idx];
    if (!cell) return "";
    if (cell.dataset && typeof cell.dataset.sortValue === "string") {
      return cell.dataset.sortValue;
    }
    return cell.textContent.trim();
  }
  rows.sort(function (a, b) {
    var av = cellSortValue(a);
    var bv = cellSortValue(b);
    var an = parseFloat(av);
    var bn = parseFloat(bv);
    if (!isNaN(an) && !isNaN(bn) && /^-?\d+(\.\d+)?$/.test(av) && /^-?\d+(\.\d+)?$/.test(bv)) {
      return asc ? an - bn : bn - an;
    }
    return asc ? av.localeCompare(bv) : bv.localeCompare(av);
  });
  rows.forEach(function (r) { tbody.appendChild(r); });
}

document.addEventListener("click", function (e) {
  var th = e.target.closest(".sortable th");
  if (!th) return;
  if (th.dataset && th.dataset.noSort !== undefined) return;
  var dir = th.dataset.sort === "asc" ? "desc" : "asc";
  applySort(th, dir);
  var table = th.closest("table");
  var key = sortableStorageKey(table);
  if (key) {
    try {
      localStorage.setItem(key, JSON.stringify({ col: th.textContent.trim(), dir: dir }));
    } catch (_) {}
  }
});

// Replay saved sort on load.
document.addEventListener("DOMContentLoaded", function () {
  sortableTables().forEach(function (table) {
    var key = sortableStorageKey(table);
    if (!key) return;
    var raw;
    try { raw = localStorage.getItem(key); } catch (_) { return; }
    if (!raw) return;
    var state;
    try { state = JSON.parse(raw); } catch (_) { return; }
    if (!state || !state.col || !state.dir) return;
    var ths = Array.from(table.querySelectorAll("thead th"));
    var th = ths.find(function (h) {
      return !(h.dataset && h.dataset.noSort !== undefined) && h.textContent.trim() === state.col;
    });
    if (!th) return;
    applySort(th, state.dir);
  });
});

// Kebab menu toggle
document.addEventListener("click", function (e) {
  var btn = e.target.closest("[data-kebab-toggle]");
  if (btn) {
    var id = btn.getAttribute("data-kebab-toggle");
    var menu = document.getElementById(id);
    if (menu) {
      var wasOpen = menu.classList.contains("open");
      document.querySelectorAll(".kebab-menu.open").forEach(function (m) { m.classList.remove("open"); });
      if (!wasOpen) {
        menu.classList.add("open");
        dismissNavHint();
      }
    }
    e.stopPropagation();
    return;
  }
  // Close kebabs on outside click
  document.querySelectorAll(".kebab-menu.open").forEach(function (m) { m.classList.remove("open"); });
});

// Expand/collapse all <details> matching [data-expand-all="<class>"]
document.addEventListener("click", function (e) {
  var btn = e.target.closest("[data-expand-all]");
  if (!btn) return;
  var cls = btn.getAttribute("data-expand-all");
  var details = document.querySelectorAll("details." + cls);
  if (!details.length) return;
  var anyClosed = Array.from(details).some(function (d) { return !d.open; });
  details.forEach(function (d) { d.open = anyClosed; });
  btn.textContent = anyClosed ? "Collapse all" : "Expand all";
});

// JSON / RAW textarea mode toggle. Radios with [data-mode-toggle] inside
// the same form as a [data-mode-target] textarea reformat its contents.
document.addEventListener("change", function (e) {
  var radio = e.target.closest("input[data-mode-toggle][type=radio]");
  if (!radio) return;
  var form = radio.form;
  if (!form) return;
  var ta = form.querySelector("textarea[data-mode-target]");
  if (!ta) return;
  var mode = radio.value;
  var text = ta.value;
  try {
    var parsed = JSON.parse(text);
    ta.value = mode === "json" ? JSON.stringify(parsed, null, 2) : JSON.stringify(parsed);
  } catch (_) {
    // Leave content untouched if it isn't valid JSON.
  }
});

// Table text filter: [data-filter-table="<id>"] filters rows of table#<id>
document.addEventListener("input", function (e) {
  var input = e.target.closest("[data-filter-table]");
  if (!input) return;
  var tableId = input.getAttribute("data-filter-table");
  var table = document.getElementById(tableId);
  if (!table) return;
  var query = input.value.toLowerCase().trim();
  var rows = table.querySelectorAll("tbody tr");
  rows.forEach(function (row) {
    var text = row.textContent.toLowerCase();
    row.style.display = !query || text.indexOf(query) !== -1 ? "" : "none";
  });
});

// URL-controlled filter: [data-url-filter="<param>"] updates ?param= on input
// Filters client-side immediately, then navigates on Enter for shareable URL.
(function () {
  document.addEventListener("input", function (e) {
    var input = e.target.closest("[data-url-filter]");
    if (!input) return;
    // Client-side filter immediately
    var table = input.closest(".card-body");
    if (!table) return;
    var tbody = table.querySelector("table tbody");
    if (!tbody) return;
    var query = input.value.toLowerCase().trim();
    var terms = query.split(/\s+/).filter(function (t) { return t; });
    var rows = tbody.querySelectorAll("tr");
    rows.forEach(function (row) {
      var text = row.textContent.toLowerCase().replace(/:\s+/g, ":");
      var match = !terms.length || terms.every(function (t) { return text.indexOf(t) !== -1; });
      row.style.display = match ? "" : "none";
    });
    // Update URL without navigation
    var param = input.getAttribute("data-url-filter");
    var url = new URL(window.location);
    if (query) {
      url.searchParams.set(param, query);
    } else {
      url.searchParams.delete(param);
    }
    history.replaceState(null, "", url);
  });

  // Submit on Enter for full server round-trip
  document.addEventListener("keydown", function (e) {
    if (e.key !== "Enter") return;
    var input = e.target.closest("[data-url-filter]");
    if (!input) return;
    e.preventDefault();
    var param = input.getAttribute("data-url-filter");
    var url = new URL(window.location);
    var val = input.value.trim();
    if (val) {
      url.searchParams.set(param, val);
    } else {
      url.searchParams.delete(param);
    }
    window.location = url;
  });
})();

// Preview targets for new deployment
(function () {
  document.addEventListener("click", function (e) {
    var btn = e.target.closest("#preview-targets-btn");
    if (!btn) return;
    var textarea = document.getElementById("target-labels-input");
    var result = document.getElementById("preview-targets-result");
    if (!textarea || !result) return;

    var raw = textarea.value.trim();
    var labels = {};
    if (raw) {
      try {
        labels = JSON.parse(raw);
      } catch (err) {
        result.innerHTML = '<span class="status-error">Invalid JSON</span>';
        return;
      }
    }

    result.textContent = "Loading…";
    fetch("/deployments/preview-targets", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ labels: labels }),
    })
      .then(function (r) { return r.json(); })
      .then(function (envs) {
        if (!envs || envs.length === 0) {
          result.innerHTML = '<span class="text-muted">No environments matched.</span>';
          return;
        }
        var html = '<span class="text-muted">' + envs.length + ' environment(s):</span> ';
        html += envs.map(function (e) {
          return '<span class="label-filter-tag">' + e.tenant + ' / ' + e.environment + '</span>';
        }).join(" ");
        result.innerHTML = html;
      })
      .catch(function () {
        result.innerHTML = '<span class="status-error">Failed to load preview</span>';
      });
  });

  // Validate JSON on form submit
  document.addEventListener("submit", function (e) {
    var form = e.target.closest("form[action='/deployments']");
    if (!form) return;
    var textarea = form.querySelector("#target-labels-input");
    if (!textarea) return;
    var raw = textarea.value.trim();
    if (raw) {
      try {
        JSON.parse(raw);
      } catch (err) {
        e.preventDefault();
        var result = document.getElementById("preview-targets-result");
        if (result) result.innerHTML = '<span class="status-error">Invalid JSON — fix before deploying</span>';
      }
    }
  });
})();

// Clear preview when popover is toggled
(function () {
  var popover = document.getElementById("new-deployment");
  if (popover) {
    popover.addEventListener("toggle", function () {
      var result = document.getElementById("preview-targets-result");
      if (result) result.innerHTML = "";
    });
  }
})();

// Nav hotdog hint
function dismissNavHint() {
  var hint = document.getElementById("nav-hotdog-hint");
  if (hint) hint.classList.remove("visible");
  var btn = document.querySelector(".nav-hotdog");
  if (btn) btn.classList.remove("highlighted");
  localStorage.setItem("nav-hotdog-seen", "1");
}
(function () {
  if (localStorage.getItem("nav-hotdog-seen")) return;
  var hint = document.getElementById("nav-hotdog-hint");
  var btn = document.querySelector(".nav-hotdog");
  if (hint) hint.classList.add("visible");
  if (btn) btn.classList.add("highlighted");
})();

// Overview view toggle (grid ↔ table)
(function () {
  var STORAGE_KEY = "feature_overview_view";

  function syncIcon() {
    var btn = document.getElementById("view-toggle");
    if (!btn) return;
    var container = document.getElementById(btn.getAttribute("data-view-toggle"));
    if (!container) return;
    var view = container.getAttribute("data-view") || "grid";
    btn.classList.toggle("view-grid", view === "grid");
  }

  function apply() {
    var container = document.getElementById("env-overview");
    if (!container) return;
    var view = localStorage.getItem(STORAGE_KEY) || "grid";
    container.setAttribute("data-view", view);
    syncIcon();
  }

  document.addEventListener("click", function (e) {
    var btn = e.target.closest("[data-view-toggle]");
    if (!btn) return;
    var container = document.getElementById(btn.getAttribute("data-view-toggle"));
    if (!container) return;
    var current = container.getAttribute("data-view") || "grid";
    var next = current === "grid" ? "table" : "grid";
    localStorage.setItem(STORAGE_KEY, next);
    container.setAttribute("data-view", next);
    syncIcon();
  });

  apply();
})();
