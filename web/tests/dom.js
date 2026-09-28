// dom.js - a jsdom page for DOM-level tests: the given markup, the given
// scripts from web/static/js run as classic scripts (sharing one global
// scope, as on the real page), a fake fetch and recorders for what the page
// tells the user. The page's console is quiet; its uncaught errors are kept
// in errors.
const fs = require("node:fs");
const path = require("node:path");
const { JSDOM, VirtualConsole } = require("jsdom");

const js = (file) => fs.readFileSync(path.join(__dirname, "..", "static", "js", file), "utf8");

// section returns the element with the given id from a template in
// web/templates (Go template actions stripped), so tests use the real markup.
function section(template, id) {
  const src = fs.readFileSync(path.join(__dirname, "..", "templates", template), "utf8");
  const m = src.match(new RegExp(`<section id="${id}"[\\s\\S]*?</section>`));
  if (!m) throw new Error(`no section #${id} in ${template}`);

  return m[0].replace(/\{\{[\s\S]*?\}\}/g, "");
}

// page builds the page and runs the scripts. routes maps "METHOD /path" to
// a response body (status 200) or to {status, body}; unknown routes are 404.
// ready: false skips DOMContentLoaded (for scripts whose page setup needs
// the whole panel, when a test calls their functions directly).
async function page(html, scripts, { routes = {}, globals = {}, ready = true } = {}) {
  const errors = [];
  const virtualConsole = new VirtualConsole();
  virtualConsole.on("jsdomError", (e) => errors.push(e));
  const dom = new JSDOM(`<!doctype html><html><body>${html}</body></html>`, {
    runScripts: "outside-only",
    virtualConsole,
    url: "http://kuspace.test/api/v1/verified/admin-panel",
  });
  const w = dom.window;
  const calls = [];
  const toasts = [];
  w.fetch = async (url, init = {}) => {
    const u = new URL(url, w.location.href);
    const method = init.method || "GET";
    const body = init.body ? String(init.body) : "";
    calls.push({ method, path: u.pathname, query: Object.fromEntries(u.searchParams), body: Object.fromEntries(new URLSearchParams(body)) });
    let r = routes[`${method} ${u.pathname}`];
    if (typeof r === "function") r = r(u, init);
    if (r === undefined) r = { status: 404, body: { error: "no such route in the test" } };
    if (r.status === undefined) r = { status: 200, body: r };
    const text = JSON.stringify(r.body);

    return { ok: r.status < 300, status: r.status, json: async () => JSON.parse(text), text: async () => text };
  };
  fillBrowserGaps(w);
  w.kToast = (msg, kind) => toasts.push({ msg, kind: kind || "error" });
  w.confirm = () => true;
  w.alert = (msg) => toasts.push({ msg, kind: "alert" });
  Object.assign(w, globals);
  const listen = w.document.addEventListener.bind(w.document);
  if (!ready) {
    w.document.addEventListener = (type, ...rest) => type === "DOMContentLoaded" || listen(type, ...rest);
  }
  for (const f of scripts) w.eval(js(f));
  if (ready && w.document.readyState === "loading") {
    await new Promise((resolve) => listen("DOMContentLoaded", resolve));
  } else if (ready) {
    w.document.dispatchEvent(new w.Event("DOMContentLoaded"));
  }
  await settle();

  return { w, doc: w.document, calls, toasts, errors, settle };
}

// fillBrowserGaps adds what browsers have and jsdom doesn't (it has no
// layout), as no-ops, so the page's own code is what a test exercises.
function fillBrowserGaps(w) {
  w.matchMedia = () => ({ matches: false, addEventListener() {}, removeEventListener() {}, addListener() {}, removeListener() {} });
  w.Element.prototype.scrollTo = function () {};
  w.Element.prototype.scrollIntoView = function () {};
  const rect = () => ({ x: 0, y: 0, top: 0, left: 0, right: 0, bottom: 0, width: 0, height: 0 });
  w.Range.prototype.getBoundingClientRect = rect;
  w.Range.prototype.getClientRects = () => [];
  // browsers default an XPath result type to ANY_TYPE (0); jsdom requires
  // one (htmx compiles an expression and evaluates it without)
  const evaluate = w.document.evaluate.bind(w.document);
  w.document.evaluate = (expr, node, ns, type, result) => evaluate(expr, node, ns ?? null, type ?? 0, result ?? null);
  const expr = Object.getPrototypeOf(w.document.createExpression("."));
  const run = expr.evaluate;
  expr.evaluate = function (node, type, result) { return run.call(this, node, type ?? 0, result ?? null); };
}

// settle lets pending fetches and their handlers finish.
async function settle() {
  for (let i = 0; i < 20; i++) await new Promise((r) => setImmediate(r));
}

module.exports = { page, section, settle };
