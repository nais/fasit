// Overview view toggle (grid ↔ table)
(() => {
  const STORAGE_KEY = "feature_overview_view";

  const syncIcon = () => {
    const btn = document.getElementById("view-toggle");
    if (!btn) return;
    const container = document.getElementById(btn.getAttribute("data-view-toggle"));
    if (!container) return;
    btn.classList.toggle("view-grid", (container.getAttribute("data-view") || "grid") === "grid");
  };

  const apply = () => {
    const container = document.getElementById("env-overview");
    if (!container) return;
    container.setAttribute("data-view", localStorage.getItem(STORAGE_KEY) || "grid");
    syncIcon();
  };

  document.addEventListener("click", (e) => {
    const btn = e.target.closest("[data-view-toggle]");
    if (!btn) return;
    const container = document.getElementById(btn.getAttribute("data-view-toggle"));
    if (!container) return;
    const next = (container.getAttribute("data-view") || "grid") === "grid" ? "table" : "grid";
    localStorage.setItem(STORAGE_KEY, next);
    container.setAttribute("data-view", next);
    syncIcon();
  });

  apply();
})();
