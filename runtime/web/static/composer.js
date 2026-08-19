(function () {
  const input = document.getElementById("composer-message");
  const catalogPanel = document.getElementById("composer-catalog-panel");
  const description = document.getElementById("composer-description");
  const preview = document.getElementById("composer-preview");
  const filePanel = document.getElementById("composer-file-panel");
  const fileOpen = document.getElementById("composer-file-open");
  const fileDialog = document.getElementById("composer-file-dialog");
  const fileClose = document.getElementById("composer-file-close");
  const attachMenu = document.getElementById("composer-attach-menu");
  const hostOpen = document.getElementById("composer-host-open");
  const hostMenu = document.getElementById("composer-host-menu");
  const hostInput = document.getElementById("composer-host");
  const hostLabel = document.getElementById("composer-host-label");
  if (!input) return;

  let catalogTrigger = null;
  let catalogActive = -1;
  let debounceTimer;
  let catalogAbort = null;

  function escapeHtml(text) {
    return text
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function syncPreviewScroll() {
    if (!preview || !input) return;
    preview.scrollLeft = input.scrollLeft;
    preview.scrollTop = input.scrollTop;
  }

  function highlightPreview(value) {
    if (!preview) return;
    let html = escapeHtml(value);
    html = html.replace(/(^|\s)(\/[a-z0-9-]+)/gi, '$1<mark class="composer-ref">$2</mark>');
    html = html.replace(/(^|\s)(@[a-z0-9./_-]+)/gi, '$1<mark class="composer-ref">$2</mark>');
    preview.innerHTML = html;
    syncPreviewScroll();
  }

  function triggerAtCursor() {
    const val = input.value;
    const pos = input.selectionStart == null ? val.length : input.selectionStart;
    const before = val.slice(0, pos);
    const slash = before.lastIndexOf("/");
    const at = before.lastIndexOf("@");
    if (slash > at && slash >= 0 && (slash === 0 || /\s/.test(before[slash - 1]))) {
      return { kind: "commands", q: before.slice(slash + 1), start: slash };
    }
    if (at >= 0 && (at === 0 || /\s/.test(before[at - 1]))) {
      const fragment = before.slice(at + 1);
      if (fragment.includes(" ")) {
        return null;
      }
      if (fragment.endsWith("/")) {
        return { kind: "files", q: fragment.replace(/\/$/, ""), start: at };
      }
      if (fragment.includes("/")) {
        return null;
      }
      return { kind: "skills", q: fragment, start: at };
    }
    return null;
  }

  function catalogItems() {
    if (!catalogPanel) return [];
    return Array.from(catalogPanel.querySelectorAll("[data-insert]"));
  }

  function setDescription(text) {
    if (!description) return;
    const copy = (text || "").trim();
    description.textContent = copy;
    description.hidden = copy === "";
  }

  function setCatalogActive(index) {
    const items = catalogItems();
    if (!items.length) {
      catalogActive = -1;
      setDescription("");
      return;
    }
    catalogActive = ((index % items.length) + items.length) % items.length;
    items.forEach((el, i) => {
      el.setAttribute("aria-selected", i === catalogActive ? "true" : "false");
    });
    const active = items[catalogActive];
    setDescription(active ? active.dataset.description || "" : "");
    if (active && active.scrollIntoView) {
      active.scrollIntoView({ block: "nearest" });
    }
  }

  function closeCatalog() {
    if (catalogAbort) {
      catalogAbort.abort();
      catalogAbort = null;
    }
    if (!catalogPanel) return;
    catalogPanel.hidden = true;
    catalogPanel.innerHTML = "";
    catalogTrigger = null;
    catalogActive = -1;
    setDescription("");
  }

  function closeFilePanel() {
    if (filePanel) filePanel.innerHTML = "";
    if (fileDialog && fileDialog.open) fileDialog.close();
  }

  function setAttachMenu(on) {
    if (!attachMenu || !fileOpen) return;
    attachMenu.hidden = !on;
    fileOpen.setAttribute("aria-expanded", on ? "true" : "false");
    if (on) {
      setHostMenu(false);
      setModelMenu(false);
      closeCatalog();
    }
  }

  async function pickHost(kind) {
    setAttachMenu(false);
    const res = await fetch("/workspace/pick?kind=" + encodeURIComponent(kind), { method: "POST" });
    if (!res.ok || res.status === 204) return;
    let data;
    try {
      data = await res.json();
    } catch (_) {
      return;
    }
    if (!data || data.cancelled || !data.path) return;
    insertAtCursor(data.path);
  }

  function setHostMenu(on) {
    if (!hostMenu || !hostOpen) return;
    hostMenu.hidden = !on;
    hostOpen.setAttribute("aria-expanded", on ? "true" : "false");
    if (on) setModelMenu(false);
  }

  const modelInput = document.getElementById("composer-model");
  const modelOpen = document.getElementById("composer-model-open");
  const modelMenu = document.getElementById("composer-model-menu");

  function setModelMenu(on) {
    if (!modelMenu || !modelOpen) return;
    modelMenu.hidden = !on;
    modelOpen.setAttribute("aria-expanded", on ? "true" : "false");
  }

  function fillModelMenu(list) {
    if (!modelMenu || !modelOpen || !modelInput) return;
    modelMenu.innerHTML = "";
    const opts = list ? list.querySelectorAll("option") : [];
    if (!opts.length) {
      modelOpen.hidden = true;
      setModelMenu(false);
      modelInput.removeAttribute("list");
      modelInput.placeholder = "Model";
      return;
    }
    modelOpen.hidden = false;
    opts.forEach((opt) => {
      const li = document.createElement("li");
      const btn = document.createElement("button");
      btn.type = "button";
      btn.className = "host-option";
      btn.setAttribute("role", "option");
      btn.textContent = opt.value;
      btn.dataset.model = opt.value;
      li.appendChild(btn);
      modelMenu.appendChild(li);
    });
    modelInput.placeholder = opts[0].value;
  }

  async function refreshCatalog() {
    if (!catalogPanel) return;
    const trig = triggerAtCursor();
    catalogTrigger = trig;
    if (!trig) {
      closeCatalog();
      closeFilePanel();
      return;
    }
    if (trig.kind === "files") {
      closeCatalog();
      setHostMenu(false);
      setModelMenu(false);
      loadFiles(trig.q);
      return;
    }
    closeFilePanel();
    setHostMenu(false);
    setModelMenu(false);
    const url =
      trig.kind === "commands"
        ? "/catalog/commands?q=" + encodeURIComponent(trig.q)
        : "/catalog/skills?q=" + encodeURIComponent(trig.q);
    if (catalogAbort) catalogAbort.abort();
    catalogAbort = new AbortController();
    let res;
    try {
      res = await fetch(url, { signal: catalogAbort.signal });
    } catch (err) {
      if (err && err.name === "AbortError") return;
      return;
    }
    if (!res.ok) return;
    if (triggerAtCursor() == null || triggerAtCursor().kind !== trig.kind) {
      return;
    }
    catalogPanel.innerHTML = await res.text();
    catalogPanel.hidden = false;
    setCatalogActive(0);
  }

  function insertAtTrigger(text) {
    if (!catalogTrigger) return;
    const val = input.value;
    const pos = input.selectionStart == null ? val.length : input.selectionStart;
    const before = val.slice(0, catalogTrigger.start);
    const after = val.slice(pos);
    const next = before + text + (after.startsWith(" ") ? "" : " ") + after.trimStart();
    input.value = next;
    const caret = (before + text + " ").length;
    input.setSelectionRange(caret, caret);
    closeCatalog();
    highlightPreview(input.value);
    input.focus();
  }

  function insertActiveCatalogItem() {
    const items = catalogItems();
    if (!items.length || catalogPanel.hidden) return false;
    const item = items[catalogActive] || items[0];
    insertAtTrigger(item.dataset.insert || "");
    return true;
  }

  function insertAtCursor(text) {
    const val = input.value;
    const start = input.selectionStart == null ? val.length : input.selectionStart;
    const end = input.selectionEnd == null ? start : input.selectionEnd;
    const before = val.slice(0, start);
    const after = val.slice(end);
    const padBefore = before.length && !/\s$/.test(before) ? " " : "";
    const padAfter = after.length && !/^\s/.test(after) ? " " : "";
    input.value = before + padBefore + text + padAfter + after;
    const caret = (before + padBefore + text).length + (padAfter ? 1 : 0);
    input.setSelectionRange(caret, caret);
    highlightPreview(input.value);
    input.focus();
  }

  input.addEventListener("input", () => {
    highlightPreview(input.value);
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(refreshCatalog, 80);
  });

  input.addEventListener("scroll", () => {
    syncPreviewScroll();
  });

  if (catalogPanel) {
    catalogPanel.addEventListener("click", (event) => {
      const item = event.target.closest("[data-insert]");
      if (!item) return;
      insertAtTrigger(item.dataset.insert || "");
    });
    catalogPanel.addEventListener("mousemove", (event) => {
      const item = event.target.closest("[data-insert]");
      if (!item) return;
      const items = catalogItems();
      const index = items.indexOf(item);
      if (index >= 0) setCatalogActive(index);
    });
  }

  async function loadFiles(dir) {
    if (!filePanel) return;
    const q = dir ? "?dir=" + encodeURIComponent(dir) : "";
    const res = await fetch("/workspace/files" + q);
    if (!res.ok) return;
    filePanel.innerHTML = await res.text();
    if (fileDialog && !fileDialog.open) fileDialog.showModal();
  }

  if (fileOpen) {
    fileOpen.addEventListener("click", (event) => {
      event.preventDefault();
      event.stopPropagation();
      closeFilePanel();
      closeCatalog();
      setHostMenu(false);
      setModelMenu(false);
      setAttachMenu(attachMenu ? attachMenu.hidden : true);
    });
  }
  if (attachMenu) {
    attachMenu.addEventListener("click", (event) => {
      event.stopPropagation();
      const option = event.target.closest("[data-kind]");
      if (!option) return;
      pickHost(option.dataset.kind || "");
    });
  }
  if (fileClose) {
    fileClose.addEventListener("click", () => closeFilePanel());
  }
  if (fileDialog) {
    fileDialog.addEventListener("click", (event) => {
      if (event.target === fileDialog) closeFilePanel();
    });
    fileDialog.addEventListener("close", () => {
      if (filePanel) filePanel.innerHTML = "";
    });
  }

  if (filePanel) {
    filePanel.addEventListener("click", (event) => {
      event.stopPropagation();
      if (event.target.closest("[data-testid=file-browser-close]")) {
        closeFilePanel();
        return;
      }
      const pick = event.target.closest("[data-testid=file-attach-row], [data-testid=file-attach-folder]");
      if (pick) {
        const insert = pick.dataset.insert || "";
        if (!insert) return;
        insertAtCursor(insert);
        closeFilePanel();
        return;
      }
      const up = event.target.closest(".file-browser-up");
      if (up) {
        loadFiles(up.dataset.dir || "");
        return;
      }
      const row = event.target.closest("[data-path]");
      if (!row) return;
      if (row.dataset.isDir === "true") {
        loadFiles(row.dataset.path || "");
        return;
      }
      const insert = row.dataset.insert || "";
      if (!insert) return;
      insertAtCursor(insert);
      closeFilePanel();
    });
  }

  if (hostOpen && hostMenu && hostInput) {
    const hostStorageKey = "vibe-composer-host";
    const prefsStorageKey = "vibe-composer-prefs";
    const agentBox = document.getElementById("composer-mode-agent");

    function loadPrefs() {
      try {
        const parsed = JSON.parse(localStorage.getItem(prefsStorageKey) || "{}");
        if (parsed && typeof parsed === "object") return parsed;
      } catch (_) {}
      return {};
    }

    function persistPrefs(id) {
      if (!id) return;
      const prefs = loadPrefs();
      prefs[id] = {
        model: modelInput ? modelInput.value : "",
        agent: !!(agentBox && agentBox.checked)
      };
      try {
        localStorage.setItem(prefsStorageKey, JSON.stringify(prefs));
      } catch (_) {}
    }

    function restorePrefs(id) {
      const saved = loadPrefs()[id];
      if (!saved || typeof saved !== "object") return;
      if (modelInput && typeof saved.model === "string") {
        modelInput.value = saved.model;
      }
      if (agentBox && typeof saved.agent === "boolean") {
        agentBox.checked = saved.agent;
      }
    }
    function applyHost(id, label) {
      hostInput.value = id;
      if (hostLabel) hostLabel.textContent = label || id;
      if (modelInput) {
        const option = hostMenu.querySelector('[data-host-id="' + id + '"]');
        const accepts = option && option.getAttribute("data-accepts-model") === "true";
        modelInput.disabled = !accepts;
        const picker = modelInput.closest(".model-picker");
        if (picker) picker.hidden = !accepts;
        fillModelMenu(accepts ? document.getElementById("composer-models-" + id) : null);
      }
      try {
        localStorage.setItem(hostStorageKey, JSON.stringify({ id: id, label: label || id }));
      } catch (_) {}
      restorePrefs(id);
    }
    try {
      const saved = JSON.parse(localStorage.getItem(hostStorageKey) || "null");
      if (saved && saved.id && hostMenu.querySelector('[data-host-id="' + saved.id + '"]')) {
        applyHost(saved.id, saved.label || saved.id);
      }
    } catch (_) {}
    hostOpen.addEventListener("click", (event) => {
      event.preventDefault();
      event.stopPropagation();
      closeFilePanel();
      closeCatalog();
      setAttachMenu(false);
      setHostMenu(hostMenu.hidden);
    });
    hostMenu.addEventListener("click", (event) => {
      event.stopPropagation();
      const option = event.target.closest("[data-host-id]");
      if (!option) return;
      applyHost(option.dataset.hostId || "", option.dataset.hostLabel || option.dataset.hostId || "");
      setHostMenu(false);
    });
    applyHost(hostInput.value, hostLabel ? hostLabel.textContent : hostInput.value);
    const composerForm = input.closest("form");
    if (composerForm) {
      composerForm.addEventListener("submit", () => {
        persistPrefs(hostInput.value);
      });
    }
  }

  if (modelOpen && modelMenu && modelInput) {
    modelOpen.addEventListener("click", (event) => {
      event.preventDefault();
      event.stopPropagation();
      closeFilePanel();
      closeCatalog();
      setAttachMenu(false);
      setHostMenu(false);
      setModelMenu(modelMenu.hidden);
    });
    modelInput.addEventListener("focus", () => {
      if (!modelOpen.hidden) {
        closeCatalog();
        setHostMenu(false);
        setModelMenu(true);
      }
    });
    modelMenu.addEventListener("click", (event) => {
      event.stopPropagation();
      const option = event.target.closest("[data-model]");
      if (!option) return;
      modelInput.value = option.dataset.model || "";
      setModelMenu(false);
    });
  }

  const form = input.closest("form");
  const hostBusy = document.getElementById("host-busy");
  if (form) {
    form.addEventListener("submit", (event) => {
      if (catalogPanel && !catalogPanel.hidden) {
        event.preventDefault();
        insertActiveCatalogItem();
        return;
      }
      if (hostBusy) hostBusy.hidden = false;
    });
  }

  input.addEventListener("keydown", (event) => {
    if (catalogPanel && !catalogPanel.hidden) {
      if (event.key === "ArrowDown") {
        event.preventDefault();
        setCatalogActive(catalogActive + 1);
        return;
      }
      if (event.key === "ArrowUp") {
        event.preventDefault();
        setCatalogActive(catalogActive - 1);
        return;
      }
      if (event.key === "Tab" || event.key === "Enter") {
        event.preventDefault();
        insertActiveCatalogItem();
        return;
      }
    }
  });

  document.addEventListener("click", (event) => {
    if (catalogPanel && !catalogPanel.hidden) {
      if (!catalogPanel.contains(event.target) && event.target !== input) {
        closeCatalog();
      }
    }
    if (fileDialog && fileDialog.open) {
      if (!fileDialog.contains(event.target) && !(fileOpen && fileOpen.contains(event.target))) {
        closeFilePanel();
      }
    }
    if (hostMenu && !hostMenu.hidden) {
      if (!hostMenu.contains(event.target) && !(hostOpen && hostOpen.contains(event.target))) {
        setHostMenu(false);
      }
    }
    if (attachMenu && !attachMenu.hidden) {
      if (!attachMenu.contains(event.target) && !(fileOpen && fileOpen.contains(event.target))) {
        setAttachMenu(false);
      }
    }
    if (modelMenu && !modelMenu.hidden) {
      if (!modelMenu.contains(event.target) && !(modelOpen && modelOpen.contains(event.target)) && event.target !== modelInput) {
        setModelMenu(false);
      }
    }
  });

  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") return;
    closeFilePanel();
    closeCatalog();
    setAttachMenu(false);
    setHostMenu(false);
    setModelMenu(false);
  });

  highlightPreview(input.value);
})();
