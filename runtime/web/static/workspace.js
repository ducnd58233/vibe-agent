(function () {
  const themeKey = "vibe-theme";
  function applyTheme(theme) {
    if (theme !== "light" && theme !== "dark") {
      return;
    }
    document.documentElement.setAttribute("data-theme", theme);
    document.querySelectorAll('input[name="theme"]').forEach((el) => {
      el.checked = el.value === theme;
    });
  }
  try {
    applyTheme(localStorage.getItem(themeKey));
  } catch (e) {}
  document.addEventListener("change", (e) => {
    if (!e.target || e.target.name !== "theme") {
      return;
    }
    applyTheme(e.target.value);
    try {
      localStorage.setItem(themeKey, e.target.value);
    } catch (err) {}
  });
})();

(function () {
  const menu = document.getElementById("workspace-menu");
  const openBtn = document.getElementById("workspace-menu-open");
  const browseOpen = document.getElementById("workspace-browse-open");
  const browseDialog = document.getElementById("workspace-browse-dialog");
  const browseClose = document.getElementById("workspace-browse-close");
  if (!menu || !openBtn) {
    return;
  }
  function setOpen(on) {
    menu.hidden = !on;
    openBtn.setAttribute("aria-expanded", on ? "true" : "false");
  }
  openBtn.addEventListener("click", (event) => {
    event.stopPropagation();
    setOpen(menu.hidden);
  });
  document.addEventListener("click", (event) => {
    if (menu.hidden) {
      return;
    }
    if (menu.contains(event.target) || openBtn.contains(event.target)) {
      return;
    }
    setOpen(false);
  });
  if (browseOpen && browseDialog) {
    browseOpen.addEventListener("click", () => {
      setOpen(false);
      browseDialog.showModal();
      loadBrowse("");
    });
  }
  if (browseClose && browseDialog) {
    browseClose.addEventListener("click", () => browseDialog.close());
  }
  const browsePanel = document.getElementById("workspace-browse-panel");
  const browsePath = document.getElementById("workspace-browse-path");
  async function loadBrowse(dir) {
    if (!browsePanel) return;
    const q = dir ? "?dir=" + encodeURIComponent(dir) : "";
    const res = await fetch("/workspace/browse" + q);
    if (!res.ok) return;
    browsePanel.innerHTML = await res.text();
    const openBtn = browsePanel.querySelector("[data-open-path]");
    if (browsePath && openBtn) {
      browsePath.value = openBtn.getAttribute("data-open-path") || "";
    }
  }
  if (browsePanel) {
    browsePanel.addEventListener("click", (event) => {
      event.preventDefault();
      const openBtn = event.target.closest("[data-testid=file-open-folder]");
      if (openBtn) {
        if (browsePath) browsePath.value = openBtn.getAttribute("data-open-path") || "";
        const form = browseDialog && browseDialog.querySelector("form");
        if (form) form.requestSubmit();
        return;
      }
      const up = event.target.closest(".file-browser-up");
      if (up) {
        loadBrowse(up.dataset.dir || "");
        return;
      }
      const row = event.target.closest("[data-path]");
      if (!row) return;
      if (row.dataset.isDir === "true") {
        loadBrowse(row.dataset.path || "");
      }
    });
  }
})();
