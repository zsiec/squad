// insights.js — stats panel rendering /api/stats: verification rate,
// claim percentiles, WIP violations, item-status mix, verification by
// kind, agent leaderboard, by-capability. Window selector switches
// 24h / 7d / 30d in place.

import { fetchJSON, escapeHtml } from './util.js';

let panelEl, bodyEl, windowSelectEl;
let chartjsLoaded = null;
let currentWindow = '168h';
const charts = [];
let renderToken = {};

const WINDOW_OPTIONS_HTML = `
  <option value="24h">last 24 hours</option>
  <option value="168h" selected>last 7 days</option>
  <option value="720h">last 30 days</option>
`;

const TPL = `
  <div class="insights-grid">
    <section class="insights-tile" data-tile="verify"><h3>Verification rate (daily)</h3>
      <canvas id="chart-verify"></canvas>
      <div class="insights-tile-summary" id="sum-verify"></div></section>
    <section class="insights-tile" data-tile="p99"><h3>Claim duration p99 (daily)</h3>
      <canvas id="chart-p99"></canvas>
      <div class="insights-tile-summary" id="sum-p99"></div></section>
    <section class="insights-tile" data-tile="wip"><h3>WIP-cap violations attempted (daily)</h3>
      <canvas id="chart-wip"></canvas>
      <div class="insights-tile-summary" id="sum-wip"></div></section>
    <section class="insights-tile" data-tile="status-mix"><h3>Item status mix</h3>
      <canvas id="chart-status-mix"></canvas>
      <div class="insights-tile-summary" id="sum-status-mix"></div></section>
    <section class="insights-tile" data-tile="claim-percentiles"><h3>Claim duration percentiles</h3>
      <canvas id="chart-claim-percentiles"></canvas>
      <div class="insights-tile-summary" id="sum-claim-percentiles"></div></section>
    <section class="insights-tile" data-tile="verify-bykind"><h3>Verification by kind</h3>
      <canvas id="chart-verify-bykind"></canvas>
      <div class="insights-tile-summary" id="sum-verify-bykind"></div></section>
    <section class="insights-tile insights-tile-wide" data-tile="leaderboard"><h3>Agent leaderboard (top 10)</h3>
      <div id="leaderboard-table" class="insights-leaderboard"></div></section>
    <section class="insights-tile" data-tile="by-capability"><h3>Done by capability</h3>
      <canvas id="chart-by-capability"></canvas>
      <div class="insights-tile-summary" id="sum-by-capability"></div></section>
  </div>`;

export function initInsights() {
  mount();
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && panelEl && !panelEl.hidden) close();
  });
}

function mount() {
  panelEl = document.createElement('div');
  panelEl.className = 'insights';
  panelEl.hidden = true;
  panelEl.innerHTML = `
    <div class="insights-backdrop" data-close></div>
    <aside class="insights-panel" role="dialog" aria-label="Stats">
      <header class="insights-head">
        <span class="insights-title">STATS</span>
        <select id="insights-window" class="insights-window" aria-label="Stats window">
          ${WINDOW_OPTIONS_HTML}
        </select>
        <button class="icon-btn" data-close aria-label="Close">✕</button>
      </header>
      <div class="insights-body" id="insights-body">
        <div class="insights-loading">loading…</div>
      </div>
    </aside>`;
  document.body.appendChild(panelEl);
  panelEl.addEventListener('click', (e) => {
    if (e.target.closest('[data-close]')) close();
  });
  bodyEl = panelEl.querySelector('#insights-body');
  windowSelectEl = panelEl.querySelector('#insights-window');
  windowSelectEl.addEventListener('change', () => {
    currentWindow = windowSelectEl.value;
    renderInsights(bodyEl);
  });
}

export function open() {
  if (!panelEl) mount();
  panelEl.hidden = false;
  requestAnimationFrame(() => panelEl.classList.add('show'));
  renderInsights(bodyEl);
}

export function close() {
  panelEl.classList.remove('show');
  setTimeout(() => { panelEl.hidden = true; }, 240);
}

