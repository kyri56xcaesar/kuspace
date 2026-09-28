// quotas.js - the "Quotas & sharing" admin page: volumes shared by a group
// (their quota is the group's) and personal quotas on the other volumes.
// Everything is built as DOM nodes with textContent (no HTML strings).
(function () {
  "use strict";
  const API = "/api/v1/verified";
  const state = { volumes: [], groups: [], users: [], shared: [], quotas: [] };

  const $ = (id) => document.getElementById(id);
  const el = (tag, attrs = {}, ...kids) => {
    const n = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs)) {
      if (k === "class") n.className = v;
      else if (k === "text") n.textContent = v;
      else n.setAttribute(k, v);
    }
    n.append(...kids.filter((k) => k != null));
    return n;
  };
  const gb = (v) => `${(Number(v) || 0).toFixed(2)} GB`;
  const toast = (msg, kind) => (typeof kToast === "function" ? kToast(msg, kind) : alert(msg));

  async function call(method, path, params) {
    const init = { method, credentials: "same-origin", headers: { Accept: "application/json" } };
    let url = API + path;
    if (params && method === "GET") url += "?" + new URLSearchParams(params);
    else if (params && method === "DELETE") url += "?" + new URLSearchParams(params);
    else if (params) {
      init.body = new URLSearchParams(params);
      init.headers["Content-Type"] = "application/x-www-form-urlencoded";
    }
    const r = await fetch(url, init);
    let body = null;
    try { body = await r.json(); } catch (e) { /* empty or not JSON */ }
    if (!r.ok) throw new Error((body && body.error) || `HTTP ${r.status}`);
    return body;
  }

  // usage over quota: a meter, or "unlimited" with the unbounded bar
  function usage(used, quota) {
    const wrap = el("div", { class: "k-vol-usage" });
    wrap.append(el("p", { class: "k-vol-numbers" }, el("strong", { text: gb(used) }),
      el("span", { text: quota > 0 ? ` of ${gb(quota)}` : " of unlimited" })));
    if (quota > 0) {
      const m = el("progress", { class: "k-meter", max: String(quota) });
      m.value = Math.min(Number(used) || 0, quota);
      if (used > quota) m.classList.add("is-over");
      wrap.append(m);
    } else wrap.append(el("div", { class: "k-meter-unbounded", "aria-hidden": "true" }));
    return wrap;
  }

  function stateBox(cls, icon, text) {
    return el("div", { class: cls }, el("i", { class: icon, "aria-hidden": "true" }), el("p", { text }));
  }

  // a quota input + Save that calls save(value)
  function quotaEditor(value, label, save) {
    const input = el("input", { type: "number", min: "0", step: "0.1", class: "k-quota-input", "aria-label": label });
    input.value = String(value);
    const btn = el("button", { type: "button", class: "k-btn k-btn-sm", text: "Save" });
    btn.addEventListener("click", async () => {
      btn.disabled = true;
      try { await save(input.value || "0"); toast("Saved.", "success"); await load(); }
      catch (e) { toast(e.message); }
      finally { btn.disabled = false; }
    });
    return el("div", { class: "k-quota-edit" }, input, btn);
  }

  const groupName = (gid) => (state.groups.find((g) => g.gid === gid) || {}).groupname || `gid ${gid}`;
  const userName = (uid) => (state.users.find((u) => u.uid === uid) || {}).username || `uid ${uid}`;

  function renderShared() {
    const target = $("gv-target");
    target.replaceChildren();
    if (!state.shared.length) {
      target.append(stateBox("k-empty", "fa-solid fa-people-roof", "No volume is shared yet. Share one below."));
      return;
    }
    const tbody = el("tbody");
    for (const gv of state.shared) {
      const stop = el("button", { type: "button", class: "k-btn k-btn-sm k-btn-danger", text: "Stop sharing" });
      stop.addEventListener("click", async () => {
        if (!confirm(`Stop sharing ${gv.vname}? Only possible while it holds no files.`)) return;
        try { await call("DELETE", "/admin/group-volumes", { volume: gv.vname }); toast(`${gv.vname} is no longer shared.`, "success"); await load(); }
        catch (e) { toast(e.message); }
      });
      tbody.append(el("tr", {},
        el("td", {}, el("strong", { class: "k-mono", text: gv.vname })),
        el("td", {}, el("span", { class: "k-chip", text: groupName(gv.gid) })),
        el("td", { class: "k-quota-cell" }, usage(gv.usage, gv.quota)),
        el("td", {}, quotaEditor(gv.quota, `Quota of ${gv.vname}`, (q) =>
          call("POST", "/admin/group-volumes", { vname: gv.vname, gid: gv.gid, quota: q }))),
        el("td", { class: "k-actions-col" }, stop)));
    }
    target.append(el("div", { class: "k-table-wrap" }, el("table", { class: "k-table" },
      el("thead", {}, el("tr", {}, el("th", { text: "Volume" }), el("th", { text: "Group" }), el("th", { text: "Usage" }),
        el("th", { text: "Group quota (GB)" }), el("th", { class: "k-actions-col" }, el("span", { class: "sr-only", text: "Actions" })))),
      tbody)));
  }

  function renderQuotas() {
    const target = $("uq-target");
    target.replaceChildren();
    if (!state.quotas.length) {
      target.append(stateBox("k-empty", "fa-solid fa-user-gear", "No personal quotas here yet: everyone gets the default on first upload."));
      return;
    }
    const tbody = el("tbody");
    for (const uq of state.quotas) {
      const reset = el("button", { type: "button", class: "k-btn k-btn-sm", text: "Reset" });
      reset.title = "Back to the default quota";
      reset.addEventListener("click", async () => {
        try { await call("DELETE", "/admin/user-volumes", { volume: uq.vname, uid: uq.uid }); toast("Back to the default quota.", "success"); await load(); }
        catch (e) { toast(e.message); }
      });
      tbody.append(el("tr", {},
        el("td", {}, el("strong", { text: userName(uq.uid) }), el("span", { class: "k-dim k-mono", text: ` #${uq.uid}` })),
        el("td", { class: "k-mono", text: uq.vname }),
        el("td", { class: "k-quota-cell" }, usage(uq.usage, uq.quota)),
        el("td", {}, quotaEditor(uq.quota, `Quota of ${userName(uq.uid)} on ${uq.vname}`, (q) =>
          call("PATCH", "/admin/user-volumes", { vname: uq.vname, uid: uq.uid, quota: q }))),
        el("td", { class: "k-actions-col" }, reset)));
    }
    target.append(el("div", { class: "k-table-wrap" }, el("table", { class: "k-table" },
      el("thead", {}, el("tr", {}, el("th", { text: "User" }), el("th", { text: "Volume" }), el("th", { text: "Usage" }),
        el("th", { text: "Quota (GB)" }), el("th", { class: "k-actions-col" }, el("span", { class: "sr-only", text: "Actions" })))),
      tbody)));
  }

  function fillSelect(select, items, label, value, placeholder) {
    const keep = select.value;
    select.replaceChildren(el("option", { value: "", text: placeholder }));
    for (const it of items) select.append(el("option", { value: String(value(it)), text: label(it) }));
    if ([...select.options].some((o) => o.value === keep)) select.value = keep;
  }

  function renderForms() {
    const sharedNames = new Set(state.shared.map((g) => g.vname));
    const isDefault = (v) => /(^|-)default$/.test(v.name);
    fillSelect($("gv-volume"), state.volumes.filter((v) => !isDefault(v)),
      (v) => (sharedNames.has(v.name) ? `${v.name} (shared)` : v.name), (v) => v.name, "Choose a volume");
    fillSelect($("gv-group"), state.groups.filter((g) => g.gid !== 0), (g) => `${g.groupname} (${g.gid})`, (g) => g.gid, "Choose a group");
    const personal = state.volumes.filter((v) => !sharedNames.has(v.name));
    fillSelect($("uq-volume"), personal, (v) => v.name, (v) => v.name, "Choose a volume");
    fillSelect($("uq-filter"), personal, (v) => v.name, (v) => v.name, "All volumes");
    fillSelect($("uq-user"), state.users.filter((u) => u.uid > 0), (u) => `${u.username} (${u.uid})`, (u) => u.uid, "Choose a user");
  }

  async function load() {
    const shared = $("gv-target");
    if (!shared) return;
    try {
      const [vols, groups, users, gvs] = await Promise.all([
        call("GET", "/fetch-volumes", { format: "json" }),
        call("GET", "/admin/fetch-groups", { format: "json" }),
        call("GET", "/admin/fetch-users", { format: "json" }),
        call("GET", "/admin/group-volumes"),
      ]);
      state.volumes = (vols && vols.volumes) || [];
      state.groups = groups || [];
      state.users = users || [];
      state.shared = gvs || [];
      renderForms();
      state.quotas = (await call("GET", "/admin/user-volumes", { volume: $("uq-filter").value })) || [];
      renderShared();
      renderQuotas();
    } catch (e) {
      for (const id of ["gv-target", "uq-target"]) {
        $(id).replaceChildren(stateBox("k-error-state", "fa-solid fa-triangle-exclamation", `Could not load quotas and sharing: ${e.message}`));
      }
    }
  }

  function bindForm(id, send, done) {
    const form = $(id);
    if (!form) return;
    form.addEventListener("submit", async (ev) => {
      ev.preventDefault();
      const data = Object.fromEntries(new FormData(form));
      if (data.quota === "") data.quota = "0";
      try { await send(data); toast(done(data), "success"); form.reset(); await load(); }
      catch (e) { toast(e.message); }
    });
  }

  document.addEventListener("DOMContentLoaded", () => {
    bindForm("gv-form", (d) => call("POST", "/admin/group-volumes", d), (d) => `${d.vname} is now shared.`);
    bindForm("uq-form", (d) => call("PATCH", "/admin/user-volumes", d), () => "Quota set.");
    $("uq-filter")?.addEventListener("change", load);
    $("quotas-reload")?.addEventListener("click", load);
  });
  // load when the page is opened
  document.addEventListener("click", (ev) => {
    if (ev.target.closest('[data-show-section="quotas"]')) load();
  });
})();
