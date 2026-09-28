/*
 * ui-actions.js - every click/change the templates used to wire with inline
 * handlers (onclick=, onchange=, onfocus=, hx-on::*, hx-vals="js:...") lives
 * here, as delegated listeners on document, so the panel runs under a CSP
 * without 'unsafe-inline' / 'unsafe-eval' and keeps working for content htmx
 * swaps in later.
 *
 * Markup vocabulary (data-* attributes):
 *   data-show-section="id"            showSection(id) + nav/crumb state
 *   data-show-subsection="parent:id"  showSubSection(parent, id) + tab state
 *   data-toggle-hidden="#sel"         toggleHidden(sel, data-toggle-group)
 *   data-hide-closest="sel"           hide(closest(sel))
 *   data-show="#sel"                  show(sel)
 *   data-copy="#sel"                  copy sel's text
 *   data-clear="#sel"                 empty sel
 *   data-reset-form="#sel"            reset form sel
 *   data-toggle-class="c" data-target="#sel"
 *   data-toggle-job-optionals="#sel"  toggle_job_optionals(sel)
 *   data-toggle-dropdown              open/close the next .dropdown
 *   data-download="url"               downloadResource(url)
 *   data-edit-user / data-cancel-edit / data-mod-job / data-mod-app / data-run-app
 *   data-autofill-guard               readonly until focused (defeats autofill)
 *   data-retry                        re-run a failed initial load
 */

/* ---------------------------------------------------------------- helpers */

// kEsc escapes a value for use inside HTML text or a quoted attribute.
function kEsc(v) {
  return String(v ?? "").replace(/[&<>"'`]/g, (ch) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;", "`": "&#96;",
  })[ch]);
}

// kQ encodes a value for a URL query parameter.
function kQ(v) {
  return encodeURIComponent(String(v ?? ""));
}

function kToast(message, kind = "error") {
  const box = document.getElementById("k-toasts");
  if (!box) return;
  const t = document.createElement("div");
  t.className = `k-toast is-${kind}`;
  t.setAttribute("role", kind === "error" ? "alert" : "status");
  const icon = document.createElement("i");
  icon.className = kind === "error" ? "fa-solid fa-circle-exclamation" : "fa-solid fa-circle-check";
  icon.setAttribute("aria-hidden", "true");
  const text = document.createElement("span");
  text.textContent = message;
  const close = document.createElement("button");
  close.type = "button";
  close.className = "k-toast-close";
  close.setAttribute("aria-label", "Dismiss");
  close.textContent = "×";
  close.addEventListener("click", () => t.remove());
  t.append(icon, text, close);
  box.appendChild(t);
  setTimeout(() => t.classList.add("is-leaving"), 6000);
  setTimeout(() => t.remove(), 6400);
}

/* ------------------------------------------------ formatting (humanize) */

function kFmtWhen(s) {
  if (!s) return "";
  const str = String(s).trim();
  if (!str || str.startsWith("0001-01-01")) return "—";
  const d = new Date(str.replace(" +0000 UTC", "Z").replace(" ", "T"));
  if (isNaN(d)) return str;
  const p = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
}

// job durations come from Go's time.Duration: nanoseconds
function kFmtDuration(v) {
  const ns = Number(v);
  if (!isFinite(ns) || ns <= 0) return "—";
  const s = ns / 1e9;
  if (s < 1) return `${Math.round(s * 1000)} ms`;
  if (s < 60) return `${s.toFixed(s < 10 ? 2 : 1)} s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${Math.round(s % 60)}s`;
  return `${Math.floor(m / 60)}h ${m % 60}m`;
}

function kFmtBytes(b) {
  const n = Number(b);
  if (!isFinite(n) || n <= 0) return n === 0 ? "0 B" : String(b);
  const u = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.min(u.length - 1, Math.floor(Math.log(n) / Math.log(1024)));
  const v = n / 1024 ** i;
  return `${v >= 10 || i === 0 ? Math.round(v) : v.toFixed(1)} ${u[i]}`;
}

// humanize rewrites display text only; the raw value stays in data-* /
// title so the JS that reads cells (search, modals) keeps working.
function kHumanize(root = document) {
  root.querySelectorAll("[data-ts]:not([data-human])").forEach((el) => {
    el.dataset.human = "1";
    const raw = el.dataset.ts;
    if (!raw) { el.textContent = "—"; return; }
    el.title = raw;
    el.textContent = kFmtWhen(raw);
  });
  root.querySelectorAll("[data-duration]:not([data-human])").forEach((el) => {
    el.dataset.human = "1";
    el.textContent = kFmtDuration(el.dataset.duration);
  });
  root.querySelectorAll("td[data-bytes]:not([data-human])").forEach((el) => {
    el.dataset.human = "1";
    el.title = `${el.dataset.bytes} bytes`;
  });
  root.querySelectorAll("[data-volume-kind]:not([data-human])").forEach((el) => {
    el.dataset.human = "1";
    const kind = kVolumeKind(el.dataset.volumeKind);
    el.textContent = kind === "default" ? "default" : kind === "shared" ? "shared" : "";
    el.classList.add(`is-${kind}`);
    if (kind === "shared") el.title = "Shared by one of your groups; usage counts against the group's quota";
  });
}