export async function renderInsights(container) {
  const myToken = (renderToken = {});
  destroyCharts();
  container.innerHTML = `<div class="insights-loading">loading…</div>`;
  try {
    const [snap] = await Promise.all([
      fetchJSON('/api/stats?window=' + encodeURIComponent(currentWindow)),
      ensureChartJs(),
    ]);
    if (renderToken !== myToken) return;
    container.innerHTML = TPL;
    drawVerify(
      container.querySelector('#chart-verify'),
      container.querySelector('#sum-verify'),
      snap.series?.verification_rate_daily || [],
    );
    drawP99(
      container.querySelector('#chart-p99'),
      container.querySelector('#sum-p99'),
      snap.series?.claim_p99_daily || [],
    );
    drawWIP(
      container.querySelector('#chart-wip'),
      container.querySelector('#sum-wip'),
      snap.series?.wip_violations_daily || [],
      snap.claims?.wip_violations_attempted || 0,
    );
    drawStatusMix(
      container.querySelector('#chart-status-mix'),
      container.querySelector('#sum-status-mix'),
      snap.items || {},
    );
    drawClaimPercentiles(
      container.querySelector('#chart-claim-percentiles'),
      container.querySelector('#sum-claim-percentiles'),
      snap.claims?.duration_seconds || {},
    );
    drawVerifyByKind(
      container.querySelector('#chart-verify-bykind'),
      container.querySelector('#sum-verify-bykind'),
      snap.verification?.by_kind || {},
    );
    drawLeaderboard(
      container.querySelector('#leaderboard-table'),
      snap.by_agent || [],
    );
    drawByCapability(
      container.querySelector('#chart-by-capability'),
      container.querySelector('#sum-by-capability'),
      snap.by_capability || [],
    );
  } catch (err) {
    if (renderToken !== myToken) return;
    container.innerHTML = `<div class="insights-error">error: ${escapeHtml(err.message || String(err))}</div>`;
  }
}

function destroyCharts() {
  while (charts.length) {
    const c = charts.pop();
    try { c.destroy(); } catch (_) {}
  }
}

async function ensureChartJs() {
  if (chartjsLoaded) return chartjsLoaded;
  chartjsLoaded = new Promise((resolve, reject) => {
    const s1 = document.createElement('script');
    s1.src = 'https://cdn.jsdelivr.net/npm/chart.js@4.5.1/dist/chart.umd.min.js';
    s1.integrity = 'sha384-jb8JQMbMoBUzgWatfe6COACi2ljcDdZQ2OxczGA3bGNeWe+6DChMTBJemed7ZnvJ';
    s1.crossOrigin = 'anonymous';
    s1.onload = () => {
      if (!window.Chart) {
        reject(new Error('chart.js loaded but window.Chart is undefined'));
        return;
      }
      const s2 = document.createElement('script');
      s2.src = 'https://cdn.jsdelivr.net/npm/chartjs-adapter-date-fns@3.0.0/dist/chartjs-adapter-date-fns.bundle.min.js';
      s2.integrity = 'sha384-cVMg8E3QFwTvGCDuK+ET4PD341jF3W8nO1auiXfuZNQkzbUUiBGLsIQUE+b1mxws';
      s2.crossOrigin = 'anonymous';
      s2.onload = () => resolve(window.Chart);
      s2.onerror = () => reject(new Error('chartjs-adapter-date-fns failed to load'));
      document.head.appendChild(s2);
    };
    s1.onerror = () => reject(new Error('chart.js failed to load'));
    document.head.appendChild(s1);
  }).catch((err) => {
    chartjsLoaded = null;
    throw err;
  });
  return chartjsLoaded;
}

// emptyCanvas paints a plain "no data" message into the supplied
// canvas in lieu of a chart.js render. Used by every chart tile so
// AC#3 ("explicit empty-state, not a blank canvas or chart.js error")
// holds uniformly.
function emptyCanvas(canvas, msg) {
  canvas.width = canvas.clientWidth || 240;
  canvas.height = canvas.clientHeight || 220;
  const ctx = canvas.getContext('2d');
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = '#6b7280';
  ctx.font = '12px ui-monospace, monospace';
  ctx.textAlign = 'center';
  ctx.fillText(msg || 'no data', canvas.width / 2, canvas.height / 2);
}

function drawVerify(canvas, sumEl, series) {
  if (!series.length) {
    sumEl.textContent = 'no data';
    emptyCanvas(canvas, 'no data');
    return;
  }
  const last = series[series.length - 1];
  const full = Math.round(last.rate * last.count);
  sumEl.textContent = `${(last.rate * 100).toFixed(1)}% (${full}/${last.count})`;
  const cfg = {
    type: 'line',
    data: {
      labels: series.map((p) => new Date(p.bucket_ts * 1000)),
      datasets: [{
        label: 'verification rate',
        data: series.map((p) => p.rate),
        borderColor: '#4ade80',
        backgroundColor: 'rgba(74, 222, 128, 0.15)',
        fill: true, tension: 0.2, spanGaps: false,
        pointRadius: 3, pointHoverRadius: 5,
      }],
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      scales: {
        x: { type: 'time', time: { unit: 'day', tooltipFormat: 'yyyy-MM-dd' } },
        y: { min: 0, max: 1, ticks: { callback: (v) => (v * 100).toFixed(0) + '%' } },
      },
      plugins: {
        legend: { display: false },
        tooltip: { callbacks: { label: (ctx) => {
          const p = series[ctx.dataIndex];
          return `${(p.rate * 100).toFixed(1)}% (${p.count} dones)`;
        } } },
      },
    },
  };
  charts.push(new window.Chart(canvas.getContext('2d'), cfg));
}

