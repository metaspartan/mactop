export const PHASE = {
  simple: true,
  advanced: false,
  complex: false,
};

const tabs = document.querySelectorAll(".tab");
const views = {
  simple: document.getElementById("view-simple"),
  advanced: document.getElementById("view-advanced"),
  complex: document.getElementById("view-complex"),
};

let activeView = "simple";

export function getActiveView() {
  return activeView;
}

export function switchView(viewName) {
  if (!PHASE[viewName]) {
    return;
  }

  activeView = viewName;

  tabs.forEach((tab) => {
    const isActive = tab.dataset.view === viewName;
    tab.classList.toggle("active", isActive);
    tab.setAttribute("aria-selected", String(isActive));
  });

  Object.entries(views).forEach(([name, element]) => {
    const isActive = name === viewName;
    element.classList.toggle("active", isActive);
    element.hidden = !isActive;
  });
}

export function initTabs() {
  tabs.forEach((tab) => {
    tab.addEventListener("click", () => {
      if (tab.disabled || !PHASE[tab.dataset.view]) {
        return;
      }
      switchView(tab.dataset.view);
    });
  });
}
