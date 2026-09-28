/* code specific that only admin panel should use. */

/**************************************************************************/
// global variables/constants
/**************************************************************************/
cachedUsers = [];
cachedGroups = [];
cachedResources = [];
var cacheVolumeResults = [];
var cacheResourceResults = [];


const PORT = window.location.port || (window.location.protocol === "https:" ? "443" : "80");
const WS_PORT = "8082"
const IP = window.location.hostname;
let fileUploadModule;


let WS_ADDRESS = null;
async function initWebSocketAddress() {
  try {
    const response = await fetch("/conf");
    if (!response.ok) {
      throw new Error(`Failed to fetch config: ${response.status}`);
    }

    const config = await response.json();
    if (!config.ws_address) {
      throw new Error("WebSocket address not provided in config");
    }

    const port = config.ws_address.trim().split(":")[1]
    WS_ADDRESS = IP + ":" + port;
    console.log("WS_ADDRESS set to:", WS_ADDRESS);
  } catch (err) {
    console.error("Error setting WS_ADDRESS:", err);
    WS_ADDRESS = null;
  }
}
(async () => {
  await initWebSocketAddress();

  if (!WS_ADDRESS) {
    console.error("WebSocket address not initialized.");
    return;
  }
})();


/************************************************************************** */
// global functions/utilities
/**************************************************************************/

// escape for HTML text/attributes (ui-actions.js has the same as kEsc; kept
// local so this file does not depend on load order)
function escHTML(v) {
  return String(v ?? "").replace(/[&<>"'`]/g, (ch) => ({
    "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;", "`": "&#96;",
  })[ch]);
}

// the original cells of rows being edited, so Cancel restores them exactly
const userEditOriginals = new Map();

// user entries control
function editUser(uid, index) {
  const row = document.getElementById(`table-${index}`);
  if (!row) return;

  const cells = row.querySelectorAll('td');
  if (!cells || !cells.length) return;

  const original = { html: [], text: [] };

  for (let i = 0; i < cells.length - 1; i++) {
    const cell = cells[i];
    original.html[i] = cell.innerHTML;
    original.text[i] = cell.textContent.trim();

    if (i == 0 || i == 5) {
      continue;
    }

    const input = document.createElement('input');
    input.type = 'text';
    input.id = 'edit-input-' + uid + '-' + i;
    input.classList.add("table-input");
    input.placeholder = original.text[i];
    input.dataset.index = i;
    cell.textContent = '';
    cell.appendChild(input);
  }
  userEditOriginals.set(String(index), original);
  row.classList.add("is-editing");

  const actionsCell = cells[cells.length - 1];
  actionsCell.innerHTML = `
    <div id="actions-btns">
      <button
        id="submit-btn-${escHTML(index)}"
        class="k-btn k-btn-sm k-btn-primary"
        data-uid="${escHTML(uid)}"
        hx-patch="/api/v1/verified/admin/userpatch"
        hx-swap="none"
        hx-trigger="click"
        hx-confirm="Are you sure you want to update user ${escHTML(original.text[0])}?"
        type="button"
      >Save</button>
      <button id="cancel-btn-${escHTML(index)}" type="button" class="k-btn k-btn-sm k-btn-ghost" data-cancel-edit="${escHTML(index)}">Cancel</button>
    </div>
  `;
  htmx.process(document.getElementById(`submit-btn-${index}`));
  const first = row.querySelector(".table-input");
  if (first) first.focus();
}

function getUserPatchValues(uid) {
  const val = (i) => {
    const el = document.getElementById("edit-input-" + uid + "-" + i);
    return el ? el.value : "";
  };
  return {
    uid: uid,
    username: val(1),
    password: val(2),
    info: val(3),
    home: val(4),
    groups: val(6),
  };
}

