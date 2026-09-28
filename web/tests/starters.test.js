// Every language's editor starter must actually run: each is executed with
// its language's command (code_modes.json, kept in sync with uspace by a Go
// test) against a stand-in for the presigned INPUT_URL/OUTPUT_URL. A
// language whose interpreter isn't installed here is skipped.
const test = require("node:test");
const assert = require("node:assert");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const http = require("node:http");
const { execFile, execFileSync } = require("node:child_process");
const { load } = require("./load.js");

const modes = JSON.parse(fs.readFileSync(path.join(__dirname, "code_modes.json"), "utf8"));
const { languageStarters } = load("jobCodeInput.js", {}, ["languageStarters"]);

// the tool each command needs on this machine
const needs = { python: "python3", node: "node", ruby: "ruby", php: "php", perl: "perl", r: "Rscript", go: "go", java: "java", c: "gcc" };
const has = (tool) => { try { execFileSync("sh", ["-c", `command -v ${tool}`], { stdio: "ignore" }); return true; } catch { return false; } };

function fakeStorage() {
  const store = { "/in": Buffer.from("hello kuspace\n") };
  const server = http.createServer((req, res) => {
    const key = req.url.split("?")[0];
    if (req.method === "GET") {
      if (!store[key]) { res.writeHead(404).end(); return; }
      res.writeHead(200, { "Content-Length": store[key].length }).end(store[key]);
      return;
    }
    const chunks = [];
    req.on("data", (c) => chunks.push(c));
    req.on("end", () => { store[key] = Buffer.concat(chunks); res.writeHead(200).end(); });
  });
  return new Promise((resolve) => server.listen(0, "127.0.0.1", () => resolve({ server, store, port: server.address().port })));
}

test("every language has a starter", () => {
  assert.deepEqual(Object.keys(languageStarters).sort(), Object.keys(modes).sort());
});

for (const [lang, mode] of Object.entries(modes)) {
  const tool = needs[lang];
  test(`${lang} starter reads the input and writes the output`, { skip: !has(tool) && `${tool} not installed` }, async () => {
    const { server, store, port } = await fakeStorage();
    const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "kuspace-starter-"));
    try {
      const base = `http://127.0.0.1:${port}`;
      const run = mode.run.replaceAll("/tmp/", tmp + "/");
      await new Promise((resolve, reject) => {
        execFile("sh", ["-c", run], {
          env: { ...process.env, LOGIC: languageStarters[lang], INPUT_URL: `${base}/in?X-Amz-Signature=x`, OUTPUT_URL: `${base}/out?X-Amz-Signature=x`, GOCACHE: path.join(tmp, "gocache") },
          timeout: 120000,
        }, (err, stdout, stderr) => (err ? reject(new Error(`${err.message}\n${stderr}`)) : resolve()));
      });
      assert.equal(String(store["/out"] || ""), "HELLO KUSPACE\n");
    } finally {
      server.close();
      fs.rmSync(tmp, { recursive: true, force: true });
    }
  });
}