// Which volumes are shared: for a regular user fetch-volumes returns the
// default volume plus the volumes their groups share. The JSON does not flag
// which is which, so the default is recognised by name.
function kIsElevated() {
  const dash = document.getElementById("dash");
  return !!dash && (dash.dataset.elevated === "1" || dash.dataset.elevated === "true");
}
function kVolumeKind(name) {
  const n = String(name || "");
  if (/(^|-)default$/.test(n)) return "default";
  return kIsElevated() ? "" : "shared";
}

/* ---------------------------------------------------- navigation state */

function kMarkSection(id) {
  document.querySelectorAll("#sidebar [data-show-section]").forEach((b) => {
    b.classList.toggle("is-active", b.dataset.showSection === id);
  });
  const sec = document.getElementById(id);
  const crumb = document.getElementById("k-crumb-current");
  if (crumb && sec) crumb.textContent = sec.dataset.title || id;
  const main = document.getElementById("main-content");
  if (main) main.scrollTo({ top: 0 });
}

function kMarkSubSection(parentId, id) {
  const parent = document.getElementById(parentId);
  if (!parent) return;
  parent.querySelectorAll("[data-show-subsection]").forEach((b) => {
    const on = b.dataset.showSubsection === `${parentId}:${id}`;
    b.setAttribute("aria-selected", on ? "true" : "false");
    b.classList.toggle("is-active", on);
  });
  // CodeMirror measures nothing while hidden: redraw once it is visible
  const cm = document.querySelector(`#${id} .CodeMirror`);
  if (cm && cm.CodeMirror) cm.CodeMirror.refresh();
}

// wrap the index.js functions so every caller (dashboard.js too) updates the
// nav highlight, the breadcrumb and the job-browser tabs
if (typeof showSection === "function") {
  const baseShowSection = showSection;
  showSection = function (sectionId) {
    baseShowSection(sectionId);
    kMarkSection(sectionId);
  };
}
if (typeof showSubSection === "function") {
  const baseShowSubSection = showSubSection;
  showSubSection = function (divId, sectionId) {
    baseShowSubSection(divId, sectionId);
    kMarkSubSection(divId, sectionId);
  };
}

/* ------------------------------------------------------- click handlers */