function cancelEdit(index) {
  const row = document.getElementById(`table-${index}`);
  const original = userEditOriginals.get(String(index));
  if (!row || !original) return;

  const cells = row.querySelectorAll('td');

  // restore the server-rendered (already escaped) cell markup
  for (let i = 0; i < cells.length - 1; i++) {
    cells[i].innerHTML = original.html[i];
  }
  userEditOriginals.delete(String(index));
  row.classList.remove("is-editing");

  const uid = original.text[0];
  const actionsCell = cells[cells.length - 1];
  actionsCell.innerHTML = `
    <div id="actions-btns">
      <button type="button" class="k-btn k-btn-sm k-btn-ghost" id="edit-btn-${escHTML(index)}" data-edit-user data-uid="${escHTML(uid)}" data-index="${escHTML(index)}">Edit</button>
      ${uid === "0" ? "" : `<button
        type="button"
        class="k-btn k-btn-sm k-btn-danger"
        id="delete-btn-${escHTML(index)}"
        hx-delete="/api/v1/verified/admin/userdel?uid=${encodeURIComponent(uid)}"
        hx-swap="none"
        hx-trigger="click"
        hx-target="#table-${escHTML(index)}"
        hx-confirm="Are you sure you want to delete user ${escHTML(uid)}?"
      >Delete</button>`}
    </div>
  `;
  const del = document.getElementById(`delete-btn-${index}`);
  if (del) htmx.process(del);
}

function toggle_job_optionals(jobDiv) {
  if (jobDiv) {
    const jobOptionals = jobDiv.querySelectorAll(".job-optional");
    jobOptionals.forEach((optional) => {
      optional.classList.toggle("hidden");
    });
  }
}

// volumes listed on the Volumes page, for the pickers
function listedVolumes() {
  return [...document.querySelectorAll(".v-body")].map((v) => ({
    name: (v.querySelector("h3") || {}).textContent?.trim() || "",
    usage: (v.querySelector(".k-vol-numbers") || {}).textContent?.replace(/\s+/g, " ").trim() || "",
    kind: typeof kVolumeKind === "function" ? kVolumeKind((v.querySelector("h3") || {}).textContent?.trim()) : "",
  })).filter((v) => v.name);
}

// fill a picker list with one button per option (DOM built, no innerHTML)
function fillPicker(listEl, items, onPick, emptyText) {
  listEl.textContent = "";
  if (!items.length) {
    const p = document.createElement("p");
    p.className = "k-dim";
    p.textContent = emptyText;
    listEl.appendChild(p);
    return;
  }
  items.forEach((it) => {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "k-pick";
    const name = document.createElement("span");
    name.className = "k-pick-name";
    name.textContent = it.label;
    button.appendChild(name);
    if (it.kind) {
      const badge = document.createElement("span");
      badge.className = `k-vol-badge is-${it.kind}`;
      badge.textContent = it.kind;
      button.appendChild(badge);
    }
    if (it.meta) {
      const meta = document.createElement("span");
      meta.className = "k-pick-meta";
      meta.textContent = it.meta;
      button.appendChild(meta);
    }
    button.addEventListener("click", () => onPick(it));
    listEl.appendChild(button);
  });
}


/**************************************************************************/
// after DOM content is loaded, actions, lets say initialization of page functionalities
/**************************************************************************/

