// DOM tests for admin-panel.js: the parts that build markup from server
// data (resource details, the user row editor, the job modal) must show it
// as text, and the row editor must restore exactly what it replaced.
const test = require("node:test");
const assert = require("node:assert/strict");
const { page } = require("./dom");

const EVIL = `"><img src=x onerror="window.pwned=1">`;
const processed = [];
const globals = {
  htmx: { process: (el) => processed.push(el) },
  getPreviewWindow: () => "0-4095", // index.js
};
async function panel(html) {
  const p = await page(html, ["admin-panel.js"], {
    globals, ready: false, routes: { "GET /conf": { ws_address: "localhost:8082" } },
  });
  assert.deepEqual(p.errors, [], "the page threw");

  return p;
}

function noInjection(w, root) {
  assert.equal(root.querySelector("img"), null, "markup from the data was parsed");
  assert.equal(w.pwned, undefined);
}

test("resource details show a hostile file name as text and encode it in URLs", async () => {
  const cells = ["7", `/docs/${EVIL}.txt`, "/docs", "lab", "text/plain", "12", "rw-r--r--",
    "2026-09-28T10:00:00Z", "2026-09-28T10:00:00Z", "2026-09-28T10:00:00Z", "1001", "1001", "3"];
  const { w, doc } = await panel(`<table id="t"><tbody><tr>${cells.map(() => "<td></td>").join("")}</tr></tbody></table><div id="resource-details"></div>`);
  const tr = doc.querySelector("#t tr");
  cells.forEach((c, i) => { tr.cells[i].textContent = c; }); // as the server-rendered, escaped row reads
  // jsdom has no layout, so innerText is textContent's stand-in
  for (const td of tr.cells) Object.defineProperty(td, "innerText", { get() { return this.textContent; } });

  const target = doc.getElementById("resource-details");
  w.parseAndInjectRTableRowdata(tr, target);
  noInjection(w, target);
  assert.equal(target.querySelector("h3").textContent, `/docs/${EVIL}.txt`);
  const del = new URL(target.querySelector(".r-btn-delete").getAttribute("hx-delete"), "http://x");
  assert.equal(del.searchParams.get("name"), `/docs/${EVIL}.txt`, "the name survives the round trip through the URL");
  assert.equal(del.searchParams.get("volume"), "lab");
  assert.match(target.querySelector(".r-btn-delete").getAttribute("hx-confirm"), /img src=x/, "the confirm text holds it literally");
  const facts = Object.fromEntries([...target.querySelectorAll("dl div")].map((d) => [d.querySelector("dt").textContent, d.querySelector("dd").textContent]));
  assert.equal(facts.Size, "12 B");
  assert.equal(facts.Owner, "1001");

  w.parseAndInjectRTableRowdata(doc.createElement("tr"), target); // not a resource row: ignored
  assert.ok(target.querySelector("h3"));
});

const userRow = (uid, name) => `<table><tbody><tr id="table-4">
  <td class="uid">${uid}</td><td class="name"><strong>${name}</strong></td><td class="k-hash"></td>
  <td class="email">a@example.com</td><td class="home">/home/a</td><td>1001</td>
  <td><span class="groups k-chips"><span class="k-chip">user</span></span></td>
  <td class="k-actions-col"><div class="k-actions-btns"><button id="edit-btn-4" data-edit-user data-uid="${uid}" data-index="4">Edit</button></div></td>
</tr></tbody></table>`;

test("editing a user row and cancelling restores it exactly", async () => {
  const { w, doc } = await panel(userRow("1001", "alice &lt;b&gt;"));
  const row = doc.getElementById("table-4");
  const before = [...row.cells].slice(0, -1).map((c) => c.innerHTML);

  w.editUser("1001", 4);
  assert.ok(row.classList.contains("is-editing"));
  const inputs = [...row.querySelectorAll("input.table-input")].map((i) => i.id);
  assert.deepEqual(inputs, [1, 2, 3, 4, 6].map((i) => `edit-input-1001-${i}`), "uid and primary group stay read-only");
  assert.equal(row.querySelector("#edit-input-1001-1").placeholder, "alice <b>");
  noInjection(w, row);
  const save = doc.getElementById("submit-btn-4");
  assert.equal(save.getAttribute("hx-patch"), "/api/v1/verified/admin/userpatch");
  assert.equal(save.getAttribute("hx-confirm"), "Are you sure you want to update user 1001?");
  assert.ok(processed.includes(save), "htmx wires the new Save button");

  doc.getElementById("edit-input-1001-3").value = "new@example.com";
  doc.getElementById("edit-input-1001-6").value = "user,physics";
  assert.deepEqual({ ...w.getUserPatchValues("1001") },
    { uid: "1001", username: "", password: "", info: "new@example.com", home: "", groups: "user,physics" });

  w.cancelEdit(4);
  assert.ok(!row.classList.contains("is-editing"));
  assert.deepEqual([...row.cells].slice(0, -1).map((c) => c.innerHTML), before);
  assert.ok(doc.getElementById("edit-btn-4"));
  assert.equal(doc.getElementById("delete-btn-4").getAttribute("hx-delete"), "/api/v1/verified/admin/userdel?uid=1001");
});

test("root's restored row has no delete button", async () => {
  const { w, doc } = await panel(userRow("0", "root"));
  w.editUser("0", 4);
  w.cancelEdit(4);
  assert.equal(doc.getElementById("delete-btn-4"), null);
});

test("the job modal carries hostile job fields as values, not markup", async () => {
  const { w, doc } = await panel(`<div id="parent"><div id="job">
    <span class="jid">42</span><span class="uid">1001</span><span class="status">${EVIL.replace(/</g, "&lt;")}</span>
    <span class="description">&lt;/textarea&gt;&lt;img src=x onerror="window.pwned=1"&gt;</span>
    <span class="logic" data-v="${EVIL.replace(/"/g, "&quot;").replace(/</g, "&lt;")}"></span>
    <span class="completed">true</span>
  </div></div>`);
  const parent = doc.getElementById("parent");
  w.modJobModal(doc.getElementById("job"), parent);
  const modal = parent.querySelector(":scope > .modal");
  noInjection(w, modal);
  const form = modal.querySelector("#modify-job-form");
  assert.equal(form.elements.jid.value, "42");
  assert.equal(form.elements.status.value, EVIL);
  assert.equal(form.elements.logic.value, EVIL, "data-v wins over the text");
  assert.equal(form.elements.description.value, `</textarea><img src=x onerror="window.pwned=1">`);
  assert.equal(form.elements.completed.checked, true);
  assert.ok(processed.includes(form));
});