const kClickActions = [
  ["[data-show-section]", (el) => showSection(el.dataset.showSection)],
  ["[data-show-subsection]", (el) => {
    const [parent, id] = el.dataset.showSubsection.split(":");
    showSubSection(parent, id);
  }],
  ["[data-toggle-hidden]", (el) => {
    const target = document.querySelector(el.dataset.toggleHidden);
    if (!target) return;
    // a second click on the same edit button closes its form again
    if (!target.classList.contains("hidden")) { hide(target); return; }
    toggleHidden(el.dataset.toggleHidden, el.dataset.toggleGroup || el.dataset.toggleHidden);
    const first = target.querySelector("input:not([type=hidden])");
    if (first) first.focus();
  }],
  ["[data-hide-closest]", (el) => hide(el.closest(el.dataset.hideClosest))],
  ["[data-show]", (el) => show(document.querySelector(el.dataset.show))],
  ["[data-hide]", (el) => hide(document.querySelector(el.dataset.hide))],
  ["[data-copy]", (el) => copyToClipboard(el.dataset.copy, el.id)],
  ["[data-clear]", (el) => {
    const t = document.querySelector(el.dataset.clear);
    if (t) t.textContent = "";
  }],
  ["[data-reset-form]", (el) => {
    const f = document.querySelector(el.dataset.resetForm);
    if (f) f.reset();
  }],
  ["[data-toggle-class]", (el) => {
    const t = document.querySelector(el.dataset.target);
    if (t) t.classList.toggle(el.dataset.toggleClass);
  }],
  ["[data-toggle-job-optionals]", (el) => {
    toggle_job_optionals(document.querySelector(el.dataset.toggleJobOptionals));
    el.classList.toggle("is-active");
  }],
  ["[data-toggle-dropdown]", (el) => {
    const dd = el.parentNode.querySelector(".dropdown");
    if (dd) dd.classList.toggle("open");
    el.setAttribute("aria-expanded", dd && dd.classList.contains("open") ? "true" : "false");
  }],
  ["[data-download]", (el) => downloadResource(el.dataset.download)],
  ["[data-edit-user]", (el) => editUser(el.dataset.uid, el.dataset.index)],
  ["[data-cancel-edit]", (el) => cancelEdit(el.dataset.cancelEdit)],
  ["[data-mod-job]", (el) => {
    const entry = el.closest(".job-display-entry");
    const list = el.closest("ul");
    modJobModal(entry, list ? list.parentNode : null);
  }],
  ["[data-mod-app]", (el) => {
    const disp = document.getElementById("apps-list-display");
    modAppModal(el.closest(".app-card"), disp ? disp.parentNode : null);
  }],
  ["[data-run-app]", (el) => {
    showSubSection("job-browser", "new-job");
    const sel = document.getElementById("language-selector");
    const app = el.dataset.runApp;
    if (sel && [...sel.options].some((o) => o.value === app)) {
      sel.value = app;
      sel.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }],
  ["[data-open-file]", (el, e) => {
    if (e.target.closest(".file-box")) return; // clicks on a queued file (and its ✖)
    const inp = document.querySelector(el.dataset.openFile);
    if (inp) inp.click();
  }],
  ["[data-retry]", (el) => {
    const src = document.getElementById(el.dataset.retry);
    if (!src || !window.htmx) return;
    const target = src.getAttribute("hx-target");
    htmx.ajax("GET", src.getAttribute("hx-get"), {
      source: src,
      target: !target || target === "this" ? src : target,
      swap: src.getAttribute("hx-swap") || "innerHTML",
    });
  }],
];

document.addEventListener("click", (e) => {
  for (const [sel, fn] of kClickActions) {
    const el = e.target.closest(sel);
    if (!el) continue;
    if (el.tagName === "A" && el.getAttribute("href") === "#") e.preventDefault();
    if (el.tagName === "BUTTON" && el.type === "submit" && !el.closest("form")) e.preventDefault();
    fn(el, e);
    return;
  }

  // tree view (tree-resources.html): show the clicked file's details
  const file = e.target.closest("#resources-main .file[data-name]");
  if (file) {
    kShowTreeFile(file);
    return;
  }

  // close an open resource dropdown when clicking elsewhere
  document.querySelectorAll(".dropdown.open").forEach((dd) => {
    if (!dd.parentNode.contains(e.target)) dd.classList.remove("open");
  });
});

// keyboard: Enter/Space on a selectable table row, Esc closes modals
document.addEventListener("keydown", (e) => {
  if ((e.key === "Enter" || e.key === " ") && e.target.matches("#resource-list-table tbody tr")) {
    e.preventDefault();
    e.target.click();
  }
  if (e.key === "Escape") {
    const open = [...document.querySelectorAll(".modal:not(.hidden)")].pop();
    if (open) hide(open);
  }
});

// clicking the dimmed backdrop of a modal closes it
document.addEventListener("mousedown", (e) => {
  if (e.target.classList && e.target.classList.contains("modal")) hide(e.target);
});

function kShowTreeFile(file) {
  const d = file.dataset;
  const target = document.getElementById("resource-details");
  if (!target) return;
  document.querySelectorAll("#resources-main .file.is-selected").forEach((f) => f.classList.remove("is-selected"));
  file.classList.add("is-selected");
  target.textContent = "";
  const h = document.createElement("h3");
  h.className = "k-panel-title";
  h.textContent = "File details";
  const dl = document.createElement("dl");
  dl.className = "k-details";
  [
    ["Name", d.name], ["Type", d.type], ["Size", d.size], ["Permissions", d.perms],
    ["Created", d.created], ["Updated", d.updated], ["Accessed", d.accessed],
    ["Owner", d.owner], ["Group", d.group], ["Volume", d.volume],
  ].forEach(([k, v]) => {
    const row = document.createElement("div");
    const dt = document.createElement("dt");
    dt.textContent = k;
    const dd = document.createElement("dd");
    dd.textContent = v ?? "";
    row.append(dt, dd);
    dl.appendChild(row);
  });
  target.append(h, dl);
}

/* ------------------------------------------------ change / focus handlers */

document.addEventListener("change", (e) => {
  const t = e.target;
  if (t.id === "dark-mode-toggle") toggleDarkMode();
  else if (t.id === "tips-mode-toggle") toggleCollapses();
  else if (t.matches("#permissions-container input[type=checkbox]")) updatePermissionString();
});

// inputs that start readonly (so browsers do not autofill them) unlock on focus
document.addEventListener("focusin", (e) => {
  if (e.target.matches("input[readonly][data-autofill-guard]")) e.target.removeAttribute("readonly");
});

/* --------------------------------- htmx: what hx-vals="js:" / hx-on did */

document.addEventListener("htmx:configRequest", (evt) => {
  const elt = evt.detail.elt;
  if (!elt || !elt.id) return;

  // job list reload sends the "search by" column as sort
  if (elt.id === "fetch-jobs-button" || elt.id === "fetch-jobs-button-2") {
    const box = document.getElementById(elt.id === "fetch-jobs-button" ? "existing-jobs-container" : "existing-jobs-container-2");
    const sel = box && box.querySelector(".k-search-by");
    if (sel) evt.detail.parameters.sort = sel.value;
  }

  // user row edit: submit the edited cells
  if (elt.id.startsWith("submit-btn-") && elt.dataset.uid !== undefined) {
    const vals = getUserPatchValues(elt.dataset.uid);
    Object.entries(vals).forEach(([k, v]) => { evt.detail.parameters[k] = v; });
  }

  // preview paging: byte window per page
  if (elt.id === "next-arrow-left" || elt.id === "next-arrow-right") {
    evt.detail.headers.Range = "bytes=" + getPreviewWindow(elt.id === "next-arrow-left" ? -1 : +1);
  }
});

document.addEventListener("htmx:beforeRequest", (evt) => {
  const elt = evt.detail.elt;
  if (!elt) return;
  if (elt.classList.contains("r-btn-delete")) {
    show(elt.closest("#resource-details, #selected-resource-display")?.querySelector(".r-loader") || document.querySelector(".r-loader"));
  } else if (elt.id === "upload-files-form-dash") {
    show(document.getElementById("progress-container"));
  }
});

document.addEventListener("htmx:afterRequest", (evt) => {
  const elt = evt.detail.elt;
  if (!elt || !elt.classList) return;
  if (elt.classList.contains("r-btn-edit") && evt.detail.successful) {
    show(elt.closest(".resource-options")?.querySelector("#edit-modal-2"));
  }
  if (elt.classList.contains("r-btn-delete")) {
    hide(elt.closest("#resource-details, #selected-resource-display")?.querySelector(".r-loader"));
  }
});

/* --------------------------------------- loading / empty / error states */

function kLoadState(elt, target, status) {
  if (!target) return;
  const what = (elt.closest("[data-title]")?.dataset.title || "this").toLowerCase();
  const box = document.createElement("div");
  if (status === 404) {
    box.className = "k-empty";
    box.innerHTML = '<i class="fa-regular fa-folder-open" aria-hidden="true"></i><p><strong>Nothing here yet.</strong></p>';
  } else {
    box.className = "k-error-state";
    const icon = document.createElement("i");
    icon.className = "fa-solid fa-triangle-exclamation";
    icon.setAttribute("aria-hidden", "true");
    const p = document.createElement("p");
    p.textContent = status ? `Could not load ${what} (HTTP ${status}).` : `Could not reach the server to load ${what}.`;
    const retry = document.createElement("button");
    retry.type = "button";
    retry.className = "k-btn k-btn-sm";
    retry.textContent = "Try again";
    if (!elt.id) elt.id = "k-load-" + Math.random().toString(36).slice(2, 8);
    retry.dataset.retry = elt.id;
    box.append(icon, p, retry);
  }
  target.replaceChildren(box);
}

function kFailed(evt) {
  const elt = evt.detail.elt;
  const xhr = evt.detail.xhr;
  const status = xhr ? xhr.status : 0;
  if (status === 401) return; // index.js sends the user to the login page
  const trig = elt && elt.getAttribute && (elt.getAttribute("hx-trigger") || "");
  const verb = (evt.detail.requestConfig && evt.detail.requestConfig.verb) || "";
  if (/\bload\b/.test(trig) && verb === "get" && evt.detail.target && elt.getAttribute("hx-swap") !== "none") {
    kLoadState(elt, evt.detail.target, status);
    return;
  }
  if (verb === "get" && status !== 404) {
    kToast(status ? `Request failed (HTTP ${status}).` : "Could not reach the server.");
  }
}
document.addEventListener("htmx:responseError", kFailed);
document.addEventListener("htmx:sendError", kFailed);

/* ------------------------------------------------------------ lifecycle */

document.addEventListener("htmx:afterSettle", (evt) => kHumanize(evt.detail.target || document));

function kInit() {
  // the upload header doubles as the "browse" button
  const boxes = document.getElementById("file-boxes");
  if (boxes) {
    boxes.dataset.openFile = "#file";
    boxes.setAttribute("title", "Browse files");
  }
  kHumanize(document);
  const visible = document.querySelector(".content-section:not(.hidden)");
  if (visible) kMarkSection(visible.id);
}

if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", kInit);
else kInit();