document.addEventListener("DOMContentLoaded", () => {
  /**************************************************************************/
  // side-bar
  /**************************************************************************/
  const sidebar = document.getElementById('sidebar');
  const toggleSidebarButton = document.getElementById('toggle-sidebar');
  const sidebarList = document.querySelectorAll('.collapsing');
  toggleSidebarButton.addEventListener('click', () => {
    sidebar.classList.toggle('collapsed');
    const collapsed = sidebar.classList.contains('collapsed');
    toggleSidebarButton.setAttribute('aria-label', collapsed ? 'Expand sidebar' : 'Collapse sidebar');
    toggleSidebarButton.title = collapsed ? 'Expand sidebar' : 'Collapse sidebar';
    sidebarList.forEach((item) => item.classList.toggle('is-rail', collapsed));
  });

  /**************************************************************************/
  // files
  /**************************************************************************/
  const dropZone = document.getElementById("drop-zone");
  const fileInput = document.getElementById("file");
  const fileBoxContainer = document.getElementById("file-boxes");
  const submitButton = document.getElementById("upload-button");
  const fileNameDisplay = document.getElementById("file-name");

  fileUploadModule = fileUploadContainerFunctionality(
    dropZone,
    fileInput,
    fileBoxContainer,
    submitButton,
    fileNameDisplay
  );


  /**************************************************************************/
  // job setup
  /**************************************************************************/
  setupJobSubmitter(document.querySelector('#job-create-form-editor'));

  // select a resource input for the job
  const job_input = document.getElementById("job-input")
  job_input.value = "";
  document.getElementById("select-resource-btn-job").addEventListener("click", () => {
    const modal = document.getElementById("select-resource-btn-job").parentNode.querySelector(".modal");
    const resourceList = modal.querySelector("#resource-list");

    const items = [];
    const rTable = document.querySelector("#resource-list-table > tbody");
    if (!rTable) {
      // not admin: the resources come from the files view
      (cachedResources || []).forEach((li) => {
        let rname = li.name || "";
        rname = rname.startsWith("/") ? rname : "/" + rname;
        items.push({ label: (li.vname || "") + rname, meta: typeof kFmtBytes === "function" ? kFmtBytes(li.size) : "" });
      });
    } else {
      rTable.querySelectorAll("tr").forEach((li) => {
        let rname = li.querySelector(".name").textContent.trim();
        rname = rname.startsWith("/") ? rname : "/" + rname;
        const vname = li.querySelector(".volume").textContent.trim();
        items.push({ label: vname + rname });
      });
    }
    fillPicker(resourceList, items, (it) => {
      job_input.value = it.label;
      modal.classList.add("hidden");
    }, "No files yet. Upload one from Access Profile first.");
    modal.classList.remove("hidden");
  });


  // select a volume output for the job
  const job_output = document.getElementById("job-output");
  job_output.value = "";
  document.getElementById("select-volume-btn-job").addEventListener("click", () => {
    const modal = document.getElementById("select-volume-btn-job").parentNode.querySelector(".modal");
    const volumeList = modal.querySelector("#volume-list");
    fillPicker(volumeList, listedVolumes().map((v) => ({ label: v.name, meta: v.usage, kind: v.kind })), (it) => {
      job_output.value = it.label + "/";
      modal.classList.add("hidden");
    }, "No volumes loaded yet.");
    modal.classList.remove("hidden");
  });



  /**************************************************************************/
  // Volumes
  /**************************************************************************/
  const vSearch = document.getElementById("volume-search");
  let vSearchBy = "name";
  const vSearchSelector = document.querySelector(".v-header").querySelector("#search-by");
  vSearchSelector.value = vSearchBy;
  vSearchSelector.addEventListener("input", () => {
    vSearchBy = vSearchSelector.value;
    vSearch.placeholder = "Search by '" + vSearchBy + "'";
  });

  vSearch.value = "";
  vSearch.addEventListener("input", function () {
    const searchValue = vSearch.value;
    cacheVolumeResults.forEach((li) => {
      const cell = li.querySelector(vSearchBy === "name" ? ".name" : ".createdat");
      const text = cell ? (cell.title || "") + " " + cell.innerText : "";
      li.classList.toggle("hidden", !text.includes(searchValue));
    });
  });

  const cancelModalbtn = document.getElementById("cancel-modal-btn");
  if (cancelModalbtn) {
    cancelModalbtn.addEventListener("click", () => {
      document.getElementById("create-volume-modal").classList.add("hidden");
    });
  }

  // for choosing a volume when upload
  document.getElementById("select-volume-btn").addEventListener("click", () => {
    const modal = document.getElementById("select-volume-btn").parentNode.querySelector(".modal");
    const volumeList = modal.querySelector("#volume-list");
    fillPicker(volumeList, listedVolumes().map((v) => ({ label: v.name, meta: v.usage, kind: v.kind })), (it) => {
      const sel = document.getElementById("selected-volume");
      sel.textContent = it.label;
      sel.dataset.kind = it.kind || "";
      modal.classList.add("hidden");
    }, "No volumes loaded yet.");
    modal.classList.remove("hidden");
  });


  document.querySelectorAll("#cancel-select").forEach((cancel_btn) => {
    cancel_btn.addEventListener("click", () => {
      cancel_btn.closest(".modal").classList.add("hidden");
    });
  })


  /**************************************************************************/
  // Resources
  /**************************************************************************/
  const rSearch = document.getElementById("resource-search");
  if (rSearch) {
    let rSearchBy = "name";
    const rSearchBySelector = document.getElementById("resources-header").querySelector("#search-by");
    rSearchBySelector.value = rSearchBy;
    rSearchBySelector.addEventListener("input", () => {
      rSearchBy = rSearchBySelector.value;
      rSearch.placeholder = "Search by '" + rSearchBy + "'";
    });
    rSearch.value = "";
    rSearch.addEventListener("input", function () {
      const searchValue = rSearch.value;
      const cls = { name: ".name", volume: ".volume", createdat: ".createdat", updateddat: ".updatedat", updatedat: ".updatedat", accessedat: ".accessedat", owner: ".owner", group: ".group" }[rSearchBy];
      if (!cls) return;
      cacheResourceResults.forEach((li) => {
        const cell = li.querySelector(cls);
        const text = cell ? (cell.dataset.ts || "") + " " + cell.innerText : "";
        li.classList.toggle("hidden", !text.includes(searchValue));
      });
    });
  }
});

