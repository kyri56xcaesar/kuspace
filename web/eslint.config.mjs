// ESLint for the browser code (make lint-js). The page scripts are classic
// <script>s sharing one global scope, so the names one script defines for
// the others are declared here as globals.
import js from "@eslint/js";
import globals from "globals";

// what each page script defines for the others (and for the templates)
const shared = {
  "index.js": ["showSection", "showSubSection", "show", "hide", "toggleHidden", "copyToClipboard",
    "downloadResource", "updatePermissionString", "getPreviewWindow", "toggleDarkMode", "toggleCollapses", "getCookie"],
  "admin-panel.js": ["cachedUsers", "cachedGroups", "cachedResources", "cacheVolumeResults", "cacheResourceResults",
    "fileUploadModule", "WS_ADDRESS", "editUser", "cancelEdit", "getUserPatchValues", "toggle_job_optionals",
    "addResourceListListeners", "setupSearchBar", "modJobModal", "modAppModal", "resourceDetailsHTML"],
  "gshell.js": ["newTerminal", "giveFunctionality"],
  "vfs.js": ["vfsRoot", "currentPath", "buildTree", "renderVFS"],
  "ui-actions.js": ["kToast", "kFmtBytes", "kFmtWhen", "kVolumeKind", "kEsc", "kQ"],
  "fileUploadModule.js": ["fileUploadContainerFunctionality"],
  "jobCodeInput.js": ["setupJobSubmitter", "createFeedbackPanel"],
};
// writable: scripts reassign them (ui-actions.js wraps showSection)
const pageGlobals = Object.fromEntries(Object.values(shared).flat().map((n) => [n, "writable"]));

export default [
  { ignores: ["static/js/htmx/**", "static/js/codemirror/**", "node_modules/**"] },
  js.configs.recommended,
  {
    files: ["static/js/**/*.js"],
    languageOptions: {
      ecmaVersion: 2023,
      sourceType: "script",
      globals: { ...globals.browser, ...pageGlobals, htmx: "readonly", CodeMirror: "readonly" },
    },
    rules: {
      // top-level functions are the pages' shared API: used by other scripts and templates
      "no-unused-vars": ["error", { vars: "local", args: "none", caughtErrors: "none" }],
      // the shared names are declared above, so defining them isn't a redeclaration
      "no-redeclare": ["error", { builtinGlobals: false }],
      "no-empty": ["error", { allowEmptyCatch: true }],
      eqeqeq: ["error", "smart"],
    },
  },
  {
    files: ["tests/**/*.js"],
    languageOptions: { ecmaVersion: 2023, sourceType: "commonjs", globals: globals.node },
  },
];
