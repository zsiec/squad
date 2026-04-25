// insights.js — slide-over stats panel: velocity, burndown, heatmap, top-N, at-risk, hot files

import { fetchJSON, escapeHtml, fmtAgo } from './util.js';
import { identicon } from './identicon.js';
import { displayName } from './names.js';

let panelEl, bodyEl;
let openItemFn = () => {};

export function initInsights({ onOpenItem }) {
  openItemFn = onOpenItem || (() => {});
  mount();
  document.getElementById('insights-btn')?.addEventListener('click', open);
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && !panelEl.hidden) close();
    if (e.key === 's' && !isTyping(e.target) && !panelEl.hidden) close();
  });
}

function isTyping(el) {
  return el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.isContentEditable);
}

function mount() {
  panelEl = document.createElement('div');
  panelEl.className = 'insights';
  panelEl.hidden = true;
  panelEl.innerHTML = `
    <div class="insights-backdrop" data-close></div>
    <aside class="insights-panel" role="dialog" aria-label="Insights">
      <header class="insights-head">
        <span class="insights-title">INSIGHTS</span>
        <span class="insights-sub" id="insights-sub">last 30 days</span>
        <button class="icon-btn" data-close aria-label="Close">✕</button>
      </header>
      <div class="insights-body" id="insights-body">
        <div class="insights-loading">loading…</div>
      </div>
    </aside>
  `;
  document.body.appendChild(panelEl);
  panelEl.addEventListener('click', (e) => {
    if (e.target.closest('[data-close]')) close();
  });
  bodyEl = panelEl.querySelector('#insights-body');
}

export function open() {
  panelEl.hidden = false;
  requestAnimationFrame(() => panelEl.classList.add('show'));
  load();
}

export function close() {
  panelEl.classList.remove('show');
  setTimeout(() => panelEl.hidden = true, 240);
}

async function load() {
  bodyEl.innerHTML = `<div class="insights-loading">loading…</div>`;
  try {
    const [stats, items, claims, agents] = await Promise.all([
      fetchJSON('/api/stats/activity?days=30'),
      fetchJSON('/api/items'),
      fetchJSON('/api/claims'),
      fetchJSON('/api/agents'),
    ]);
    render(stats, items, claims, agents);
  } catch (err) {
    bodyEl.innerHTML = `<div class="insights-error">error: ${escapeHtml(err.message)}</div>`;
  }
}

function render(stats, items, claims, agents) {
  bodyEl.innerHTML = `
    <section class="card kpis">
      ${kpis(stats, items, claims, agents)}
    </section>
    <section class="card">
      <h3>Velocity — shipped per day (30d)</h3>
      <div id="card-velocity"></div>
    </section>
    <section class="card">
      <h3>Messages — 24h × 7d heatmap (UTC)</h3>
      <div id="card-heatmap"></div>
    </section>
    <section class="card">
      <h3>Daily activity (30d)</h3>
      <div id="card-activity"></div>
    </section>
    <section class="insights-grid">
      <div class="card">
        <h3>Top areas · open</h3>
        <div id="card-areas"></div>
      </div>
      <div class="card">
        <h3>Top chatters · 30d</h3>
        <div id="card-agents"></div>
      </div>
    </section>
    <section class="card">
      <h3>At-risk items</h3>
      <div id="card-atrisk"></div>
    </section>
    <section class="card">
      <h3>Hot files — currently held by ≥1 agent</h3>
      <div id="card-hotfiles"></div>
    </section>
  `;

  renderVelocity(stats.per_day, bodyEl.querySelector('#card-velocity'));
  renderHeatmap(stats.hour_of_week, bodyEl.querySelector('#card-heatmap'));
  renderActivityArea(stats.per_day, bodyEl.querySelector('#card-activity'));
  renderBarChart(stats.top_areas || [], bodyEl.querySelector('#card-areas'));
  renderAgentsTop(stats.top_agents || [], bodyEl.querySelector('#card-agents'));
  renderAtRisk(items, claims, bodyEl.querySelector('#card-atrisk'));
  renderHotFiles(stats.hot_files || [], bodyEl.querySelector('#card-hotfiles'));
}

