const test = require("node:test");
const assert = require("node:assert");
const { load } = require("./load.js");

const TOKEN = "ab".repeat(32);

function setup() {
  const sent = [];
  let hook = null;
  function XHR() {}
  XHR.prototype.open = function () {};
  XHR.prototype.setRequestHeader = function (n, v) { sent.push([n.toLowerCase(), v]); };
  XHR.prototype.send = function () {};
  const fetched = [];
  const ctx = load("csrf.js", {
    location: { href: "http://h/api/v1/x", origin: "http://h" },
    document: { cookie: "a=1; csrf_token=" + TOKEN, addEventListener: (ev, fn) => { hook = fn; } },
    XMLHttpRequest: XHR,
    Headers: class { constructor(h) { this.h = new Map(Object.entries(h || {})); } set(k, v) { this.h.set(k, v); } get(k) { return this.h.get(k); } },
    fetch: async (input, init) => { fetched.push({ input, init }); return {}; },
  });
  return { ctx, sent, fetched, hook: () => hook };
}

test("htmx writes carry the token exactly once", () => {
  const { ctx, sent, hook } = setup();
  const detail = { verb: "post", headers: {} };
  hook()({ detail });
  assert.equal(detail.headers["X-CSRF-Token"], TOKEN);
  // htmx then sends through XHR with those headers
  const x = new ctx.XMLHttpRequest();
  x.open("POST", "/api/v1/verified/rm");
  for (const [n, v] of Object.entries(detail.headers)) x.setRequestHeader(n, v);
  x.send();
  assert.deepEqual(sent.filter(([n]) => n === "x-csrf-token"), [["x-csrf-token", TOKEN]], "sent twice gives the server 'tok, tok'");
});

test("htmx reads carry no token", () => {
  const { hook } = setup();
  const detail = { verb: "get", headers: {} };
  hook()({ detail });
  assert.equal(detail.headers["X-CSRF-Token"], undefined);
});

test("plain XHR writes get the token; reads and other origins don't", () => {
  const { ctx, sent } = setup();
  for (const [method, url] of [["DELETE", "/api/v1/verified/rm"], ["GET", "/api/v1/verified/x"], ["POST", "https://evil.example/x"]]) {
    const x = new ctx.XMLHttpRequest();
    x.open(method, url);
    x.send();
  }
  assert.equal(sent.filter(([n]) => n === "x-csrf-token").length, 1);
});

test("fetch writes to this origin get the token", async () => {
  const { ctx, fetched } = setup();
  await ctx.fetch("/api/v1/verified/upload", { method: "POST" });
  await ctx.fetch("/api/v1/verified/fetch-jobs");
  await ctx.fetch("https://evil.example/x", { method: "POST" });
  assert.equal(fetched[0].init.headers.get("X-CSRF-Token"), TOKEN);
  assert.equal(fetched[1].init.headers, undefined);
  assert.equal(fetched[2].init.headers, undefined);
});
