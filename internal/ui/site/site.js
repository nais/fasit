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

// Sidebar scroll persistence
(function () {
  var sidebar = document.querySelector(".sidebar");
  if (!sidebar) return;
  var saved = sessionStorage.getItem("sidebar-scroll");
  if (saved) sidebar.scrollTop = parseInt(saved, 10);
  sidebar.addEventListener("scroll", function () {
    sessionStorage.setItem("sidebar-scroll", sidebar.scrollTop);
  });
})();

// Click on overridden row jumps to and highlights the overriding deployment header.
document.addEventListener("click", function (e) {
  if (e.target.closest("a, button")) return;
  var row = e.target.closest("tr.deployment-overridden");
  if (!row) return;
  var id = row.getAttribute("data-overridden-by");
  if (!id) return;
  var target = document.getElementById("deployment-" + id);
  if (!target) return;
  target.scrollIntoView({ behavior: "smooth", block: "center" });
  target.classList.remove("highlight");
  void target.offsetWidth;
  target.classList.add("highlight");
});

// Sidebar client-side filter (fuzzy subsequence match)
function fuzzyMatch(query, text) {
  if (!query) return true;
  var qi = 0;
  for (var i = 0; i < text.length && qi < query.length; i++) {
    if (text.charCodeAt(i) === query.charCodeAt(qi)) qi++;
  }
  return qi === query.length;
}
document.addEventListener("input", function (e) {
  var input = e.target.closest(".sidebar-filter");
  if (!input) return;
  var sidebar = input.closest(".sidebar");
  if (!sidebar) return;
  var q = input.value.trim().toLowerCase();
  sidebar.querySelectorAll(".nav li").forEach(function (li) {
    var text = li.textContent.toLowerCase();
    li.hidden = !fuzzyMatch(q, text);
  });
});

// Sidebar filter shortcut (Cmd/Ctrl+K) and Escape handling
(function () {
  var isMac = navigator.platform && navigator.platform.toUpperCase().includes("MAC");
  document.querySelectorAll(".sidebar-filter").forEach(function (input) {
    input.placeholder = isMac ? "Filter\u2026 (\u2318K)" : "Filter\u2026 (Ctrl+K)";
  });
  document.addEventListener("keydown", function (e) {
    if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
      var input = document.querySelector(".sidebar-filter");
      if (!input) return;
      e.preventDefault();
      input.focus();
      input.select();
    }
  });
  document.addEventListener("keydown", function (e) {
    if (e.key !== "Escape") return;
    var input = e.target.closest && e.target.closest(".sidebar-filter");
    if (!input) return;
    if (input.value !== "") {
      input.value = "";
      input.dispatchEvent(new Event("input", { bubbles: true }));
    }
    input.blur();
  });

  // Arrow key navigation between filter input and visible result links.
  document.addEventListener("keydown", function (e) {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    var sidebar = e.target.closest && e.target.closest(".sidebar");
    if (!sidebar) return;
    var onFilter = e.target.classList && e.target.classList.contains("sidebar-filter");
    var onLink = e.target.matches && e.target.matches(".nav li a");
    if (!onFilter && !onLink) return;
    var links = Array.from(sidebar.querySelectorAll(".nav li a")).filter(function (a) {
      return !a.closest("li").hidden;
    });
    if (!links.length) return;
    e.preventDefault();
    var idx = links.indexOf(e.target);
    var next;
    if (e.key === "ArrowDown") {
      next = idx === -1 ? 0 : Math.min(idx + 1, links.length - 1);
    } else {
      if (idx <= 0) {
        sidebar.querySelector(".sidebar-filter").focus();
        return;
      }
      next = idx - 1;
    }
    links[next].focus();
  });
})();

// Sortable tables
document.addEventListener("click", function (e) {
  var th = e.target.closest(".sortable th");
  if (!th) return;
  if (th.dataset && th.dataset.noSort !== undefined) return;

  var table = th.closest("table");
  var tbody = table.querySelector("tbody");
  if (!tbody) return;

  var idx = Array.from(th.parentElement.children).indexOf(th);
  var rows = Array.from(tbody.querySelectorAll("tr"));

  var asc = th.dataset.sort !== "asc";
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