function drawP99(canvas, sumEl, series) {
  if (!series.length) {
    sumEl.textContent = 'no data';
    emptyCanvas(canvas, 'no data');
    return;
  }
  const last = series[series.length - 1];
  sumEl.textContent = `${Math.round(last.p99_seconds)}s (n=${last.count})`;
  const cfg = {
    type: 'line',
    data: {
      labels: series.map((p) => new Date(p.bucket_ts * 1000)),
      datasets: [{
        label: 'claim p99 (s)',
        data: series.map((p) => p.p99_seconds),
        borderColor: '#60a5fa',
        backgroundColor: 'rgba(96, 165, 250, 0.15)',
        fill: true, tension: 0.2, spanGaps: false,
        pointRadius: 3, pointHoverRadius: 5,
      }],
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      scales: {
        x: { type: 'time', time: { unit: 'day', tooltipFormat: 'yyyy-MM-dd' } },
        y: { beginAtZero: true, ticks: { callback: (v) => v + 's' } },
      },
      plugins: {
        legend: { display: false },
        tooltip: { callbacks: { label: (ctx) => {
          const p = series[ctx.dataIndex];
          return `${Math.round(p.p99_seconds)}s (n=${p.count})`;
        } } },
      },
    },
  };
  charts.push(new window.Chart(canvas.getContext('2d'), cfg));
}

function drawWIP(canvas, sumEl, series, totalAttempted) {
  sumEl.textContent = `${totalAttempted} attempted in window`;
  if (!series.length) {
    emptyCanvas(canvas, 'no violations recorded');
    return;
  }
  const cfg = {
    type: 'bar',
    data: {
      labels: series.map((p) => new Date(p.bucket_ts * 1000)),
      datasets: [{
        label: 'wip violations',
        data: series.map((p) => p.count),
        backgroundColor: '#f87171',
      }],
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      scales: {
        x: { type: 'time', time: { unit: 'day', tooltipFormat: 'yyyy-MM-dd' } },
        y: { beginAtZero: true, ticks: { precision: 0, stepSize: 1 } },
      },
      plugins: {
        legend: { display: false },
        tooltip: { callbacks: { label: (ctx) => `${ctx.parsed.y} attempted` } },
      },
    },
  };
  charts.push(new window.Chart(canvas.getContext('2d'), cfg));
}

function drawStatusMix(canvas, sumEl, items) {
  const open    = items.open    || 0;
  const claimed = items.claimed || 0;
  const blocked = items.blocked || 0;
  const done    = items.done    || 0;
  const total   = open + claimed + blocked + done;
  if (total === 0) {
    sumEl.textContent = 'no data';
    emptyCanvas(canvas, 'no items');
    return;
  }
  sumEl.textContent = `${total} total (${done} done · ${claimed} claimed · ${open} open · ${blocked} blocked)`;
  const cfg = {
    type: 'doughnut',
    data: {
      labels: ['done', 'claimed', 'open', 'blocked'],
      datasets: [{
        data: [done, claimed, open, blocked],
        backgroundColor: ['#4ade80', '#60a5fa', '#9ca3af', '#f59e0b'],
        borderWidth: 0,
      }],
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      plugins: {
        legend: { position: 'bottom', labels: { boxWidth: 12, font: { size: 11 } } },
        tooltip: { callbacks: { label: (ctx) => `${ctx.label}: ${ctx.parsed} (${((ctx.parsed / total) * 100).toFixed(0)}%)` } },
      },
    },
  };
  charts.push(new window.Chart(canvas.getContext('2d'), cfg));
}

