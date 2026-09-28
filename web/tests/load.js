// load.js - runs a browser script from web/static/js in an isolated context
// with only the globals a test provides.
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");

function load(file, globals, expose = []) {
  const src = fs.readFileSync(path.join(__dirname, "..", "static", "js", file), "utf8");
  const ctx = vm.createContext({ URL, URLSearchParams, console, setTimeout, clearTimeout, ...globals });
  ctx.window = ctx.window || ctx;
  // top-level const/let aren't context properties: hand the named ones out
  const tail = expose.map((n) => `globalThis.${n} = ${n};`).join("\n");
  vm.runInContext(src + "\n" + tail, ctx, { filename: file });

  return ctx;
}

module.exports = { load };
