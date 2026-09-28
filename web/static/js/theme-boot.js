// Loaded first, synchronously, in every page's <head>: applies the saved
// theme before first paint so dark mode never flashes light. index.js keeps
// it in sync afterwards (applyTheme / toggleDarkMode).
try {
  if (localStorage.getItem("darkMode") === "true") {
    document.documentElement.classList.add("theme-dark");
  }
} catch (e) {
  // storage blocked: stay on the light theme
}
