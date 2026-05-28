// Reconciler SSE streaming.
(() => {
  const btn = document.getElementById("reconcile-btn");
  if (!btn) return;

  const badgeClass = {
    deploy: "status-badge status-success",
    unchanged: "status-badge",
    "in-progress": "status-badge status-pending",
    disabled: "status-badge status-disabled",
    unhealthy: "status-badge status-disabled",
    "missing-deps": "status-badge status-error",
    "missing-config": "status-badge status-error",
    "render-error": "status-badge status-error",
  };

  const esc = (s) => {
    const d = document.createElement("div");
    d.appendChild(document.createTextNode(s));
    return d.innerHTML;
  };

  const fmtNs = (ns) => {
    if (ns < 1_000_000) return `${(ns / 1_000).toFixed(0)}µs`;
    if (ns < 1_000_000_000) return `${(ns / 1_000_000).toFixed(0)}ms`;
    return `${(ns / 1_000_000_000).toFixed(2)}s`;
  };

  btn.addEventListener("click", () => {
    const tbody = document.getElementById("reconcile-tbody");
    const card = document.getElementById("reconcile-table-card");
    const status = document.getElementById("reconcile-status");
    const summaryEl = document.getElementById("reconcile-summary");
    if (!tbody || !card || !status) return;

    btn.disabled = true;
    btn.textContent = "Reconciling\u2026";
    tbody.innerHTML = "";
    summaryEl.innerHTML = "";
    card.style.display = "";
    status.textContent = "Streaming decisions\u2026";

    let count = 0;
    const es = new EventSource("/reconciler/stream");

    es.addEventListener("decision", (e) => {
      count++;
      const d = JSON.parse(e.data);
      const cls = badgeClass[d.action] ?? "status-badge";
      const tr = document.createElement("tr");
      tr.innerHTML =
        `<td><span class="${cls}">${esc(d.action)}</span></td>` +
        `<td>${esc(d.tenant)}</td>` +
        `<td>${esc(d.environment)}</td>` +
        `<td>${esc(d.feature)}</td>` +
        `<td>${esc(d.version)}</td>` +
        `<td>${esc(d.message)}</td>`;
      tbody.insertBefore(tr, tbody.firstChild);
      status.textContent = `${count} decisions\u2026`;
    });

    es.addEventListener("summary", (e) => {
      es.close();
      const s = JSON.parse(e.data);
      const total = s.fetchDur + s.computeDur;
      status.textContent = `${s.total} decisions in ${fmtNs(total)}`;
      summaryEl.innerHTML =
        `<div class="reconciler-summary"><div class="card"><div class="card-body">` +
        `<strong>Fetch:</strong> ${fmtNs(s.fetchDur)} ` +
        `&middot; <strong>Compute:</strong> ${fmtNs(s.computeDur)} ` +
        `&middot; <strong>Total:</strong> ${fmtNs(total)}` +
        `</div></div></div>`;
      btn.disabled = false;
      btn.textContent = "Run reconcile";

      const table = document.getElementById("reconcile-table");
      if (table) {
        table.classList.add("sortable");
        table.setAttribute("data-sort-key", "reconciler-decisions");
        window.initSortable?.(table);
      }
    });

    es.addEventListener("error", (e) => {
      es.close();
      if (e.data) {
        const { error } = JSON.parse(e.data);
        status.textContent = `Error: ${error ?? "unknown"}`;
      } else {
        status.textContent = "Connection lost";
      }
      btn.disabled = false;
      btn.textContent = "Run reconcile";
    });

    es.onerror = () => {
      es.close();
      status.textContent = "Connection lost";
      btn.disabled = false;
      btn.textContent = "Run reconcile";
    };
  });
})();
