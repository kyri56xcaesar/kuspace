/*
 * kuspace dashboard: jobs, storage, applications ("helm") and platform totals.
 * Everything is drawn from the same JSON endpoints the rest of the panel uses
 * (?format=json), as plain SVG - no chart library.
 */
(() => {
  const API = "/api/v1/verified";
  const DAYS = 14;
  const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

  // Job states use the fixed status palette and always ship with icon + label.
  const STATES = [
    { key: "completed", label: "Completed", icon: "fa-circle-check", cls: "st-good" },
    { key: "running", label: "Running", icon: "fa-spinner", cls: "st-run" },
    { key: "pending", label: "Pending", icon: "fa-hourglass-half", cls: "st-warn" },
    { key: "failed", label: "Failed", icon: "fa-circle-xmark", cls: "st-crit" },
  ];

  const esc = (v) =>
    String(v ?? "").replace(/[&<>"']/g, (ch) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[ch]);
  const asList = (x) => (Array.isArray(x) ? x : (x && Array.isArray(x.content) ? x.content : []));
  const fmtInt = (n) => Number(n).toLocaleString();

  async function getJSON(path) {
    try {
      const r = await fetch(API + path, { credentials: "same-origin", headers: { Accept: "application/json" } });
      if (!r.ok) return null;
      return await r.json();
    } catch {
      return null;
    }
  }

  // "2026-09-27 17:17:12+00:00" and RFC3339 both parse once the space is a T.
  function parseTime(s) {
    if (!s) return null;
    const d = new Date(String(s).replace(" ", "T"));
    return isNaN(d) ? null : d;
  }

  function ago(d) {
    if (!d) return "";
    const s = Math.round((Date.now() - d.getTime()) / 1000);
    if (s < 60) return "just now";
    const units = [["day", 86400], ["hour", 3600], ["minute", 60]];
    for (const [name, sec] of units) {
      const n = Math.floor(s / sec);
      if (n >= 1) return `${n} ${name}${n > 1 ? "s" : ""} ago`;
    }
    return "";
  }

  function fmtBytes(b) {
    if (!b) return "0 B";
    const u = ["B", "KB", "MB", "GB", "TB"];
    const i = Math.min(u.length - 1, Math.floor(Math.log(b) / Math.log(1024)));
    const v = b / 1024 ** i;
    return `${v >= 10 || i === 0 ? Math.round(v) : v.toFixed(1)} ${u[i]}`;
  }

  function stateOf(job) {
    const s = String(job.status || "").toLowerCase();
    if (s.startsWith("complet") || s === "succeeded" || s === "success") return "completed";
    if (s.startsWith("run") || s === "active" || s === "dispatched") return "running";
    if (s.startsWith("fail") || s === "error" || s === "canceled" || s === "cancelled") return "failed";
    return "pending";
  }
  const stateMeta = (key) => STATES.find((s) => s.key === key);

  /* ---------- shared tooltip (hover + keyboard focus) ---------- */
  const tip = document.createElement("div");
  tip.className = "dash-tip";
  tip.setAttribute("role", "tooltip");
  tip.hidden = true;

  function bindTips(root) {
    const show = (el) => {
      const text = el.getAttribute("data-tip");
      if (!text) return;
      tip.textContent = text;
      tip.hidden = false;
      const r = el.getBoundingClientRect();
      const t = tip.getBoundingClientRect();
      let x = r.left + r.width / 2 - t.width / 2;
      x = Math.max(8, Math.min(x, window.innerWidth - t.width - 8));
      let y = r.top - t.height - 8;
      if (y < 8) y = r.bottom + 8;
      tip.style.left = `${x}px`;
      tip.style.top = `${y}px`;
    };
    const hide = () => { tip.hidden = true; };
    root.querySelectorAll("[data-tip]").forEach((el) => {
      el.addEventListener("mouseenter", () => show(el));
      el.addEventListener("mouseleave", hide);
      el.addEventListener("focus", () => show(el));
      el.addEventListener("blur", hide);
    });
  }

  function tableTwin(caption, head, rows) {
    return `<details class="dash-table-twin"><summary>Show as table</summary>
      <table><caption class="sr-only">${esc(caption)}</caption>
      <thead><tr>${head.map((h) => `<th scope="col">${esc(h)}</th>`).join("")}</tr></thead>
      <tbody>${rows.map((r) => `<tr>${r.map((c) => `<td>${esc(c)}</td>`).join("")}</tr>`).join("")}</tbody>
      </table></details>`;
  }

  /* ---------- stat tiles ---------- */
  function tiles(el, items) {
    el.innerHTML = items
      .map(
        (t) => `<div class="dash-tile ${t.cls || ""}">
          <span class="dash-tile-label">${t.icon ? `<i class="fa-solid ${t.icon}" aria-hidden="true"></i> ` : ""}${esc(t.label)}</span>
          <span class="dash-tile-value">${esc(t.value)}</span>
        </div>`
      )
      .join("");
  }

  /* ---------- jobs: status part-to-whole bar ---------- */
  function statusBar(el, counts, total) {
    if (!total) {
      el.innerHTML = `<p class="dash-empty">No jobs yet. <a href="#" data-action="new-job">Submit your first job</a> to see it here.</p>`;
      return;
    }
    const segs = STATES.filter((s) => counts[s.key] > 0);
    const W = 100;
    let x = 0;
    const gap = 0.6; // ~2px at typical card widths - surface gap between fills
    const rects = segs
      .map((s, i) => {
        const w = (counts[s.key] / total) * W;
        const pct = Math.round((counts[s.key] / total) * 100);
        const rx = x;
        x += w;
        const ww = Math.max(0, w - (i < segs.length - 1 ? gap : 0));
        return `<rect class="${s.cls}" x="${rx}" y="0" width="${ww}" height="10" rx="1.2"
          tabindex="0" data-tip="${esc(`${s.label}: ${counts[s.key]} (${pct}%)`)}"
          aria-label="${esc(`${s.label}: ${counts[s.key]} jobs, ${pct} percent`)}"></rect>`;
      })
      .join("");
    const legend = STATES.map(
      (s) => `<li class="${counts[s.key] ? "" : "is-zero"}"><span class="dash-swatch ${s.cls}" aria-hidden="true"></span>
        ${s.label} <strong>${counts[s.key] || 0}</strong></li>`
    ).join("");
    el.innerHTML = `
      <svg class="dash-statusbar" viewBox="0 0 100 10" preserveAspectRatio="none" role="img"
        aria-label="Jobs by status">${rects}</svg>
      <ul class="dash-legend">${legend}</ul>
      ${tableTwin("Jobs by status", ["Status", "Jobs"], STATES.map((s) => [s.label, counts[s.key] || 0]))}`;
  }

  /* ---------- jobs: submissions per day (columns) ---------- */
  function dayColumns(el, jobs) {
    const today = new Date();
    today.setHours(0, 0, 0, 0);
    const days = [];
    for (let i = DAYS - 1; i >= 0; i--) {
      const d = new Date(today);
      d.setDate(today.getDate() - i);
      days.push({ d, n: 0 });
    }
    jobs.forEach((j) => {
      const t = parseTime(j.createdAt);
      if (!t) return;
      const k = new Date(t);
      k.setHours(0, 0, 0, 0);
      const idx = Math.round((k - days[0].d) / 86400000);
      if (idx >= 0 && idx < DAYS) days[idx].n++;
    });
    const max = Math.max(1, ...days.map((x) => x.n));
    const niceMax = max <= 4 ? max : Math.ceil(max / 5) * 5;
    const W = 560, H = 150, padL = 28, padB = 22, padT = 8;
    const plotW = W - padL, plotH = H - padB - padT;
    const bw = plotW / DAYS;
    const fmtDay = (d) => d.toLocaleDateString(undefined, { day: "numeric", month: "short" });
    const grid = [0, niceMax / 2, niceMax]
      .map((v) => {
        const y = padT + plotH - (v / niceMax) * plotH;
        return `<line class="dash-gridline" x1="${padL}" x2="${W}" y1="${y}" y2="${y}"></line>
          <text class="dash-axis" x="${padL - 6}" y="${y + 3}" text-anchor="end">${Number.isInteger(v) ? v : ""}</text>`;
      })
      .join("");
    const bars = days
      .map((x, i) => {
        const h = (x.n / niceMax) * plotH;
        const bx = padL + i * bw + 2; // 2px surface gap each side
        const w = Math.max(1, bw - 4);
        const label = `${fmtDay(x.d)}: ${x.n} job${x.n === 1 ? "" : "s"}`;
        const bar = x.n
          ? `<path class="dash-col" d="M${bx},${padT + plotH} v${-(h - 3)} q0,-3 3,-3 h${w - 6} q3,0 3,3 v${h - 3} z"></path>`
          : "";
        // hit target spans the whole column slot, bigger than the mark
        return `<g class="dash-col-g" tabindex="0" data-tip="${esc(label)}" aria-label="${esc(label)}">
          <rect class="dash-hit" x="${padL + i * bw}" y="${padT}" width="${bw}" height="${plotH}"></rect>${bar}</g>`;
      })
      .join("");
    const ticks = days
      .map((x, i) => (i % 3 === 1 || i === DAYS - 1
        ? `<text class="dash-axis" x="${padL + i * bw + bw / 2}" y="${H - 6}" text-anchor="middle">${i === DAYS - 1 ? "Today" : esc(fmtDay(x.d))}</text>`
        : ""))
      .join("");
    el.innerHTML = `
      <svg class="dash-days" viewBox="0 0 ${W} ${H}" role="img" aria-label="Jobs submitted per day over the last ${DAYS} days">
        ${grid}<line class="dash-baseline" x1="${padL}" x2="${W}" y1="${padT + plotH}" y2="${padT + plotH}"></line>
        ${bars}${ticks}
      </svg>
      ${tableTwin("Jobs submitted per day", ["Day", "Jobs"], days.map((x) => [fmtDay(x.d), x.n]))}`;
  }

  /* ---------- storage meter ---------- */
  function storage(el, resources, uid, quotaGB) {
    const mine = resources.filter((r) => Number(r.uid) === uid && (r.type || "file") !== "dir");
    const used = mine.reduce((a, r) => a + (Number(r.size) || 0), 0);
    const quota = (Number(quotaGB) || 0) * 1024 ** 3;
    const pct = quota ? (used / quota) * 100 : 0;
    const shown = used > 0 ? Math.max(pct, 0.8) : 0; // keep a non-zero use visible
    el.innerHTML = `
      <p class="dash-hero"><span class="dash-hero-value">${fmtBytes(used)}</span>
        <span class="dash-hero-unit">${quota ? `of ${fmtBytes(quota)}` : "used"}</span></p>
      ${quota ? `<div class="dash-meter" role="meter" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${pct.toFixed(1)}"
          aria-label="Storage used: ${pct.toFixed(1)} percent" tabindex="0"
          data-tip="${esc(`${fmtBytes(used)} of ${fmtBytes(quota)} (${pct < 0.1 && used ? "<0.1" : pct.toFixed(1)}%)`)}">
          <span style="width:${Math.min(100, shown)}%"></span></div>` : ""}
      <p class="dash-note">${fmtInt(mine.length)} file${mine.length === 1 ? "" : "s"} you own${quota ? " · default per-user quota" : ""}</p>`;
  }

  /* ---------- recent jobs ---------- */
  function recent(el, jobs) {
    const items = jobs
      .map((j) => ({ j, t: parseTime(j.createdAt) }))
      .sort((a, b) => (b.t || 0) - (a.t || 0))
      .slice(0, 5);
    if (!items.length) {
      el.innerHTML = `<li class="dash-empty">Jobs you submit will be listed here.</li>`;
      return;
    }
    el.innerHTML = items
      .map(({ j, t }) => {
        const st = stateMeta(stateOf(j));
        return `<li><a href="#" data-action="jobs" class="dash-recent-row">
          <span class="dash-recent-id">#${esc(j.jid)}</span>
          <span class="dash-recent-app">${esc(j.logic || "custom")}</span>
          <span class="dash-state ${st.cls}"><i class="fa-solid ${st.icon}" aria-hidden="true"></i> ${st.label}</span>
          <time datetime="${t ? t.toISOString() : ""}">${esc(ago(t))}</time>
        </a></li>`;
      })
      .join("");
  }

  /* ---------- applications helm ---------- */
  // Kubernetes is Greek for "helmsman": installed apps sit on the spokes of a
  // ship's wheel. Choosing one turns the wheel to bring it to the top.
  function helm(el, apps) {
    if (!apps.length) {
      el.innerHTML = `<p class="dash-empty">No applications are installed. An admin can add them under Job Browser → Apps.</p>`;
      return;
    }
    const n = apps.length;
    const C = 150, R = 100, hub = 26, knob = 14;
    const knobY = C - R - 8 - knob; // knob centre on the (unrotated) top spoke
    const step = 360 / n;
    const spokes = apps
      .map((a, i) => `<g class="helm-spoke" transform="rotate(${i * step} ${C} ${C})">
          <line class="helm-bar" x1="${C}" y1="${C - hub}" x2="${C}" y2="${knobY + knob}"></line>
          <g class="helm-handle" role="button" tabindex="0" data-i="${i}"
             aria-label="${esc(`${a.name} ${a.version || ""}`)}">
            <circle class="helm-hit" cx="${C}" cy="${knobY}" r="${knob + 8}"></circle>
            <circle class="helm-knob" cx="${C}" cy="${knobY}" r="${knob}"></circle>
            <g class="helm-label"><text>${esc(a.name)}</text></g>
          </g>
        </g>`)
      .join("");
    el.innerHTML = `
      <div class="helm">
        <svg class="helm-svg" viewBox="-78 -30 456 352" role="group" aria-label="Installed applications, ${n} total">
          <g class="helm-wheel">
            <circle class="helm-rim" cx="${C}" cy="${C}" r="${R}"></circle>
            <circle class="helm-rim-inner" cx="${C}" cy="${C}" r="${R - 12}"></circle>
            ${spokes}
            <circle class="helm-hub" cx="${C}" cy="${C}" r="${hub}"></circle>
            <circle class="helm-hub-cap" cx="${C}" cy="${C}" r="${hub - 12}"></circle>
          </g>
        </svg>
        <div class="helm-detail" aria-live="polite"></div>
      </div>`;

    const wheel = el.querySelector(".helm-wheel");
    const detail = el.querySelector(".helm-detail");
    const handles = [...el.querySelectorAll(".helm-handle")];
    let current = 0;
    let turn = 0; // accumulated rotation, so the wheel always takes the short way round
    let settle;

    // Name plates sit outside each knob, upright, on the side facing away
    // from the hub. They are placed for the wheel's resting angle.
    function placeLabels() {
      handles.forEach((h, k) => {
        const theta = turn + k * step;
        const rad = (theta * Math.PI) / 180;
        const sx = Math.sin(rad), cy = -Math.cos(rad);
        const d = knob + 7;
        const anchor = sx > 0.35 ? "start" : sx < -0.35 ? "end" : "middle";
        const dy = cy > 0.35 ? 11 : cy < -0.35 ? -2 : 4;
        const lab = h.querySelector(".helm-label");
        lab.setAttribute("transform",
          `translate(${C} ${knobY}) rotate(${-theta}) translate(${(sx * d).toFixed(1)} ${(cy * d + dy).toFixed(1)})`);
        lab.querySelector("text").setAttribute("text-anchor", anchor);
      });
      el.classList.remove("is-turning");
    }

    function select(i, focus) {
      const delta = ((((i - current) * step) % 360) + 540) % 360 - 180;
      turn -= delta;
      current = i;
      clearTimeout(settle);
      if (delta !== 0 && !reduceMotion) {
        el.classList.add("is-turning");
        settle = setTimeout(placeLabels, 900);
      } else {
        placeLabels();
      }
      wheel.style.transition = reduceMotion ? "none" : "";
      wheel.style.transform = `rotate(${turn}deg)`;
      handles.forEach((h, k) => {
        h.classList.toggle("is-active", k === i);
        h.setAttribute("aria-pressed", k === i ? "true" : "false");
      });
      const a = apps[i];
      const ok = String(a.status || "").toLowerCase() === "available";
      detail.innerHTML = `
        <p class="helm-eyebrow">Application ${i + 1} of ${n}</p>
        <h4 class="helm-name">${esc(a.name)} <span class="helm-version">${esc(a.version || "")}</span></h4>
        <p class="helm-desc">${esc(a.description || "No description.")}</p>
        <dl class="helm-facts">
          <div><dt>Status</dt><dd><span class="dash-state ${ok ? "st-good" : "st-warn"}">
            <i class="fa-solid ${ok ? "fa-circle-check" : "fa-circle-pause"}" aria-hidden="true"></i> ${esc(a.status || "unknown")}</span></dd></div>
          <div><dt>Image</dt><dd><code>${esc(a.image)}</code></dd></div>
        </dl>
        <button type="button" class="dash-action primary" data-action="run-app" data-app="${esc(a.name)}" ${ok ? "" : "disabled"}>
          <i class="fa-solid fa-play" aria-hidden="true"></i> Run ${esc(a.name)}</button>
        ${ok ? "" : `<p class="dash-note">This application isn't available to run yet.</p>`}`;
      if (focus) handles[i].focus();
    }

    handles.forEach((h) => {
      h.addEventListener("click", () => select(Number(h.dataset.i)));
      h.addEventListener("keydown", (e) => {
        if (e.key === "Enter" || e.key === " ") { e.preventDefault(); select(Number(h.dataset.i)); }
        if (e.key === "ArrowRight" || e.key === "ArrowDown") { e.preventDefault(); select((current + 1) % n, true); }
        if (e.key === "ArrowLeft" || e.key === "ArrowUp") { e.preventDefault(); select((current - 1 + n) % n, true); }
      });
    });
    select(0);
  }

  /* ---------- admin: volumes ---------- */
  function volumes(el, vols) {
    if (!vols.length) {
      el.innerHTML = `<p class="dash-empty">No volumes yet.</p>`;
      return;
    }
    el.innerHTML = `<ul class="dash-vols">${vols
      .map((v) => {
        const cap = Number(v.capacity) || 0, use = Number(v.usage) || 0;
        const pct = cap ? (use / cap) * 100 : 0;
        return `<li><span class="dash-vol-name">${esc(v.name)}</span>
          <div class="dash-meter" tabindex="0" role="meter" aria-valuemin="0" aria-valuemax="100" aria-valuenow="${pct.toFixed(1)}"
            aria-label="${esc(`${v.name}: ${pct.toFixed(1)} percent used`)}"
            data-tip="${esc(`${v.name}: ${use.toFixed(2)} of ${cap} GB · ${fmtInt(v.objectCount || 0)} objects`)}">
            <span style="width:${Math.min(100, use > 0 ? Math.max(pct, 0.8) : 0)}%"></span></div>
          <span class="dash-vol-val">${cap ? `${pct < 0.1 && use ? "<0.1" : pct.toFixed(1)}%` : "—"}</span></li>`;
      })
      .join("")}</ul>`;
  }

  function cluster(el, m) {
    if (!m || typeof m !== "object" || m.error) { el.innerHTML = ""; return; }
    const pods = m.pods || {};
    const podTotal = Object.values(pods).reduce((a, b) => a + (Number(b) || 0), 0);
    el.innerHTML = `<h4 class="dash-sub">Cluster</h4><div class="dash-tiles"></div>`;
    tiles(el.querySelector(".dash-tiles"), [
      { label: "Pods", value: fmtInt(podTotal) },
      { label: "CPU requested", value: `${fmtInt(m.total_cpu_milli || 0)}m` },
      { label: "Memory", value: fmtBytes(Number(m.total_mem_bytes) || 0) },
    ]);
  }

  /* ---------- navigation from the dashboard ---------- */
  function go(action, app) {
    switch (action) {
      case "new-job":
      case "run-app":
        showSection("job-browser");
        showSubSection("job-browser", "new-job");
        if (app) {
          const sel = document.getElementById("language-selector");
          if (sel && [...sel.options].some((o) => o.value === app)) {
            sel.value = app;
            sel.dispatchEvent(new Event("change", { bubbles: true }));
          }
        }
        break;
      case "apps":
        showSection("job-browser");
        showSubSection("job-browser", "application-display");
        break;
      case "jobs":
        showSection("job-browser");
        showSubSection("job-browser", "job-feedback-view");
        break;
      case "upload":
        showSection("access-profile");
        document.querySelector("#access-profile .fupload")?.scrollIntoView({ behavior: reduceMotion ? "auto" : "smooth" });
        break;
    }
  }

  async function load() {
    const root = document.getElementById("dash");
    if (!root) return;
    document.body.appendChild(tip);
    root.addEventListener("click", (e) => {
      const a = e.target.closest("[data-action]");
      if (!a || a.disabled) return;
      e.preventDefault();
      go(a.dataset.action, a.dataset.app);
    });

    const uid = Number(root.dataset.uid);
    const admin = root.dataset.elevated === "1";
    const [jobsR, resR, appsR, volR, usersR, groupsR, metricsR] = await Promise.all([
      getJSON("/fetch-jobs?format=json"),
      getJSON("/fetch-resources?format=json"),
      getJSON("/fetch-apps?format=json"),
      admin ? getJSON("/fetch-volumes?format=json") : null,
      admin ? getJSON("/admin/fetch-users?format=json") : null,
      admin ? getJSON("/admin/fetch-groups?format=json") : null,
      admin ? getJSON("/admin/system-metrics?format=json") : null,
    ]);

    const jobs = asList(jobsR);
    const resources = asList(resR);
    const apps = asList(appsR);

    const counts = { completed: 0, running: 0, pending: 0, failed: 0 };
    jobs.forEach((j) => { counts[stateOf(j)]++; });

    tiles(root.querySelector("#dash-job-tiles"), [
      { label: "Total", value: fmtInt(jobs.length) },
      ...STATES.map((s) => ({ label: s.label, value: fmtInt(counts[s.key]), icon: s.icon, cls: s.cls })),
    ]);
    statusBar(root.querySelector("#dash-job-status"), counts, jobs.length);
    dayColumns(root.querySelector("#dash-job-days"), jobs);
    storage(root.querySelector("#dash-storage"), resources, uid, root.dataset.quotaGb);
    recent(root.querySelector("#dash-recent"), jobs);
    helm(root.querySelector("#dash-helm"), apps);

    if (admin) {
      const vols = asList(volR && volR.volumes);
      tiles(root.querySelector("#dash-platform-tiles"), [
        { label: "Users", value: fmtInt(asList(usersR).length) },
        { label: "Groups", value: fmtInt(asList(groupsR).length) },
        { label: "Applications", value: fmtInt(apps.length) },
        { label: "Volumes", value: fmtInt(vols.length) },
      ]);
      volumes(root.querySelector("#dash-volumes"), vols);
      cluster(root.querySelector("#dash-cluster"), metricsR);
    }

    const mineBytes = resources.filter((r) => Number(r.uid) === uid).reduce((a, r) => a + (Number(r.size) || 0), 0);
    const bits = [];
    if (counts.running) bits.push(`${counts.running} job${counts.running > 1 ? "s" : ""} running`);
    if (counts.pending) bits.push(`${counts.pending} waiting`);
    if (!bits.length) bits.push(jobs.length ? "No jobs running right now" : "No jobs yet");
    bits.push(`${fmtBytes(mineBytes)} stored`);
    bits.push(`${apps.length} application${apps.length === 1 ? "" : "s"} available`);
    root.querySelector("#dash-summary").textContent = bits.join(" · ");

    bindTips(root);
    root.classList.add("is-ready");
  }

  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", load);
  else load();
})();
