// Run before the application/styles so the first painted canvas has the right theme.
(() => {
  const key = "folio.theme";
  const modes = ["system", "light", "dark"];
  const media = window.matchMedia("(prefers-color-scheme: dark)");
  const listeners = new Set();
  const stored = () => {
    try {
      const value = localStorage.getItem(key);
      return modes.includes(value) ? value : "system";
    } catch {
      return "system";
    }
  };
  let preference = stored();
  function apply() {
    const resolved =
      preference === "system" ? (media.matches ? "dark" : "light") : preference;
    const root = document.documentElement;
    root.classList.add("theme-switching");
    root.dataset.theme = resolved;
    // Flush the new palette while hover transitions are disabled. Otherwise
    // foregrounds can change before a transitioning background catches up.
    void root.offsetHeight;
    requestAnimationFrame(() => root.classList.remove("theme-switching"));
    const meta = document.querySelector('meta[name="theme-color"]');
    if (meta) meta.content = resolved === "dark" ? "#15171c" : "#ffffff";
    listeners.forEach((listener) => listener());
  }
  window.folioTheme = {
    get: () => preference,
    set(value) {
      if (!modes.includes(value)) return;
      preference = value;
      try {
        localStorage.setItem(key, value);
      } catch {
        /* Storage is optional. */
      }
      apply();
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
  };
  media.addEventListener("change", apply);
  window.addEventListener("storage", (event) => {
    if (event.key === key || event.key === null) {
      preference = stored();
      apply();
    }
  });
  apply();
})();
