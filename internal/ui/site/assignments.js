(() => {
  const previewTimers = new WeakMap();
  const previewControllers = new WeakMap();

  function renderMessage(result, message, className) {
    const span = document.createElement("span");
    span.className = className;
    span.textContent = message;
    result.replaceChildren(span);
  }

  function targetLabels(form, result) {
    const textarea = form.querySelector("#target-labels-input");
    if (textarea) {
      const raw = textarea.value.trim();
      if (!raw) return {};
      try {
        return JSON.parse(raw);
      } catch {
        renderMessage(result, "Invalid JSON", "status-error");
        return null;
      }
    }

    const labels = {};
    for (const raw of new FormData(form).getAll("target_label")) {
      if (!raw) continue;
      const separator = raw.indexOf("=");
      if (separator > 0) labels[raw.slice(0, separator)] = raw.slice(separator + 1);
    }
    return labels;
  }

  function cancelPreview(form) {
    clearTimeout(previewTimers.get(form));
    previewTimers.delete(form);
    previewControllers.get(form)?.abort();
    previewControllers.delete(form);
    const result = form.querySelector("#preview-targets-result");
    result?.classList.remove("is-loading");
    result?.removeAttribute("aria-busy");
  }

  function previewTargets(form) {
    const result = form.querySelector("#preview-targets-result");
    if (!result) return;
    cancelPreview(form);
    const labels = targetLabels(form, result);
    if (labels === null) return;

    previewControllers.get(form)?.abort();
    const controller = new AbortController();
    previewControllers.set(form, controller);
    const kinds = new FormData(form).getAll("environment_kind");
    if (!result.hasChildNodes()) renderMessage(result, "Loading matching environments…", "text-muted");
    result.classList.add("is-loading");
    result.setAttribute("aria-busy", "true");
    fetch("/assignments/preview-targets", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ labels, kinds }),
      signal: controller.signal,
    })
      .then((response) => {
        if (!response.ok) throw new Error("preview failed");
        return response.json();
      })
      .then((envs) => {
        if (!envs?.length) {
          renderMessage(result, "No environments match", "text-muted");
          return;
        }

        const preview = document.createElement("div");
        preview.className = "assignment-preview-pills";
        if (Object.keys(labels).length === 0) {
          const all = document.createElement("span");
          all.className = "label-filter-tag";
          all.textContent = "All environments";
          preview.append(all);
        } else {
          for (const env of envs) {
            const pill = document.createElement("span");
            pill.className = "label-filter-tag";
            pill.textContent = `${env.tenant} / ${env.environment}`;
            preview.append(pill);
          }
        }
        result.replaceChildren(preview);
      })
      .catch((error) => {
        if (error.name !== "AbortError") renderMessage(result, "Failed to load preview", "status-error");
      })
      .finally(() => {
        if (previewControllers.get(form) === controller) {
          previewControllers.delete(form);
          result.classList.remove("is-loading");
          result.removeAttribute("aria-busy");
        }
      });
  }

  function schedulePreview(form) {
    cancelPreview(form);
    previewTimers.set(form, setTimeout(() => previewTargets(form), 200));
  }

  document.addEventListener("click", (event) => {
    const button = event.target.closest("#preview-targets-btn");
    const form = button?.closest("form");
    if (form) previewTargets(form);
  });

  document.addEventListener("change", (event) => {
    if (event.target.matches("[name='target_label']")) {
      const form = event.target.closest("form[data-assignment-form]");
      if (form) schedulePreview(form);
    }
  });

  document.addEventListener("assignment-target-change", (event) => {
    const form = event.target.closest("form[data-assignment-form]");
    if (form) schedulePreview(form);
  });

  document.addEventListener("submit", (event) => {
    const form = event.target.closest("form[action='/assignments']");
    if (!form) return;
    const textarea = form.querySelector("#target-labels-input");
    if (!textarea) return;
    const raw = textarea.value.trim();
    if (raw) {
      try {
        JSON.parse(raw);
      } catch {
        event.preventDefault();
        const result = form.querySelector("#preview-targets-result");
        if (result) renderMessage(result, "Invalid JSON — fix before deploying", "status-error");
      }
    }
  });

  for (const popover of document.querySelectorAll("#new-assignment, #new-feature-assignment")) {
    popover.addEventListener("toggle", () => {
      const result = popover.querySelector("#preview-targets-result");
      const form = popover.querySelector("form[data-assignment-form]");
      if (!result) return;
      result.replaceChildren();
      if (form) cancelPreview(form);
      if (popover.id === "new-feature-assignment" && popover.matches(":popover-open") && form) {
        schedulePreview(form);
      }
    });
  }
})();

