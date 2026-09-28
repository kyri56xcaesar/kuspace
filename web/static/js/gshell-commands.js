// gshell-commands.js - the commands gShell understands. They call the same
// frontapp endpoints the pages use, as the logged-in user, and print plain
// text. "say" is handled by gshell.js (it goes to the shared room).
(function () {
  "use strict";
  const API = "/api/v1/verified";
  const MAX_CAT = 64 * 1024; // bytes shown by cat

  async function get(path, params) {
    const url = API + path + (params ? "?" + new URLSearchParams(params) : "");
    const r = await fetch(url, { credentials: "same-origin", headers: { Accept: "application/json" } });
    return r;
  }
  async function getJSON(path, params) {
    const r = await get(path, Object.assign({ format: "json" }, params || {}));
    let body = null;
    try { body = await r.json(); } catch (e) { /* not JSON */ }
    if (!r.ok) throw new Error((body && body.error) || `HTTP ${r.status}`);
    return body;
  }
  async function send(method, path, params) {
    const r = await fetch(API + path + "?" + new URLSearchParams(params), { method, credentials: "same-origin" });
    let body = null;
    try { body = await r.json(); } catch (e) { /* not JSON */ }
    if (!r.ok) throw new Error((body && body.error) || `HTTP ${r.status}`);
    return body;
  }

  const size = (n) => {
    n = Number(n) || 0;
    const u = ["B", "KB", "MB", "GB", "TB"];
    let i = 0;
    while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
    return (i ? n.toFixed(1) : String(n)) + " " + u[i];
  };
  const pad = (s, n) => String(s).padEnd(n);
  const path = (p) => (p.startsWith("/") ? p : "/" + p);

  const commands = {
    help: {
      usage: "help",
      about: "list the commands",
      run: async (_, out) => {
        for (const [name, c] of Object.entries(commands)) out(`${pad(c.usage, 28)} ${c.about}`);
        out(`${pad("say <message>", 28)} talk to everyone in the gShell room`);
        out(`${pad("clear", 28)} clear the screen`);
      },
    },
    whoami: {
      usage: "whoami",
      about: "who you are logged in as",
      run: async (_, out) => {
        const name = document.querySelector(".k-user-name")?.textContent?.trim() || "?";
        const uid = document.getElementById("dash")?.dataset.uid || "?";
        out(`${name} (uid ${uid})`);
      },
    },
    volumes: {
      usage: "volumes",
      about: "the volumes you can use",
      run: async (_, out) => {
        const d = await getJSON("/fetch-volumes");
        for (const v of (d && d.volumes) || []) {
          const quota = v.capacity > 0 ? `${Number(v.capacity).toFixed(2)} GB` : "unlimited";
          out(`${pad(v.name, 24)} ${pad(`${Number(v.usage || 0).toFixed(2)} GB of ${quota}`, 28)}${v.shared ? " shared (group " + v.gid + ")" : ""}`);
        }
      },
    },
    ls: {
      usage: "ls [path] [-v volume]",
      about: "list files (default: all your volumes)",
      run: async (args, out) => {
        const { rest, volume } = volumeFlag(args);
        const prefix = rest[0] ? path(rest[0]) : "/";
        const files = (await getJSON("/fetch-resources", { volume: volume || "*" })) || [];
        const shown = files.filter((f) => f.name.startsWith(prefix));
        if (!shown.length) { out("(no files)"); return; }
        for (const f of shown) out(`${f.perms || "?????????"}  ${pad(f.uid, 6)} ${pad(size(f.size), 10)} ${f.vname}${f.name}`);
      },
    },
    cat: {
      usage: "cat <path> [-v volume]",
      about: `print a text file (first ${MAX_CAT / 1024} KB)`,
      run: async (args, out) => {
        const { rest, volume } = volumeFlag(args);
        if (!rest[0]) throw new Error("usage: cat <path> [-v volume]");
        const params = { target: path(rest[0]) };
        if (volume) params.volume = volume;
        const r = await get("/download", params);
        if (!r.ok) {
          let msg = `HTTP ${r.status}`;
          try { msg = (await r.json()).error || msg; } catch (e) { /* not JSON */ }
          throw new Error(msg);
        }
        const text = await r.text();
        for (const line of text.slice(0, MAX_CAT).split("\n")) out(line);
        if (text.length > MAX_CAT) out(`... (${size(text.length)}; download it for the rest)`);
      },
    },
    rm: {
      usage: "rm <path> [-v volume]",
      about: "delete a file",
      run: async (args, out) => {
        const { rest, volume } = volumeFlag(args);
        if (!rest[0]) throw new Error("usage: rm <path> [-v volume]");
        const params = { name: path(rest[0]) };
        if (volume) params.volume = volume;
        await send("DELETE", "/rm", params);
        out(`removed ${params.name}`);
      },
    },
    jobs: {
      usage: "jobs",
      about: "your jobs, newest first",
      run: async (_, out) => {
        const d = await getJSON("/fetch-jobs");
        const jobs = (d && d.content) || [];
        if (!jobs.length) { out("(no jobs)"); return; }
        for (const j of jobs.slice(0, 50)) out(`#${pad(j.jid, 6)} ${pad(j.status || "?", 10)} ${pad(j.logic || "", 16)} ${j.input || ""} -> ${j.output || ""}`);
      },
    },
    log: {
      usage: "log <jid>",
      about: "the saved output of a job",
      run: async (args, out) => {
        if (!/^\d+$/.test(args[0] || "")) throw new Error("usage: log <jid>");
        const r = await get("/job-log", { jid: args[0] });
        const text = await r.text();
        if (!r.ok) throw new Error(text || `HTTP ${r.status}`);
        for (const line of text.split("\n")) out(line);
      },
    },
    cancel: {
      usage: "cancel <jid>",
      about: "stop a queued or running job",
      run: async (args, out) => {
        if (!/^\d+$/.test(args[0] || "")) throw new Error("usage: cancel <jid>");
        await send("POST", "/job-cancel", { jid: args[0] });
        out(`job ${args[0]} cancelled`);
      },
    },
  };

  function volumeFlag(args) {
    const rest = [];
    let volume = "";
    for (let i = 0; i < args.length; i++) {
      if (args[i] === "-v" && i + 1 < args.length) volume = args[++i];
      else rest.push(args[i]);
    }
    return { rest, volume };
  }

  // split a line into words; "double quotes" keep spaces
  function words(line) {
    const out = [];
    const re = /"([^"]*)"|(\S+)/g;
    let m;
    while ((m = re.exec(line)) !== null) out.push(m[1] !== undefined ? m[1] : m[2]);
    return out;
  }

  // gshellRun runs one command line, printing through out(text).
  window.gshellRun = async function (line, out) {
    const [name, ...args] = words(line.trim());
    if (!name) return;
    const cmd = commands[name];
    if (!cmd) { out(`${name}: unknown command (try "help")`); return; }
    try { await cmd.run(args, out); }
    catch (e) { out(`${name}: ${e.message}`); }
  };
  window.gshellWords = words;
})();
