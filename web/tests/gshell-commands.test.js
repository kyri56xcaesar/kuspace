const test = require("node:test");
const assert = require("node:assert");
const { load } = require("./load.js");

function setup(replies) {
  const calls = [];
  const ctx = load("gshell-commands.js", {
    document: {
      querySelector: () => ({ textContent: "alice" }),
      getElementById: () => ({ dataset: { uid: "1001" } }),
    },
    fetch: async (url, init) => {
      const u = new URL(url, "http://h");
      calls.push(((init && init.method) || "GET") + " " + u.pathname + u.search);
      const r = replies[u.pathname] || { status: 404, json: { error: "nope" } };
      return {
        ok: !r.status || r.status < 300,
        status: r.status || 200,
        json: async () => r.json,
        text: async () => (r.text !== undefined ? r.text : JSON.stringify(r.json)),
      };
    },
  });
  const run = async (line) => {
    const out = [];
    await ctx.window.gshellRun(line, (t) => out.push(t));
    return out;
  };
  return { run, calls };
}

const replies = {
  "/api/v1/verified/fetch-resources": { json: [
    { name: "/docs/a.txt", vname: "vol", uid: 1001, size: 2048, perms: "rw-r-----" },
    { name: "/b.txt", vname: "vol", uid: 1001, size: 3, perms: "rw-r-----" },
  ] },
  "/api/v1/verified/download": { text: "line one\nline two" },
  "/api/v1/verified/fetch-jobs": { json: { content: [{ jid: 7, status: "completed", logic: "python", input: "vol/in", output: "vol/out" }] } },
  "/api/v1/verified/fetch-volumes": { json: { volumes: [{ name: "team", usage: 1, capacity: 2, shared: true, gid: 500 }] } },
  "/api/v1/verified/rm": { status: 403, json: { error: "no write access" } },
};

test("ls filters by path prefix", async () => {
  const { run } = setup(replies);
  const out = await run("ls docs");
  assert.equal(out.length, 1);
  assert.match(out[0], /vol\/docs\/a\.txt/);
  assert.match(out[0], /2\.0 KB/);
});

test("cat sends quoted names and the volume, prints lines", async () => {
  const { run, calls } = setup(replies);
  assert.deepEqual(await run('cat "a b.txt" -v vol'), ["line one", "line two"]);
  assert.ok(calls.some((c) => c.includes("target=%2Fa+b.txt") && c.includes("volume=vol")), calls.join("\n"));
});

test("errors are printed with the command name", async () => {
  const { run, calls } = setup(replies);
  assert.deepEqual(await run("rm b.txt"), ["rm: no write access"]);
  assert.ok(calls.some((c) => c.startsWith("DELETE /api/v1/verified/rm?name=%2Fb.txt")));
});

test("jobs, volumes, whoami", async () => {
  const { run } = setup(replies);
  assert.match((await run("jobs"))[0], /#7 +completed/);
  assert.match((await run("volumes"))[0], /team .* shared \(group 500\)/);
  assert.deepEqual(await run("whoami"), ["alice (uid 1001)"]);
});

test("usage and unknown commands", async () => {
  const { run, calls } = setup(replies);
  assert.deepEqual(await run("log x"), ["log: usage: log <jid>"]);
  assert.deepEqual(await run("cancel"), ["cancel: usage: cancel <jid>"]);
  assert.match((await run("frobnicate"))[0], /unknown command/);
  assert.equal(calls.length, 0, "invalid input must not reach the server");
  const help = await run("help");
  assert.ok(help.some((l) => l.startsWith("ls")) && help.some((l) => l.startsWith("say")));
});
