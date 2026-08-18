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
    });
  }
  if (browseClose && browseDialog) {
    browseClose.addEventListener("click", () => browseDialog.close());
  }
})();