function addResourceListListeners() {
  const tableRows = document.querySelectorAll("#resource-list-table tbody tr");
  const resourceDetails = document.getElementById("resource-details");

  tableRows.forEach((row) => {
    row.addEventListener("click", () => {
      tableRows.forEach((r) => r.classList.remove("selected"));
      row.classList.add("selected");

      parseAndInjectRTableRowdata(row, resourceDetails);

      resourceDetails.querySelectorAll(".r-btn-download, .r-btn-edit, .r-btn-delete, #preview-resource-btn, #next-arrow-right, #next-arrow-left").forEach(button => {
        htmx.process(button);
      });
    });
  });
}

// resourceDetailsHTML renders the details + actions block shared by the
// Resources page and the Files view. Every value is escaped.
function resourceDetailsHTML(r, opts) {
  const q = encodeURIComponent;
  const previewURL = `/api/v1/verified/preview?rid=${q(r.id)}&resourcename=${q(r.name)}&volume=${q(r.vname)}`;
  const extraCls = opts.vfs ? " vfs-action-btn" : "";
  const arrow = (dir) => `
    <button type="button"
      id="next-arrow-${dir}"
      class="next-arrow k-btn k-btn-icon k-btn-ghost${extraCls}"
      title="${dir === "left" ? "Previous page" : "Next page"}"
      aria-label="${dir === "left" ? "Previous page" : "Next page"}"
      hx-target="#${opts.previewId}"
      hx-trigger="click"
      hx-swap="innerHTML"
      hx-get="${escHTML(previewURL)}"
    ><i class="fa-solid fa-chevron-${dir}" aria-hidden="true"></i></button>`;
  const facts = opts.facts.map(([k, v]) => `<div><dt>${escHTML(k)}</dt><dd>${escHTML(v)}</dd></div>`).join("");

  return `
    <div class="resource-details-headers">
      <div class="k-details-title">
        <p class="k-eyebrow">${escHTML(r.vname)}</p>
        <h3 class="k-mono" title="${escHTML(r.name)}">${escHTML(r.name)}</h3>
      </div>
      <div class="resource-options">
        <button type="button" id="resource-options-dropdown-button" class="k-btn k-btn-sm" data-toggle-dropdown aria-haspopup="true" aria-expanded="false">Actions <i class="fa-solid fa-chevron-down" aria-hidden="true"></i></button>
        <div class="resource-options-inner dropdown" role="menu">
          <button type="button" class="r-btn-download" role="menuitem"
            data-download="${escHTML(`/api/v1/verified/download?target=${q(r.name)}&volume=${q(r.vname)}`)}"
          ><i class="fa-solid fa-download" aria-hidden="true"></i> Download</button>
          <button type="button"
            id="preview-resource-btn"
            class="${opts.vfs ? "vfs-action-btn" : ""}"
            role="menuitem"
            hx-target="#${opts.previewId}"
            hx-trigger="click"
            hx-swap="innerHTML"
            hx-get="${escHTML(previewURL)}"
            hx-headers='{"Range": "bytes=${getPreviewWindow(0)}"}'
          ><i class="fa-solid fa-eye" aria-hidden="true"></i> Preview</button>
          <button type="button"
            class="r-btn-edit"
            role="menuitem"
            hx-get="${escHTML(`/api/v1/verified/edit-form?resourcename=${q(r.name)}&owner=${q(r.owner || 0)}&group=${q(r.group || 0)}&perms=${q(r.perms)}&rid=${q(r.id)}&volume=${q(r.vname)}`)}"
            hx-swap="innerHTML"
            hx-trigger="click"
            hx-target="next .modal"
          ><i class="fa-solid fa-pen" aria-hidden="true"></i> Edit</button>
          <button type="button"
            class="r-btn-delete${extraCls}"
            role="menuitem"
            hx-delete="${escHTML(`/api/v1/verified/rm?name=${q(r.name)}&volume=${q(r.vname)}`)}"
            hx-trigger="click"
            hx-swap="none"
            hx-confirm="${escHTML(`Are you sure you want to delete resource ${r.name}?`)}"
          ><i class="fa-solid fa-trash" aria-hidden="true"></i> Delete</button>
          <button type="button" id="close-r-selected-display" role="menuitem" ${opts.closeAttr}><i class="fa-solid fa-xmark" aria-hidden="true"></i> Close</button>
        </div>
        <div id="edit-modal-2" class="modal hidden darkened"></div>
      </div>
      ${opts.draggable ? '<div id="selected-resource-draggable-bar" class="draggable-bar" title="Drag to move"></div>' : ""}
    </div>
    <div class="resource-details-main">
      <dl class="resource-details-inner k-details">${facts}</dl>
      <div id="resource-preview" class="resource-preview-window">
        <div class="resource-preview-main blurred">
          <div id="${opts.previewId}" class="resource-preview-content"><p class="k-dim">Choose <strong>Preview</strong> to read the first 4 KB.</p></div>
          <div id="resource-preview-controls">
            ${arrow("left")}
            <span class="k-page">page <span id="page-index">0</span></span>
            ${arrow("right")}
          </div>
        </div>
      </div>
    </div>
    <div class="resource-details-footer">
      <div class="feedback"></div>
      <div class="r-loader hidden"><div></div></div>
    </div>`;
}

