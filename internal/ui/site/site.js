document.addEventListener("click", (e) => {
  const th = e.target.closest(".sortable th");
  if (!th) return;

  const table = th.closest("table");
  const tbody = table.querySelector("tbody");
  if (!tbody) return;

  const idx = Array.from(th.parentElement.children).indexOf(th);
  const rows = Array.from(tbody.querySelectorAll("tr"));

  const asc = th.dataset.sort !== "asc";
  table.querySelectorAll("th").forEach((h) => delete h.dataset.sort);
  th.dataset.sort = asc ? "asc" : "desc";

  rows.sort((a, b) => {
    const av = (a.children[idx]?.textContent || "").trim();
    const bv = (b.children[idx]?.textContent || "").trim();
    const an = parseFloat(av);
    const bn = parseFloat(bv);
    if (!isNaN(an) && !isNaN(bn)) return asc ? an - bn : bn - an;
    return asc ? av.localeCompare(bv) : bv.localeCompare(av);
  });

  rows.forEach((r) => tbody.appendChild(r));
});
