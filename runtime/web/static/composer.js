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
  /** @type {Map<string, string>} display token (@basename) -> send value (@abs path) */
  const attachPathMap = new Map();
  let attachTooltip = null;
  let attachTooltipTimer = null;

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

  function attachRefTitle(ref) {
    if (ref.startsWith('@"') && ref.endsWith('"')) {
      return ref.slice(2, -1);
    }
    return ref.slice(1);
  }

  function isAbsoluteAttachRef(ref) {
    return (
      /^@[A-Za-z]:/.test(ref) ||
      /^@"[A-Za-z]:/.test(ref) ||
      ref.startsWith("@/")
    );
  }

  function escapeRegex(text) {
    return text.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  }

  function pathBasename(path) {
    const bare = String(path || "").replace(/^"|"$/g, "");
    const slash = bare.replace(/\\/g, "/");
    const parts = slash.split("/").filter(Boolean);
    return parts.length ? parts[parts.length - 1] : bare;
  }

  function attachSendValue(path) {
    return "@" + path;
  }

  function makeAttachToken(basename) {
    let token = "@" + basename;
    let suffix = 2;
    while (attachPathMap.has(token)) {
      token = "@" + basename + "~" + suffix;
      suffix += 1;
    }
    return token;
  }

  function expandAttachRefs(text) {
    let out = text;
    const tokens = [...attachPathMap.keys()].sort((a, b) => b.length - a.length);
    tokens.forEach((token) => {
      out = out.split(token).join(attachPathMap.get(token) || token);
    });
    return out;
  }

  function findAttachRefAt(value, pos) {
    const tokens = [...attachPathMap.keys()].sort((a, b) => b.length - a.length);
    for (let i = 0; i < tokens.length; i += 1) {
      const token = tokens[i];
      let idx = 0;
      while (idx < value.length) {
        const at = value.indexOf(token, idx);
        if (at < 0) break;
        const leadOk = at === 0 || /\s/.test(value[at - 1]);
        if (leadOk) {
          const end = at + token.length;
          if (pos >= at && pos <= end) return token;
        }
        idx = at + 1;
      }
    }
    const re = /(^|\s)(@"[^"]+"|@[A-Za-z]:[^\s]+|@[^\s]+)/g;
    let match;
    while ((match = re.exec(value)) !== null) {
      const start = match.index + match[1].length;
      const ref = match[2];
      const end = start + ref.length;
      if (pos >= start && pos <= end) return ref;
    }
    return null;
  }

  function caretIndexFromPoint(inputEl, x, y) {
    if (typeof document.caretPositionFromPoint === "function") {
      const caret = document.caretPositionFromPoint(x, y);
      if (caret && caret.offsetNode === inputEl) return caret.offset;
    }
    return null;
  }

  function ensureAttachTooltip() {
    if (attachTooltip) return attachTooltip;
    const wrap = input.closest(".composer-field-wrap");
    if (!wrap) return null;
    attachTooltip = document.createElement("div");
    attachTooltip.id = "composer-attach-tooltip";
    attachTooltip.className = "composer-attach-tooltip";
    attachTooltip.hidden = true;
    attachTooltip.setAttribute("role", "tooltip");
    wrap.appendChild(attachTooltip);
    return attachTooltip;
  }

  function hideAttachTooltip() {
    if (attachTooltipTimer) {
      window.clearTimeout(attachTooltipTimer);
      attachTooltipTimer = null;
    }
    if (attachTooltip) attachTooltip.hidden = true;
  }

  function showAttachTooltip(fullPath, clientX, clientY) {
    const tip = ensureAttachTooltip();
    if (!tip || !fullPath) return;
    tip.textContent = fullPath;
    tip.hidden = false;
    const pad = 8;
    tip.style.left = clientX + pad + "px";
    tip.style.top = clientY + pad + "px";
  }

  function attachRefFullPath(ref) {
    if (attachPathMap.has(ref)) {
      return attachRefTitle(attachPathMap.get(ref));
    }
    if (isAbsoluteAttachRef(ref)) {
      return attachRefTitle(ref);
    }
    return "";
  }

  function updateAttachTooltipFromPointer(event) {
    const pos = caretIndexFromPoint(input, event.clientX, event.clientY);
    if (pos == null) {
      hideAttachTooltip();
      return;
    }
    const ref = findAttachRefAt(input.value, pos);
    const fullPath = ref ? attachRefFullPath(ref) : "";
    if (!fullPath) {
      hideAttachTooltip();
      return;
    }
    showAttachTooltip(fullPath, event.clientX, event.clientY);
  }

  /** Unicode private-use sentinel; never typed by the user. */
  const HIGHLIGHT_MASK = "\uE000";

  function attachSuggestions(query) {
    const q = (query || "").toLowerCase();
    const hits = [];
    attachPathMap.forEach((sendValue, token) => {
      const name = token.slice(1);
      const lower = name.toLowerCase();
      if (!q || lower.startsWith(q) || lower.includes(q)) {
        hits.push({
          token: token,
          path: attachRefTitle(sendValue),
          name: name
        });
      }
    });
    hits.sort((a, b) => {
      const aStart = a.name.toLowerCase().startsWith(q) ? 0 : 1;
      const bStart = b.name.toLowerCase().startsWith(q) ? 0 : 1;
      if (aStart !== bStart) return aStart - bStart;
      return a.name.length - b.name.length;
    });
    return hits;
  }

  function catalogHasNoMatches() {
    if (!catalogPanel || catalogPanel.hidden) return false;
    return catalogItems().length === 0 && !!catalogPanel.querySelector(".catalog-empty");
  }

  function getHighlightSuppressRange(value) {
    if (!catalogTrigger || catalogTrigger.kind !== "skills") return null;
    if (!catalogHasNoMatches()) return null;
    const pos = input.selectionStart == null ? value.length : input.selectionStart;
    const end = Math.max(pos, catalogTrigger.start + 1);
    if (end <= catalogTrigger.start) return null;
    return { start: catalogTrigger.start, end: end };
  }

  function renderAttachCatalogItems(hits) {
    return hits
      .map((hit) => {
        const token = escapeHtml(hit.token);
        const path = escapeHtml(hit.path);
        return (
          '<li class="catalog-item" data-testid="catalog-item" role="option"' +
          ' data-insert="' + token + '" data-description="' + path + '">' +
          '<span class="catalog-item-name">' + token + "</span>" +
          '<span class="catalog-item-desc">' + path + "</span></li>"
        );
      })
      .join("");
  }

  function renderAttachCatalog(hits) {
    if (!hits.length) {
      return (
        '<ul class="catalog-list" data-testid="composer-catalog" role="listbox">' +
        '<li class="catalog-empty" aria-hidden="true">No matches</li></ul>'
      );
    }
    return (
      '<ul class="catalog-list" data-testid="composer-catalog" role="listbox">' +
      renderAttachCatalogItems(hits) +
      "</ul>"
    );
  }

  function mergeAttachIntoSkillsCatalog(attachHits, skillsHtml) {
    if (!attachHits.length) return skillsHtml;
    const rows = renderAttachCatalogItems(attachHits);
    if (!skillsHtml.includes('data-testid="catalog-item"')) {
      return renderAttachCatalog(attachHits);
    }
    return skillsHtml.replace(
      /(<ul class="catalog-list"[^>]*>)/,
      "$1" + rows
    );
  }

  function isCompleteAttachFragment(fragment) {
    if (!fragment) return false;
    return attachPathMap.has("@" + fragment);
  }

  function maskHighlightRange(value, suppress) {
    if (!suppress) return { masked: value, seg: "" };
    const seg = value.slice(suppress.start, suppress.end);
    const masked =
      value.slice(0, suppress.start) + HIGHLIGHT_MASK + value.slice(suppress.end);
    return { masked: masked, seg: seg };
  }

  function unmaskHighlightHtml(html, seg) {
    if (!seg) return html;
    return html.split(escapeHtml(HIGHLIGHT_MASK)).join(escapeHtml(seg));
  }

  function markComposerRef(lead, ref, fullPath) {
    const tip = fullPath || (isAbsoluteAttachRef(ref) ? attachRefTitle(ref) : "");
    const titleAttr = tip ? ' title="' + escapeHtml(tip) + '"' : "";
    const dataAttr = tip ? ' data-full-path="' + escapeHtml(tip) + '"' : "";
    const cls = tip ? "composer-ref composer-ref-attach" : "composer-ref";
    return lead + '<mark class="' + cls + '"' + titleAttr + dataAttr + ">" + ref + "</mark>";
  }

  function highlightMappedAttachRefs(html) {
    const tokens = [...attachPathMap.keys()].sort((a, b) => b.length - a.length);
    tokens.forEach((token) => {
      const sendValue = attachPathMap.get(token) || "";
      const fullPath = attachRefTitle(sendValue);
      const re = new RegExp("(^|\\s)(" + escapeRegex(escapeHtml(token)) + ")(?=\\s|$)", "g");
      html = html.replace(re, (_, lead, ref) => markComposerRef(lead, ref, fullPath));
    });
    return html;
  }

  function highlightComposerAtRefs(html) {
    html = html.replace(/(^|\s)(@"[^"]+")/g, (_, lead, ref) =>
      markComposerRef(lead, ref, isAbsoluteAttachRef(ref) ? attachRefTitle(ref) : "")
    );
    html = html.replace(/(^|\s)(@[A-Za-z]:[^\s]+)/g, (_, lead, ref) =>
      markComposerRef(lead, ref, attachRefTitle(ref))
    );
    html = html.replace(/(^|\s)(@[a-z0-9./_-]+)/gi, (_, lead, ref) => markComposerRef(lead, ref, ""));
    return html;
  }

  function highlightPreview(value) {
    if (!preview) return;
    const suppress = getHighlightSuppressRange(value);
    const masked = maskHighlightRange(value, suppress);
    let html = escapeHtml(masked.masked);
    html = html.replace(/(^|\s)(\/[a-z0-9-]+)/gi, '$1<mark class="composer-ref">$2</mark>');
    html = highlightMappedAttachRefs(html);
    html = highlightComposerAtRefs(html);
    html = unmaskHighlightHtml(html, masked.seg);
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
      if (isCompleteAttachFragment(fragment)) {
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
    highlightPreview(input.value);
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

  let pickNoticeTimer;

  function showPickNotice(text) {
    setDescription(text);
    if (pickNoticeTimer) window.clearTimeout(pickNoticeTimer);
    if (!text) return;
    pickNoticeTimer = window.setTimeout(() => setDescription(""), 5000);
  }

  async function pickHost(kind) {
    setAttachMenu(false);
    const res = await fetch("/workspace/pick?kind=" + encodeURIComponent(kind), { method: "POST" });
    if (res.status === 501) {
      showPickNotice("Attach picker is not available on this system.");
      return;
    }
    if (res.status === 502) {
      showPickNotice("Attach picker failed. Try again.");
      return;
    }
    if (!res.ok || res.status === 204) {
      if (!res.ok) showPickNotice("Attach picker failed.");
      return;
    }
    let data;
    try {
      data = await res.json();
    } catch (_) {
      showPickNotice("Attach picker failed.");
      return;
    }
    if (!data || data.cancelled || !data.path) return;
    showPickNotice("");
    const sendValue = attachSendValue(data.path);
    const token = makeAttachToken(pathBasename(data.path));
    attachPathMap.set(token, sendValue);
    insertAtCursor(token);
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
    const attachHits = trig.kind === "skills" ? attachSuggestions(trig.q) : [];
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
      if (attachHits.length) {
        catalogPanel.innerHTML = renderAttachCatalog(attachHits);
        catalogPanel.hidden = false;
        setCatalogActive(0);
        highlightPreview(input.value);
      }
      return;
    }
    if (!res.ok) return;
    if (triggerAtCursor() == null || triggerAtCursor().kind !== trig.kind) {
      return;
    }
    let body = await res.text();
    if (trig.kind === "skills") {
      body = mergeAttachIntoSkillsCatalog(attachHits, body);
    }
    catalogPanel.innerHTML = body;
    catalogPanel.hidden = false;
    setCatalogActive(0);
    highlightPreview(input.value);
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

  const fieldWrap = input.closest(".composer-field-wrap");
  if (fieldWrap) {
    fieldWrap.addEventListener("mousemove", (event) => {
      updateAttachTooltipFromPointer(event);
    });
    fieldWrap.addEventListener("mouseleave", () => {
      hideAttachTooltip();
    });
  }

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
      input.value = expandAttachRefs(input.value);
      attachPathMap.clear();
      hideAttachTooltip();
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