function parseAndInjectRTableRowdata(tr, injectTarget) {
  if (!tr || tr.cells.length != 13 || !injectTarget) {
    return
  }
  const cell = (i) => (tr.cells[i].dataset.ts || tr.cells[i].innerText).trim();

  const resource = {
    id: cell(0),
    name: cell(1),
    path: cell(2),
    vname: cell(3),
    type: cell(4),
    size: cell(5),
    perms: cell(6),
    createdAt: cell(7),
    updatedAt: cell(8),
    accessedAt: cell(9),
    owner: cell(10),
    group: cell(11),
    vid: cell(12),
  };

  injectTarget.innerHTML = resourceDetailsHTML(resource, {
    previewId: "resource-preview-content-1",
    vfs: false,
    draggable: false,
    closeAttr: 'data-clear="#resource-details"',
    facts: [
      ["RID", resource.id], ["Path", resource.path], ["Volume", resource.vname],
      ["Type", resource.type], ["Size", `${resource.size} B`], ["Permissions", resource.perms],
      ["Created", resource.createdAt], ["Updated", resource.updatedAt], ["Accessed", resource.accessedAt],
      ["Owner", resource.owner], ["Group", resource.group], ["VID", resource.vid],
    ],
  });
}

function setupSearchBar(jobSearchDiv, cacheJobResults) {
  let searchBy = "jid";
  const jobSearch = jobSearchDiv.querySelector("#job-search");
  const jobSearchSelector = jobSearchDiv.querySelector("#search-by");
  jobSearchSelector.value = searchBy;
  jobSearchSelector.addEventListener("input", () => {
    searchBy = jobSearchSelector.value;
    jobSearch.placeholder = "Search by '" + searchBy + "'";
  });

  const cls = { jid: ".jid", uid: ".uid", createdAt: ".createdAt", completed_at: ".completedAt", status: ".status", output: ".output", input: ".input" };
  jobSearch.value = "";
  jobSearch.addEventListener("input", function () {
    const searchValue = jobSearch.value;
    const sel = cls[searchBy];
    if (!sel) return;
    cacheJobResults.forEach((li) => {
      const el = li.querySelector(sel);
      const text = el ? (el.dataset.v || "") + " " + el.innerText : "";
      li.classList.toggle("hidden", !text.includes(searchValue));
    });
  });
}