(() => {
  for (const select of document.querySelectorAll("[data-version-select]")) {
    const form = select.closest("form");
    const customGroup = form?.querySelector("[data-custom-version]");
    const customInput = customGroup?.querySelector("input");
    const useListButton = customGroup?.querySelector("[data-use-version-list]");
    if (!customGroup || !customInput || !useListButton) continue;

    function syncCustomVersion(focus) {
      const custom = select.value === "__custom__";
      select.hidden = custom;
      customGroup.hidden = !custom;
      customInput.required = custom;
      if (custom && focus) customInput.focus();
    }

    select.addEventListener("change", () => syncCustomVersion(true));
    useListButton.addEventListener("click", () => {
      select.value = "";
      syncCustomVersion(false);
      select.focus();
    });
    syncCustomVersion(false);
  }
})();

(() => {
  for (const builder of document.querySelectorAll("[data-label-builder]")) {
    const chips = builder.querySelector("[data-label-rows]");
    const addButton = builder.querySelector("[data-add-label]");
    const options = [...builder.querySelectorAll("[data-label-options] > [data-label-key]")].map((node) => ({
      key: node.getAttribute("data-label-key"),
      values: [...node.querySelectorAll("[data-label-value]")].map((value) => value.textContent),
    }));
    if (!chips || !addButton) continue;

    function notifyTargetChange() {
      builder.dispatchEvent(new Event("assignment-target-change", { bubbles: true }));
    }

    function selectedKeys() {
      return new Set([...chips.querySelectorAll("[data-label-chip]")].map((chip) => chip.getAttribute("data-label-key")));
    }

    function updateAddButton() {
      addButton.disabled = options.length === 0 || selectedKeys().size >= options.length;
    }

    function addChip(key, value) {
      const chip = document.createElement("span");
      chip.className = "assignment-label-chip";
      chip.setAttribute("data-label-chip", "");
      chip.setAttribute("data-label-key", key);

      const text = document.createElement("span");
      text.textContent = `${key}: ${value}`;
      const input = document.createElement("input");
      input.type = "hidden";
      input.name = "target_label";
      input.value = `${key}=${value}`;
      const remove = document.createElement("button");
      remove.type = "button";
      remove.setAttribute("aria-label", `Remove ${key} label`);
      remove.textContent = "×";
      remove.addEventListener("click", () => {
        builder.querySelector("[data-label-editor]")?.remove();
        addButton.hidden = false;
        chip.remove();
        updateAddButton();
        notifyTargetChange();
        addButton.focus();
      });

      chip.append(text, input, remove);
      addButton.before(chip);
      updateAddButton();
      notifyTargetChange();
    }

    function showEditor() {
      if (builder.querySelector("[data-label-editor]")) return;
      const editor = document.createElement("div");
      editor.className = "assignment-label-editor";
      editor.setAttribute("data-label-editor", "");

      const keySelect = document.createElement("select");
      keySelect.setAttribute("aria-label", "Label key");
      keySelect.required = true;
      const keyPlaceholder = document.createElement("option");
      keyPlaceholder.value = "";
      keyPlaceholder.textContent = "Choose label…";
      keyPlaceholder.disabled = true;
      keyPlaceholder.selected = true;
      keySelect.append(keyPlaceholder);
      const used = selectedKeys();
      for (const option of options) {
        if (used.has(option.key)) continue;
        const keyOption = document.createElement("option");
        keyOption.value = option.key;
        keyOption.textContent = option.key;
        keySelect.append(keyOption);
      }

      const valueSelect = document.createElement("select");
      valueSelect.setAttribute("aria-label", "Label value");
      valueSelect.required = true;
      valueSelect.disabled = true;
      const valuePlaceholder = document.createElement("option");
      valuePlaceholder.value = "";
      valuePlaceholder.textContent = "Choose value…";
      valuePlaceholder.disabled = true;
      valuePlaceholder.selected = true;
      valueSelect.append(valuePlaceholder);

      const cancel = document.createElement("button");
      cancel.type = "button";
      cancel.className = "assignment-label-cancel";
      cancel.setAttribute("aria-label", "Cancel adding label");
      cancel.textContent = "×";

      keySelect.addEventListener("change", () => {
        const selected = options.find((option) => option.key === keySelect.value);
        valueSelect.replaceChildren();
        const placeholder = document.createElement("option");
        placeholder.value = "";
        placeholder.textContent = "Choose value…";
        placeholder.disabled = true;
        placeholder.selected = true;
        valueSelect.append(placeholder);
        for (const value of selected?.values ?? []) {
          const valueOption = document.createElement("option");
          valueOption.value = value;
          valueOption.textContent = value;
          valueSelect.append(valueOption);
        }
        valueSelect.disabled = false;
        valueSelect.focus();
      });
      valueSelect.addEventListener("change", () => {
        addChip(keySelect.value, valueSelect.value);
        editor.remove();
        addButton.hidden = false;
        addButton.focus();
      });
      cancel.addEventListener("click", () => {
        editor.remove();
        addButton.hidden = false;
        addButton.focus();
      });

      editor.append(keySelect, valueSelect, cancel);
      chips.after(editor);
      addButton.hidden = true;
      keySelect.focus();
    }

    addButton.addEventListener("click", showEditor);
    updateAddButton();
  }
})();
