// DOM tests for quotas.js, the "Quotas & sharing" admin page, on the real
// markup from admin-panel.html.
const test = require("node:test");
const assert = require("node:assert/strict");
const { page, section } = require("./dom");

const EVIL = `<img src=x onerror="window.pwned=1">`;

function routes(over = {}) {
  return {
    "GET /api/v1/verified/fetch-volumes": { volumes: [
      { name: "uspace-default", capacity: 100 },
      { name: "lab", capacity: 50 },
      { name: "scratch", capacity: 20 },
      { name: EVIL, capacity: 1 },
    ] },
    "GET /api/v1/verified/admin/fetch-groups": [
      { gid: 0, groupname: "admin" }, { gid: 1000, groupname: "user" }, { gid: 1500, groupname: "physics" },
    ],
    "GET /api/v1/verified/admin/fetch-users": [
      { uid: 0, username: "root" }, { uid: 1001, username: "alice" }, { uid: 1002, username: EVIL },
    ],
    "GET /api/v1/verified/admin/group-volumes": [{ vname: "lab", gid: 1500, quota: 10, usage: 12.5 }],
    "GET /api/v1/verified/admin/user-volumes": [
      { vname: "scratch", uid: 1001, quota: 2, usage: 0.5 },
      { vname: EVIL, uid: 1002, quota: 0, usage: 3 },
    ],
    ...over,
  };
}

async function opened(r = routes()) {
  const p = await page(section("admin-panel.html", "quotas"), ["quotas.js"], { routes: r });
  p.doc.getElementById("quotas-reload").click();
  await p.settle();
  assert.deepEqual(p.errors, [], "the page threw");

  return p;
}

const options = (doc, id) => [...doc.getElementById(id).options].map((o) => o.value);
const rows = (doc, id) => [...doc.querySelectorAll(`#${id} tbody tr`)];

test("shared volumes and personal quotas are listed with names, usage and meters", async () => {
  const { doc } = await opened();
  const [lab] = rows(doc, "gv-target");
  assert.equal(rows(doc, "gv-target").length, 1);
  assert.match(lab.textContent, /lab/);
  assert.match(lab.textContent, /physics/, "the group is shown by name");
  assert.match(lab.textContent, /12\.50 GB of 10\.00 GB/);
  assert.ok(lab.querySelector("progress.k-meter.is-over"), "usage over the quota is marked");

  const quotas = rows(doc, "uq-target");
  assert.equal(quotas.length, 2);
  assert.match(quotas[0].textContent, /alice/);
  assert.match(quotas[0].textContent, /0\.50 GB of 2\.00 GB/);
  assert.match(quotas[1].textContent, /of unlimited/, "quota 0 is unlimited");
  assert.ok(quotas[1].querySelector(".k-meter-unbounded"));
});

test("names from the server are text, never markup", async () => {
  const { w, doc } = await opened();
  assert.equal(doc.querySelector("#quotas img"), null);
  assert.equal(w.pwned, undefined);
  assert.ok(rows(doc, "uq-target")[1].textContent.includes(EVIL), "shown literally");
  assert.ok(options(doc, "uq-user").includes("1002"));
});

test("the pickers offer only what can be chosen", async () => {
  const { doc } = await opened();
  assert.deepEqual(options(doc, "gv-volume"), ["", "lab", "scratch", EVIL], "the default volume can't be shared");
  assert.equal(doc.querySelector('#gv-volume option[value="lab"]').textContent, "lab (shared)");
  assert.deepEqual(options(doc, "gv-group"), ["", "1000", "1500"], "not the admin group");
  assert.deepEqual(options(doc, "uq-volume"), ["", "uspace-default", "scratch", EVIL], "shared volumes have no personal quotas");
  assert.deepEqual(options(doc, "uq-user"), ["", "1001", "1002"], "not root");
});