function kpis(stats, items, claims, agents) {
  const openItems = items.filter((i) => i.status !== 'done').length;
  const p0 = items.filter((i) => i.priority === 'P0' && i.status !== 'done').length;
  const p1 = items.filter((i) => i.priority === 'P1' && i.status !== 'done').length;
  const active = agents.filter((a) => (a.status || a.Status) === 'active').length;
  const msgs30 = (stats.per_day || []).reduce((s, d) => s + d.messages, 0);
  const shipped30 = (stats.per_day || []).reduce((s, d) => s + d.dones, 0);
  const claim30   = (stats.per_day || []).reduce((s, d) => s + d.claims, 0);
  return [
    ['open items', openItems],
    ['in flight', claims.length],
    ['P0', p0, 'p0'],
    ['P1', p1, 'p1'],
    ['active agents', active],
    ['shipped · 30d', shipped30, 'ok'],
    ['claimed · 30d', claim30],
    ['messages · 30d', msgs30],
  ].map(([label, v, mod]) => `
    <div class="kpi${mod ? ' kpi-' + mod : ''}">
      <div class="kpi-val">${v}</div>
      <div class="kpi-label">${escapeHtml(label)}</div>
    </div>`).join('');
}

function renderVelocity(perDay, host) {
  const max = Math.max(1, ...perDay.map((d) => d.dones));
  const bars = perDay.map((d) => {
    const h = Math.round((d.dones / max) * 48);
    return `<div class="bar" style="height:${h}px" title="${d.day}: ${d.dones} shipped"><span class="bar-val">${d.dones || ''}</span></div>`;
  }).join('');
  host.innerHTML = `<div class="bar-chart"><div class="bars">${bars}</div><div class="axis-x">${perDay.filter((_,i) => i%5===0).map((d) => `<span>${d.day.slice(5)}</span>`).join('')}</div></div>`;
}

function renderActivityArea(perDay, host) {
  // stacked bars: messages+claims+releases+dones
  const total = perDay.map((d) => d.messages + d.claims + d.releases + d.dones);
  const max = Math.max(1, ...total);
  const bars = perDay.map((d, i) => {
    const t = total[i];
    const h = Math.round((t / max) * 56);
    const pMsg = (d.messages / (t||1)) * 100;
    const pClaim = (d.claims / (t||1)) * 100;
    const pRel = (d.releases / (t||1)) * 100;
    const pDone = (d.dones / (t||1)) * 100;
    return `<div class="stack-bar" style="height:${h}px" title="${d.day}: ${d.messages} msg · ${d.claims} claim · ${d.releases} release · ${d.dones} done">
      <div class="seg seg-msg"    style="flex-basis:${pMsg}%"></div>
      <div class="seg seg-claim"  style="flex-basis:${pClaim}%"></div>
      <div class="seg seg-rel"    style="flex-basis:${pRel}%"></div>
      <div class="seg seg-done"   style="flex-basis:${pDone}%"></div>
    </div>`;
  }).join('');
  host.innerHTML = `
    <div class="stack-chart">${bars}</div>
    <div class="stack-legend">
      <span><span class="lg seg-msg"></span>messages</span>
      <span><span class="lg seg-claim"></span>claims</span>
      <span><span class="lg seg-rel"></span>releases</span>
      <span><span class="lg seg-done"></span>done</span>
    </div>`;
}

function renderHeatmap(hourOfWeek, host) {
  const days = ['Sun','Mon','Tue','Wed','Thu','Fri','Sat'];
  let max = 0;
  for (let d = 0; d < 7; d++) for (let h = 0; h < 24; h++) max = Math.max(max, hourOfWeek[d][h]);
  const cells = [];
  for (let d = 0; d < 7; d++) {
    const row = [`<div class="hm-label">${days[d]}</div>`];
    for (let h = 0; h < 24; h++) {
      const v = hourOfWeek[d][h] || 0;
      const alpha = max ? Math.max(0.07, v / max) : 0.05;
      row.push(`<div class="hm-cell" style="background:rgba(224,168,96,${alpha.toFixed(3)})" title="${days[d]} ${h}:00 — ${v}"></div>`);
    }
    cells.push(`<div class="hm-row">${row.join('')}</div>`);
  }
  const hourAxis = Array.from({length: 24}, (_, h) => `<span>${h % 3 === 0 ? h : ''}</span>`).join('');
  host.innerHTML = `<div class="heatmap">${cells.join('')}<div class="hm-row"><div class="hm-label"></div>${hourAxis}</div></div>`;
}