// field reads a value the list templates keep in data-v (falls back to text)
function field(root, cls) {
  const el = root.querySelector("." + cls);
  if (!el) return "";
  return (el.dataset.v !== undefined ? el.dataset.v : el.textContent).trim();
}

function showModal(parentDiv, html, formSel) {
  let modal = parentDiv.querySelector(':scope > .modal');
  if (!modal) {
    modal = document.createElement("div");
    modal.className = "modal";
    parentDiv.appendChild(modal);
  }
  modal.innerHTML = html;
  modal.classList.remove('hidden');
  htmx.process(modal.querySelector(formSel));
  const first = modal.querySelector("textarea, input:not([readonly]):not([type=hidden])");
  if (first) first.focus();
}

function modJobModal(div, parentDiv) {
  if (!div || !parentDiv) {
    return
  }
  const f = (c) => escHTML(field(div, c));
  const completed = field(div, 'completed');

  const html = `
  <div class="modal-content k-modal-wide" role="dialog" aria-labelledby="mod-job-title">
    <h2 id="mod-job-title">Modify job <span class="k-mono">#${f('jid')}</span></h2>
    <form
      id="modify-job-form"
      hx-put="/api/v1/verified/admin/jobs"
      hx-swap="none"
      hx-trigger="submit"
    >
      <div class="disabled-display k-readonly-grid">
        <div class="k-field"><label>Job id</label><input type="number" name="jid" value="${f('jid')}" readonly tabindex="-1"></div>
        <div class="k-field"><label>Duration</label><input type="text" name="duration" value="${f('duration')}" readonly tabindex="-1"></div>
        <div class="k-field"><label>Input</label><input type="text" name="input" value="${f('input')}" readonly tabindex="-1"></div>
        <div class="k-field"><label>Output</label><input type="text" name="output" value="${f('output')}" readonly tabindex="-1"></div>
        <div class="k-field"><label>Created</label><input type="text" name="createdAt" value="${f('createdAt')}" readonly tabindex="-1"></div>
        <div class="k-field"><label>Completed at</label><input type="text" name="completedAt" value="${f('completedAt')}" readonly tabindex="-1"></div>
        <input type="hidden" name="logic" value="${f('logic')}">
        <input type="hidden" name="logicBody" value="${f('logicBody')}">
        <input type="hidden" name="logicHeaders" value="${f('logicHeaders')}">
        <input type="hidden" name="ephemeralStorageLimit" value="${f('ephemeralStorageLimit')}">
        <input type="hidden" name="ephemeralStorageRequest" value="${f('ephemeralStorageRequest')}">
        <input type="hidden" name="cpuLimit" value="${f('cpuLimit')}">
        <input type="hidden" name="memoryLimit" value="${f('memoryLimit')}">
        <input type="hidden" name="cpuRequest" value="${f('cpuRequest')}">
        <input type="hidden" name="memoryRequest" value="${f('memoryRequest')}">
        <input type="hidden" name="priority" value="${f('priority')}">
        <input type="hidden" name="parallelism" value="${f('parallelism')}">
        <input type="hidden" name="timeout" value="${f('timeout')}">
      </div>
      <div class="k-field">
        <label for="mj-description">Description</label>
        <textarea id="mj-description" name="description" maxlength="150">${f('description')}</textarea>
      </div>
      <div class="k-field-row">
        <div class="k-field">
          <label for="mj-uid">User id</label>
          <input id="mj-uid" type="number" name="uid" value="${f('uid')}" min="0" required>
        </div>
        <div class="k-field">
          <label for="mj-status">Status</label>
          <input id="mj-status" type="text" name="status" value="${f('status')}" required>
        </div>
      </div>
      <label class="k-check">
        <input type="checkbox" name="completed" value="true" ${completed === "true" ? "checked" : ""}> Completed
      </label>
      <div class="modal-actions">
        <button type="button" id="cancel-modal-btn" class="k-btn" data-hide-closest=".modal">Cancel</button>
        <button type="submit" class="k-btn k-btn-primary">Save</button>
      </div>
    </form>
  </div>
  <div class="feedback hidden modal-feedback"></div>
  `;
  showModal(parentDiv, html, "#modify-job-form");
}

