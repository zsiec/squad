// insights.js — verification rate / claim p99 / wip violations (R7)

import { fetchJSON, escapeHtml } from './util.js';

let panelEl, bodyEl;
let chartjsLoaded = null;
const charts = [];

const TPL = `
  <div class="insights-grid">
    <section class="insights-tile"><h3>Verification rate (daily)</h3>
      <canvas id="chart-verify"></canvas>
      <div class="insights-tile-summary" id="sum-verify"></div></section>
    <section class="insights-tile"><h3>Claim duration p99 (daily)</h3>
      <canvas id="chart-p99"></canvas>
      <div class="insights-tile-summary" id="sum-p99"></div></section>
    <section class="insights-tile"><h3>WIP-cap violations attempted (daily)</h3>
      <canvas id="chart-wip"></canvas>
      <div class="insights-tile-summary" id="sum-wip"></div></section>
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
        <span class="insights-sub">last 7 days</span>
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
  destroyCharts();
  container.innerHTML = `<div class="insights-loading">loading…</div>`;
  try {
    const [snap] = await Promise.all([
      fetchJSON('/api/stats?window=168h'),
      ensureChartJs(),
    ]);
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
  } catch (err) {
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

function drawVerify(canvas, sumEl, series) {
  if (!series.length) { sumEl.textContent = 'no data'; return; }
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
  if (!series.length) { sumEl.textContent = 'no data'; return; }
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
  if (!series.length) return;
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
