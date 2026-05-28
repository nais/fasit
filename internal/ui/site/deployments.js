// Preview targets for new deployment
(() => {
  document.addEventListener("click", (e) => {
    const btn = e.target.closest("#preview-targets-btn");
    if (!btn) return;
    const textarea = document.getElementById("target-labels-input");
    const result = document.getElementById("preview-targets-result");
    if (!textarea || !result) return;

    const raw = textarea.value.trim();
    let labels = {};
    if (raw) {
      try {
        labels = JSON.parse(raw);
      } catch {
        result.innerHTML = '<span class="status-error">Invalid JSON</span>';
        return;
      }
    }

    result.textContent = "Loading…";
    fetch("/deployments/preview-targets", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ labels }),
    })
      .then((r) => r.json())
      .then((envs) => {
        if (!envs?.length) {
          result.innerHTML = '<span class="text-muted">No environments matched.</span>';
          return;
        }
        result.innerHTML =
          `<span class="text-muted">${envs.length} environment(s):</span> ` +
          envs
            .map((e) => `<span class="label-filter-tag">${e.tenant} / ${e.environment}</span>`)
            .join(" ");
      })
      .catch(() => {
        result.innerHTML = '<span class="status-error">Failed to load preview</span>';
      });
  });

  // Validate JSON on form submit
  document.addEventListener("submit", (e) => {
    const form = e.target.closest("form[action='/deployments']");
    if (!form) return;
    const textarea = form.querySelector("#target-labels-input");
    if (!textarea) return;
    const raw = textarea.value.trim();
    if (raw) {
      try {
        JSON.parse(raw);
      } catch {
        e.preventDefault();
        const result = document.getElementById("preview-targets-result");
        if (result) result.innerHTML = '<span class="status-error">Invalid JSON — fix before deploying</span>';
      }
    }
  });
})();

// Clear preview when popover is toggled
(() => {
  const popover = document.getElementById("new-deployment");
  popover?.addEventListener("toggle", () => {
    const result = document.getElementById("preview-targets-result");
    if (result) result.innerHTML = "";
  });
})();
