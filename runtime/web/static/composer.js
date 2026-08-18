(function () {
  const input = document.getElementById("composer-message");
  const catalogPanel = document.getElementById("composer-catalog-panel");
  const descPanel = document.getElementById("composer-description");
  const preview = document.getElementById("composer-preview");
  const filePanel = document.getElementById("composer-file-panel");
  const fileOpen = document.getElementById("composer-file-open");
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

  function highlightPreview(value) {
    if (!preview) return;
    let html = escapeHtml(value);
    html = html.replace(/\/([a-z0-9-]+)/gi, '<mark class="composer-ref">/$1</mark>');
    html = html.replace(/@([a-z0-9./_-]+)/gi, '<mark class="composer-ref">@$1</mark>');
    preview.innerHTML = html;
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

  async function refreshCatalog() {
    if (!catalogPanel) return;
    const trig = triggerAtCursor();
    catalogTrigger = trig;
    if (!trig) {
      catalogPanel.hidden = true;
      catalogPanel.innerHTML = "";
      return;
    }
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
    input.value = before + text + (after.startsWith(" ") ? "" : " ") + after.trimStart();
    catalogPanel.hidden = true;
    catalogPanel.innerHTML = "";
    catalogTrigger = null;
    highlightPreview(input.value);
    input.focus();
  }

  input.addEventListener("input", () => {
    highlightPreview(input.value);
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(refreshCatalog, 120);
  });

  if (catalogPanel) {
    catalogPanel.addEventListener("click", (event) => {
      const item = event.target.closest("[data-insert]");
      if (!item) return;
      if (descPanel) {
        descPanel.textContent = item.dataset.description || "";
        descPanel.hidden = false;
      }
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
  }

  if (fileOpen) {
    fileOpen.addEventListener("click", () => loadFiles(""));
  }

  if (filePanel) {
    filePanel.addEventListener("click", async (event) => {
      const up = event.target.closest(".file-browser-up");
      if (up) {
        loadFiles(up.dataset.dir || "");
        return;
      }
      const row = event.target.closest("[data-path]");
      if (!row) return;
      const path = row.dataset.path;
      const isDir = row.dataset.isDir === "true";
      if (isDir) {
        loadFiles(path);
        return;
      }
      const res = await fetch("/workspace/files/preview?path=" + encodeURIComponent(path));
      if (!res.ok) return;
      filePanel.innerHTML = await res.text();
    });

    filePanel.addEventListener("click", (event) => {
      const attach = event.target.closest("[data-testid=file-attach]");
      if (!attach) return;
      const insert = attach.dataset.insert || "";
      input.value = (input.value.trim() + " " + insert).trim();
      if (descPanel && attach.dataset.excerpt) {
        descPanel.textContent = attach.dataset.excerpt;
        descPanel.hidden = false;
      }
      filePanel.hidden = true;
      highlightPreview(input.value);
      input.focus();
    });
  }
})();
