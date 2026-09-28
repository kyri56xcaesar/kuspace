// The whole console page: the real admin-panel.html (template actions
// stripped, so every section is present) with every page script in the
// page's order must boot without a script error.
const fs = require("node:fs");
const path = require("node:path");
const test = require("node:test");
const assert = require("node:assert/strict");
const { page } = require("./dom");

const template = fs.readFileSync(path.join(__dirname, "..", "templates", "admin-panel.html"), "utf8");
const body = template.match(/<body[^>]*>([\s\S]*)<\/body>/)[1]
  .replace(/\{\{[\s\S]*?\}\}/g, "")
  .replace(/<script[\s\S]*?<\/script>/g, "");
// the <script src> order of the page, vendor code included
const scripts = [...template.matchAll(/<script src="[^"]*\/js\/([^"]+)"/g)].map((m) => m[1]);

test("the admin panel boots without script errors", async () => {
  assert.ok(scripts.includes("index.js") && scripts.includes("quotas.js"), scripts.join(" "));
  const { w, errors, doc, settle } = await page(body, scripts, {
    routes: { "GET /conf": { ws_address: "localhost:8082" } },
  });
  await settle();
  assert.deepEqual(errors.map((e) => String(e.cause || e.message)), []);
  assert.ok(doc.getElementById("quotas"), "the quotas section is on the page");
  w.close(); // its timers
});