function renderBarChart(rows, host) {
  if (!rows.length) { host.innerHTML = `<div class="empty">no data</div>`; return; }
  const max = Math.max(1, ...rows.map((r) => r.count));
  host.innerHTML = rows.map((r) => {
    const pct = (r.count / max) * 100;
    return `<div class="hbar">
      <span class="hbar-label">${escapeHtml(r.key)}</span>
      <span class="hbar-track"><span class="hbar-fill" style="width:${pct}%"></span></span>
      <span class="hbar-count">${r.count}</span>
    </div>`;
  }).join('');
}

function renderAgentsTop(rows, host) {
  if (!rows.length) { host.innerHTML = `<div class="empty">no activity</div>`; return; }
  const max = Math.max(1, ...rows.map((r) => r.count));
  host.innerHTML = rows.map((r) => {
    const pct = (r.count / max) * 100;
    const pretty = displayName(r.agent_id, r.display_name);
    const ico = identicon(r.agent_id, { size: 18, name: pretty });
    const div = document.createElement('div');
    div.className = 'hbar';
    div.innerHTML = `<span class="hbar-label ident-wrap"></span>
      <span class="hbar-track"><span class="hbar-fill" style="width:${pct}%"></span></span>
      <span class="hbar-count">${r.count}</span>`;
    const label = div.querySelector('.hbar-label');
    label.appendChild(ico);
    const name = document.createElement('span');
    name.className = 'hbar-name';
    name.textContent = pretty;
    label.appendChild(name);
    return div.outerHTML;
  }).join('');
}

function renderAtRisk(items, claims, host) {
  const now = Math.floor(Date.now() / 1000);
  const scored = [];

  for (const c of claims) {
    let score = 0;
    const reasons = [];
    const it = items.find((i) => i.id === c.item_id) || {};
    const age = now - (c.claimed_at || now);
    const quiet = now - (c.last_touch || c.claimed_at || now);
    if (quiet > 2 * 3600) { score += 3; reasons.push(`quiet ${Math.round(quiet/3600)}h`); }
    if (age > 3 * 86400)   { score += 2; reasons.push(`claimed ${Math.round(age/86400)}d ago`); }
    if (it.priority === 'P0') { score += 4; reasons.push('P0'); }
    if (it.priority === 'P1') { score += 1; }
    if (score >= 3) {
      scored.push({ id: c.item_id, title: it.title || '(unknown)', priority: it.priority || '', score, reasons, agent: c.agent_id });
    }
  }

  // unclaimed P0 / P1
  const claimedIds = new Set(claims.map((c) => c.item_id));
  for (const it of items) {
    if (claimedIds.has(it.id) || it.status === 'done') continue;
    const reasons = [];
    let score = 0;
    if (it.priority === 'P0') { score += 5; reasons.push('P0 unclaimed'); }
    if (it.priority === 'P1') { score += 2; reasons.push('P1 unclaimed'); }
    if (it.status === 'blocked') { score += 2; reasons.push('blocked'); }
    if (score >= 3) {
      scored.push({ id: it.id, title: it.title, priority: it.priority, score, reasons, agent: '' });
    }
  }

  scored.sort((a, b) => b.score - a.score);
  const top = scored.slice(0, 8);
  if (!top.length) {
    host.innerHTML = `<div class="empty">nothing looks at risk — nice.</div>`;
    return;
  }
  host.innerHTML = top.map((r) => `
    <div class="atrisk-row" data-item-id="${escapeHtml(r.id)}">
      <span class="atrisk-id">${escapeHtml(r.id)}</span>
      <span class="atrisk-pri pri-pip ${r.priority ? r.priority.toLowerCase() : 'p-unknown'}">${escapeHtml((r.priority || '').replace(/^P/, ''))}</span>
      <span class="atrisk-title">${escapeHtml(r.title)}</span>
      <span class="atrisk-reasons">${r.reasons.map((x) => `<span class="reason">${escapeHtml(x)}</span>`).join('')}</span>
      ${r.agent ? `<span class="atrisk-agent">${escapeHtml(r.agent)}</span>` : ''}
    </div>
  `).join('');
  host.querySelectorAll('.atrisk-row').forEach((el) => {
    el.addEventListener('click', () => {
      openItemFn(el.dataset.itemId);
      close();
    });
  });
}

function renderHotFiles(rows, host) {
  if (!rows.length) { host.innerHTML = `<div class="empty">no files held right now</div>`; return; }
  host.innerHTML = rows.map((r) => `
    <div class="hotfile">
      <span class="hotfile-count">${r.count}×</span>
      <code class="hotfile-path">${escapeHtml(r.key)}</code>
    </div>`).join('');
}
