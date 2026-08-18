(function () {
  const input = document.getElementById("composer-message");
  const catalogPanel = document.getElementById("composer-catalog-panel");
  const preview = document.getElementById("composer-preview");
  const filePanel = document.getElementById("composer-file-panel");
  const fileOpen = document.getElementById("composer-file-open");
  const hostOpen = document.getElementById("composer-host-open");
  const hostMenu = document.getElementById("composer-host-menu");
  const hostInput = document.getElementById("composer-host");
  const hostLabel = document.getElementById("composer-host-label");
  if (!input) return;

  let catalogTrigger = null;
  let debounceTimer;

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
    html = html.replace(/\/([a-z0-9-]+)/gi, '<mark class="composer-ref">/$1</mark>');
    html = html.replace(/@([a-z0-9./_-]+)/gi, '<mark class="composer-ref">@$1</mark>');
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
      if (fragment.includes(" ") || fragment.includes("/")) {
        return null;
      }
      return { kind: "skills", q: fragment, start: at };
    }
    return null;
  }

  function closeCatalog() {
    if (!catalogPanel) return;
    catalogPanel.hidden = true;
    catalogPanel.innerHTML = "";
    catalogTrigger = null;
  }

  function closeFilePanel() {
    if (!filePanel) return;
    filePanel.hidden = true;
    filePanel.innerHTML = "";
    if (fileOpen) fileOpen.setAttribute("aria-expanded", "false");
  }

  function setHostMenu(on) {
    if (!hostMenu || !hostOpen) return;
    hostMenu.hidden = !on;
    hostOpen.setAttribute("aria-expanded", on ? "true" : "false");
  }

  async function refreshCatalog() {
    if (!catalogPanel) return;
    const trig = triggerAtCursor();
    catalogTrigger = trig;
    if (!trig) {
      closeCatalog();
      return;
    }
    closeFilePanel();
    setHostMenu(false);
    const url =
      trig.kind === "commands"
        ? "/catalog/commands?q=" + encodeURIComponent(trig.q)
        : "/catalog/skills?q=" + encodeURIComponent(trig.q);
    const res = await fetch(url);
    if (!res.ok) return;
    catalogPanel.innerHTML = await res.text();
    catalogPanel.hidden = false;
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
    debounceTimer = setTimeout(refreshCatalog, 120);
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
  }

  async function loadFiles(dir) {
    if (!filePanel) return;
    const q = dir ? "?dir=" + encodeURIComponent(dir) : "";
    const res = await fetch("/workspace/files" + q);
    if (!res.ok) return;
    filePanel.innerHTML = await res.text();
    filePanel.hidden = false;
    if (fileOpen) fileOpen.setAttribute("aria-expanded", "true");
  }

  if (fileOpen) {
    fileOpen.addEventListener("click", (event) => {
      event.stopPropagation();
      setHostMenu(false);
      if (filePanel && !filePanel.hidden) {
        closeFilePanel();
        return;
      }
      closeCatalog();
      loadFiles("");
    });
  }

  if (filePanel) {
    filePanel.addEventListener("click", (event) => {
      event.stopPropagation();
      if (event.target.closest("[data-testid=file-browser-close]")) {
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
    function applyHost(id, label) {
      hostInput.value = id;
      if (hostLabel) hostLabel.textContent = label || id;
      try {
        localStorage.setItem(hostStorageKey, JSON.stringify({ id: id, label: label || id }));
      } catch (_) {}
    }
    try {
      const saved = JSON.parse(localStorage.getItem(hostStorageKey) || "null");
      if (saved && saved.id && hostMenu.querySelector('[data-host-id="' + saved.id + '"]')) {
        applyHost(saved.id, saved.label || saved.id);
      }
    } catch (_) {}
    hostOpen.addEventListener("click", (event) => {
      event.stopPropagation();
      closeFilePanel();
      closeCatalog();
      setHostMenu(hostMenu.hidden);
    });
    hostMenu.addEventListener("click", (event) => {
      const option = event.target.closest("[data-host-id]");
      if (!option) return;
      applyHost(option.dataset.hostId || "", option.dataset.hostLabel || option.dataset.hostId || "");
      setHostMenu(false);
    });
  }

  const form = input.closest("form");
  const hostBusy = document.getElementById("host-busy");
  if (form && hostBusy) {
    form.addEventListener("submit", () => {
      hostBusy.hidden = false;
    });
  }

  document.addEventListener("click", (event) => {
    if (filePanel && !filePanel.hidden) {
      if (!filePanel.contains(event.target) && !(fileOpen && fileOpen.contains(event.target))) {
        closeFilePanel();
      }
    }
    if (hostMenu && !hostMenu.hidden) {
      if (!hostMenu.contains(event.target) && !(hostOpen && hostOpen.contains(event.target))) {
        setHostMenu(false);
      }
    }
  });

  document.addEventListener("keydown", (event) => {
    if (event.key !== "Escape") return;
    closeFilePanel();
    closeCatalog();
    setHostMenu(false);
  });
})();
