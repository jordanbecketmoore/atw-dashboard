// Frontend for the ATW Dashboard. Fetches an initial snapshot from /api/state
// and subscribes to /events for live updates pushed by the Go backend.

const CONSOLE_LINE_CAP = 50;

const charts = new Map();        // warrior name -> { chart, sending, receiving }
let nickname = "";
let leaderboard = {};

function humanBytes(bytes) {
  if (bytes > 1024 * 1024 * 1024) {
    return (Math.round(10 * bytes / (1024 * 1024 * 1024)) / 10) + " GB";
  } else if (bytes > 1024 * 1024) {
    return (Math.round(10 * bytes / (1024 * 1024)) / 10) + " MB";
  } else {
    return (Math.round(10 * bytes / 1024) / 10) + " kB";
  }
}

function buildCard(w) {
  const card = document.createElement("div");
  card.id = w.name;
  card.className = "chartSet";
  if (w.connected) card.classList.add("connected");
  if (w.status?.error) card.classList.add("error");
  if (w.status?.uploading) card.classList.add("uploading");
  if (w.status?.throttle) card.classList.add("throttle");

  card.innerHTML = `
    <div class="legend">
      <div id="${w.name}-heading" class="heading">
        <h2>${w.name}</h2>
        <h2 class="isOffline">Offline</h2>
      </div>
      <div class="bandwidth sending" id="bandwidth-sending-${w.name}">${humanBytes(w.sending || 0)}/s</div>
      <div class="bandwidth receiving" id="bandwidth-receiving-${w.name}">${humanBytes(w.receiving || 0)}/s</div>
      <div class="bandwidth sent" id="bandwidth-sent-${w.name}">${humanBytes(w.sent || 0)}</div>
      <div class="bandwidth received" id="bandwidth-received-${w.name}">${humanBytes(w.received || 0)}</div>
    </div>
    <canvas id="bandwidth-canvas-${w.name}" width="300" height="250"></canvas>
  `;
  return card;
}

function ensureChart(name) {
  if (charts.has(name)) return charts.get(name);
  const canvas = document.getElementById(`bandwidth-canvas-${name}`);
  if (!canvas) return null;
  const chart = new SmoothieChart({
    minValue: 0,
    millisPerPixel: 100,
    grid: {
      fillStyle: "#000000",
      strokeStyle: "#444444",
      lineWidth: 1,
      millisPerLine: 2000,
      verticalSections: 3,
    },
  });
  const receiving = new TimeSeries();
  const sending = new TimeSeries();
  chart.addTimeSeries(receiving, { strokeStyle: "#459B34", fillStyle: "rgba(0,138,92,0.30)" });
  chart.addTimeSeries(sending);
  chart.streamTo(canvas, 1000);
  const entry = { chart, sending, receiving };
  charts.set(name, entry);
  return entry;
}

function appendConsole(name, line, time) {
  const consoleDiv = document.getElementById("console");
  const p = document.createElement("p");
  const ts = time ? new Date(time).toLocaleString() : new Date().toLocaleString();
  if (line.includes("=403")) p.classList.add("error", "e403");
  p.innerHTML = `<span class="instName ${name}">${name}</span>: <span class="msg">${escapeHTML(line)}</span> - <span class="timestamp">${ts}</span>`;
  consoleDiv.appendChild(p);
  consoleDiv.scrollTop = consoleDiv.scrollHeight;
  while (consoleDiv.childNodes.length > CONSOLE_LINE_CAP) {
    consoleDiv.removeChild(consoleDiv.firstElementChild);
  }
}