function drawClaimPercentiles(canvas, sumEl, p) {
  const p50 = p.p50, p90 = p.p90, p99 = p.p99;
  if (p50 == null && p90 == null && p99 == null) {
    sumEl.textContent = 'no data';
    emptyCanvas(canvas, 'no claims completed');
    return;
  }
  sumEl.textContent = `n=${p.count || 0} · p50 ${fmtSeconds(p50)} · p90 ${fmtSeconds(p90)} · p99 ${fmtSeconds(p99)}`;
  const cfg = {
    type: 'bar',
    data: {
      labels: ['p50', 'p90', 'p99'],
      datasets: [{
        label: 'seconds',
        data: [p50 || 0, p90 || 0, p99 || 0],
        backgroundColor: ['#4ade80', '#60a5fa', '#f87171'],
        borderWidth: 0,
      }],
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      scales: { y: { beginAtZero: true, ticks: { callback: (v) => fmtSeconds(v) } } },
      plugins: {
        legend: { display: false },
        tooltip: { callbacks: { label: (ctx) => fmtSeconds(ctx.parsed.y) } },
      },
    },
  };
  charts.push(new window.Chart(canvas.getContext('2d'), cfg));
}

function drawVerifyByKind(canvas, sumEl, byKind) {
  const kinds = Object.keys(byKind || {});
  if (!kinds.length) {
    sumEl.textContent = 'no data';
    emptyCanvas(canvas, 'no attestations recorded');
    return;
  }
  kinds.sort();
  const passedData   = kinds.map((k) => byKind[k].passed);
  const attestedData = kinds.map((k) => byKind[k].attested);
  const totalAttested = attestedData.reduce((a, b) => a + b, 0);
  const totalPassed   = passedData.reduce((a, b) => a + b, 0);
  sumEl.textContent = `${totalPassed}/${totalAttested} passed across ${kinds.length} kinds`;
  const cfg = {
    type: 'bar',
    data: {
      labels: kinds,
      datasets: [
        { label: 'passed',   data: passedData,                                                backgroundColor: '#4ade80' },
        { label: 'attested', data: attestedData.map((a, i) => Math.max(0, a - passedData[i])), backgroundColor: '#9ca3af' },
      ],
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      scales: {
        x: { stacked: true },
        y: { stacked: true, beginAtZero: true, ticks: { precision: 0, stepSize: 1 } },
      },
      plugins: { legend: { position: 'bottom', labels: { boxWidth: 12, font: { size: 11 } } } },
    },
  };
  charts.push(new window.Chart(canvas.getContext('2d'), cfg));
}

function drawLeaderboard(host, rows) {
  if (!rows.length) {
    host.innerHTML = `<div class="insights-empty">no agent activity in window</div>`;
    return;
  }
  const sorted = rows.slice().sort((a, b) => (b.claims_completed || 0) - (a.claims_completed || 0)).slice(0, 10);
  const cells = sorted.map((r) => `
    <tr>
      <td>${escapeHtml(r.display_name || r.agent_id)}</td>
      <td class="num">${r.claims_completed || 0}</td>
      <td class="num">${r.release_count || 0}</td>
      <td class="num">${r.ratio == null ? '–' : r.ratio.toFixed(1)}</td>
      <td class="num">${r.verification_rate == null ? '–' : (r.verification_rate * 100).toFixed(0) + '%'}</td>
    </tr>`).join('');
  host.innerHTML = `
    <table class="insights-leaderboard-table">
      <thead><tr>
        <th>agent</th><th>done</th><th>released</th><th>ratio</th><th>verify</th>
      </tr></thead>
      <tbody>${cells}</tbody>
    </table>`;
}

function drawByCapability(canvas, sumEl, rows) {
  if (!rows.length) {
    sumEl.textContent = 'no data';
    emptyCanvas(canvas, 'no capability tags');
    return;
  }
  const sorted = rows.slice().sort((a, b) => (b.done_count || 0) - (a.done_count || 0)).slice(0, 10);
  const labels = sorted.map((r) => r.capability || '(untagged)');
  const data   = sorted.map((r) => r.done_count || 0);
  const totalDone = data.reduce((a, b) => a + b, 0);
  sumEl.textContent = `${rows.length} capabilities · ${totalDone} tagged-done counts (top 10)`;
  const cfg = {
    type: 'bar',
    data: {
      labels,
      datasets: [{
        label: 'done items',
        data,
        backgroundColor: '#a78bfa',
      }],
    },
    options: {
      responsive: true, maintainAspectRatio: false,
      indexAxis: 'y',
      scales: { x: { beginAtZero: true, ticks: { precision: 0, stepSize: 1 } } },
      plugins: { legend: { display: false } },
    },
  };
  charts.push(new window.Chart(canvas.getContext('2d'), cfg));
}

function fmtSeconds(v) {
  if (v == null || v === 0) return '0s';
  if (v < 60) return Math.round(v) + 's';
  if (v < 3600) return (v / 60).toFixed(1) + 'm';
  if (v < 86400) return (v / 3600).toFixed(1) + 'h';
  return (v / 86400).toFixed(1) + 'd';
}