test("each action makes the call it names", async () => {
  const saved = [];
  const r = routes({
    "POST /api/v1/verified/admin/group-volumes": () => (saved.push("gv"), { message: "ok" }),
    "PATCH /api/v1/verified/admin/user-volumes": () => (saved.push("uq"), { message: "ok" }),
    "DELETE /api/v1/verified/admin/group-volumes": () => (saved.push("stop"), { message: "ok" }),
    "DELETE /api/v1/verified/admin/user-volumes": () => (saved.push("reset"), { message: "ok" }),
  });
  const { doc, calls, toasts, settle } = await opened(r);
  const last = () => calls.filter((c) => c.method !== "GET").at(-1);

  // group quota: edit and save in the table
  const lab = rows(doc, "gv-target")[0];
  lab.querySelector(".k-quota-input").value = "25";
  lab.querySelector(".k-quota-edit button").click();
  await settle();
  assert.deepEqual(last(), { method: "POST", path: "/api/v1/verified/admin/group-volumes", query: {}, body: { vname: "lab", gid: "1500", quota: "25" } });

  // stop sharing: the volume goes in the query
  [...lab.querySelectorAll("button")].find((b) => b.textContent === "Stop sharing").click();
  await settle();
  assert.deepEqual(last(), { method: "DELETE", path: "/api/v1/verified/admin/group-volumes", query: { volume: "lab" }, body: {} });

  // reset a personal quota
  [...rows(doc, "uq-target")[0].querySelectorAll("button")].find((b) => b.textContent === "Reset").click();
  await settle();
  assert.deepEqual(last(), { method: "DELETE", path: "/api/v1/verified/admin/user-volumes", query: { volume: "scratch", uid: "1001" }, body: {} });

  // share a volume through the form; an empty quota is sent as 0
  doc.getElementById("gv-volume").value = "scratch";
  doc.getElementById("gv-group").value = "1000";
  doc.getElementById("gv-form").requestSubmit();
  await settle();
  assert.deepEqual(last(), { method: "POST", path: "/api/v1/verified/admin/group-volumes", query: {}, body: { vname: "scratch", gid: "1000", quota: "0" } });

  // set a personal quota through the form
  doc.getElementById("uq-user").value = "1001";
  doc.getElementById("uq-volume").value = "uspace-default";
  doc.getElementById("uq-quota").value = "3.5";
  doc.getElementById("uq-form").requestSubmit();
  await settle();
  assert.deepEqual(last(), { method: "PATCH", path: "/api/v1/verified/admin/user-volumes", query: {}, body: { uid: "1001", vname: "uspace-default", quota: "3.5" } });

  assert.deepEqual(saved, ["gv", "stop", "reset", "gv", "uq"]);
  assert.ok(toasts.every((t) => t.kind === "success"), JSON.stringify(toasts));
});

test("the volume filter asks for that volume's quotas", async () => {
  const { doc, calls, settle } = await opened();
  const filter = doc.getElementById("uq-filter");
  filter.value = "scratch";
  filter.dispatchEvent(new doc.defaultView.Event("change"));
  await settle();
  const asked = calls.filter((c) => c.path === "/api/v1/verified/admin/user-volumes").at(-1);
  assert.deepEqual(asked.query, { volume: "scratch" });
  assert.equal(filter.value, "scratch", "the choice survives the reload");
});

test("failures are shown: a refused save as a toast, a failed load in place", async () => {
  const r = routes({ "PATCH /api/v1/verified/admin/user-volumes": { status: 400, body: { error: "quota must not be negative" } } });
  const p = await opened(r);
  const row = rows(p.doc, "uq-target")[0];
  row.querySelector(".k-quota-input").value = "-1";
  row.querySelector(".k-quota-edit button").click();
  await p.settle();
  assert.deepEqual(p.toasts.at(-1), { msg: "quota must not be negative", kind: "error" });

  const down = await opened(routes({ "GET /api/v1/verified/admin/group-volumes": { status: 503, body: { error: "uspace is unavailable" } } }));
  for (const id of ["gv-target", "uq-target"]) {
    const box = down.doc.querySelector(`#${id} .k-error-state`);
    assert.ok(box, id);
    assert.match(box.textContent, /uspace is unavailable/);
  }
});