function escapeHTML(s) {
  return s.replace(/[&<>"']/g, (c) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  }[c]));
}

function renderLeaderboard() {
  const metric = document.getElementById("metric");
  const projects = Object.keys(leaderboard).sort();
  metric.innerHTML = projects.map((project) => {
    const s = leaderboard[project];
    if (!s) return "";
    const positionLabel = s.position > 0 ? `#${s.position} of ${s.total}` : `not ranked (${s.total} downloaders)`;
    return `
      <div id="${project}-metric" class="project-metric">
        <h3>${escapeHTML(project)}</h3>
        <div class="rank">${escapeHTML(nickname)}: ${positionLabel}</div>
        <div class="stats">${humanBytes(s.bytes)} &middot; ${s.items.toLocaleString()} items</div>
        <a href="https://tracker.archiveteam.org/${encodeURIComponent(project)}/" target="_blank" rel="noopener">tracker</a>
      </div>
    `;
  }).join("");
}

function setConnectionStatus(text, cls) {
  const el = document.getElementById("connection-status");
  if (!el) return;
  el.textContent = text;
  el.className = `connection-status ${cls || ""}`.trim();
}

function applyStatus(name, status) {
  const card = document.getElementById(name);
  if (!card) return;
  card.classList.toggle("error", !!status.error);
  card.classList.toggle("uploading", !!status.uploading);
  card.classList.toggle("throttle", !!status.throttle);
}

function applyConnection(name, connected) {
  const card = document.getElementById(name);
  if (!card) return;
  card.classList.toggle("connected", !!connected);
}

function applyProject(name, project) {
  const card = document.getElementById(name);
  if (!card) return;
  const heading = card.querySelector(".heading h2:first-child");
  if (heading) heading.textContent = project ? `${name} — ${project}` : name;
}

function applyBandwidth(name, msg) {
  const entry = ensureChart(name);
  if (!entry) return;
  const now = Date.now();
  entry.sending.append(now, msg.sending / 1024);
  entry.receiving.append(now, msg.receiving / 1024);
  const set = (id, val) => {
    const el = document.getElementById(id);
    if (el) el.innerHTML = val;
  };
  set(`bandwidth-sending-${name}`, humanBytes(msg.sending) + "/s");
  set(`bandwidth-receiving-${name}`, humanBytes(msg.receiving) + "/s");
  set(`bandwidth-sent-${name}`, humanBytes(msg.sent));
  set(`bandwidth-received-${name}`, humanBytes(msg.received));
}

async function loadInitialState() {
  const resp = await fetch("/api/state");
  if (!resp.ok) throw new Error(`/api/state returned ${resp.status}`);
  const state = await resp.json();
  nickname = state.nickname || "";
  leaderboard = state.leaderboard || {};

  const chartArea = document.getElementById("chartArea");
  chartArea.innerHTML = "";
  const warriors = (state.warriors || []).slice().sort((a, b) => a.name.localeCompare(b.name));
  for (const w of warriors) {
    chartArea.appendChild(buildCard(w));
    ensureChart(w.name);
    if (w.project) applyProject(w.name, w.project);
  }
  renderLeaderboard();
}

function connectEvents() {
  const es = new EventSource("/events");
  es.addEventListener("open", () => setConnectionStatus("connected", "ok"));
  es.addEventListener("error", () => setConnectionStatus("reconnecting…", "warn"));

  es.addEventListener("bandwidth", (e) => {
    const m = JSON.parse(e.data);
    applyBandwidth(m.name, m);
  });
  es.addEventListener("status", (e) => {
    const m = JSON.parse(e.data);
    applyStatus(m.name, m.status || {});
  });
  es.addEventListener("connection", (e) => {
    const m = JSON.parse(e.data);
    applyConnection(m.name, m.connected);
    appendConsole(m.name, m.connected ? "Connected." : (m.message || "Disconnected."));
  });
  es.addEventListener("project", (e) => {
    const m = JSON.parse(e.data);
    applyProject(m.name, m.project);
  });
  es.addEventListener("console", (e) => {
    const m = JSON.parse(e.data);
    appendConsole(m.name, m.line, m.time);
  });
  es.addEventListener("leaderboard", (e) => {
    const m = JSON.parse(e.data);
    if (m.stats) {
      leaderboard[m.project] = m.stats;
      renderLeaderboard();
    }
  });
}

window.addEventListener("DOMContentLoaded", async () => {
  try {
    await loadInitialState();
  } catch (err) {
    console.error("failed to load /api/state:", err);
    setConnectionStatus("failed to load initial state", "err");
    return;
  }
  connectEvents();
});