function modAppModal(div, parentDiv) {
  if (!div || !parentDiv) {
    return
  }
  const f = (c) => escHTML(field(div, c));
  const version = escHTML(field(div, 'app-version').replace(/^v/, ""));

  const html = `
  <div class="modal-content k-modal-wide" role="dialog" aria-labelledby="mod-app-title">
    <h2 id="mod-app-title">Modify application</h2>
    <form
      id="modify-app-form"
      class="modify-app"
      hx-put="/api/v1/verified/admin/apps"
      hx-swap="none"
      hx-trigger="submit"
    >
      <div class="disabled-display k-readonly-grid">
        <div class="k-field"><label>App id</label><input type="number" name="id" value="${f('app-id')}" readonly tabindex="-1"></div>
        <div class="k-field"><label>Created</label><input type="text" name="createdAt" value="${f('createdAt')}" readonly tabindex="-1"></div>
        <div class="k-field"><label>Inserted</label><input type="text" name="insertedAt" value="${f('insertedAt')}" readonly tabindex="-1"></div>
        <input type="hidden" name="authorId" value="${f('authorId')}">
      </div>
      <div class="k-field-row">
        <div class="k-field"><label for="ma-name">Name</label><input id="ma-name" type="text" name="name" value="${f('app-name')}" required></div>
        <div class="k-field"><label for="ma-version">Version</label><input id="ma-version" type="text" name="version" value="${version}" required></div>
      </div>
      <div class="k-field"><label for="ma-image">Image</label><input id="ma-image" class="k-mono" type="text" name="image" value="${f('image')}" required></div>
      <div class="k-field-row">
        <div class="k-field"><label for="ma-author">Author</label><input id="ma-author" type="text" name="author" value="${f('author')}" required></div>
        <div class="k-field"><label for="ma-status">Status</label><input id="ma-status" type="text" name="status" value="${f('app-status')}" required></div>
      </div>
      <div class="k-field">
        <label for="ma-description">Description</label>
        <textarea id="ma-description" name="description" maxlength="150">${f('app-description')}</textarea>
      </div>
      <div class="modal-actions">
        <button type="button" id="cancel-modal-btn" class="k-btn" data-hide-closest=".modal">Cancel</button>
        <button type="submit" class="k-btn k-btn-primary">Save</button>
      </div>
    </form>
  </div>
  <div class="feedback hidden modal-feedback"></div>
  `;
  showModal(parentDiv, html, "#modify-app-form");
}
