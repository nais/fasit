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

// Sortable tables
document.addEventListener("click", function (e) {
  var th = e.target.closest(".sortable th");
  if (!th) return;

  var table = th.closest("table");
  var tbody = table.querySelector("tbody");
  if (!tbody) return;

  var idx = Array.from(th.parentElement.children).indexOf(th);
  var rows = Array.from(tbody.querySelectorAll("tr"));

  var asc = th.dataset.sort !== "asc";
  table.querySelectorAll("th").forEach(function (h) { delete h.dataset.sort; });
  th.dataset.sort = asc ? "asc" : "desc";

  rows.sort(function (a, b) {
    var av = (a.children[idx] ? a.children[idx].textContent : "").trim();
    var bv = (b.children[idx] ? b.children[idx].textContent : "").trim();
    var an = parseFloat(av);
    var bn = parseFloat(bv);
    if (!isNaN(an) && !isNaN(bn)) return asc ? an - bn : bn - an;
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
