// board.js — table-mode board + agents rail

import { fetchJSON, postJSON, escapeHtml, fmtAgo, priClass } from './util.js';
import { identicon } from './identicon.js';
import { displayName } from './names.js';
import { sparkline, bucketize } from './sparkline.js';
import { openAgentDetail } from './agent_detail.js';
import { toast } from './toasts.js';

const boardBody     = document.getElementById('board-body');
const boardTabs     = document.querySelectorAll('.board-tab');
const agentsList    = document.getElementById('agents-list');
const agentsCount   = document.getElementById('agents-count');
const countEls = {
  'in-progress': document.getElementById('count-in-progress'),
  ready:         document.getElementById('count-ready'),
  blocked:       document.getElementById('count-blocked'),
  done:          document.getElementById('count-done'),
};

let activeTab = 'in-progress';
let cached = { items: [], claims: [], agents: [] };
let selectedId = null;
let onItemClick = () => {};

boardTabs.forEach((t) => {
  t.addEventListener('click', () => {
    boardTabs.forEach((x) => x.classList.toggle('active', x === t));
    activeTab = t.dataset.section;
    renderBoard();
  });
});

export function setItemClickHandler(fn) { onItemClick = fn; }

export function setSelected(id) {
  selectedId = id;
  boardBody.querySelectorAll('tr[data-item]').forEach((tr) => {
    tr.dataset.selected = tr.dataset.item === id ? 'true' : 'false';
  });
}

export async function refreshBoard() {
  const [items, claims, agents] = await Promise.all([
    fetchJSON('/api/items'),
    fetchJSON('/api/claims'),
    fetchJSON('/api/agents'),
  ]);
  cached = { items, claims, agents };
  renderBoard();
  renderAgents();
  renderFilters();
  // prime sparklines for in-progress items (cheap)
  primeActivity(claims.map((c) => c.item_id)).then(() => renderBoard());
}

export async function refreshClaimsAndAgents() {
  const [claims, agents] = await Promise.all([
    fetchJSON('/api/claims'),
    fetchJSON('/api/agents'),
  ]);
  cached.claims = claims;
  cached.agents = agents;
  renderBoard();
  renderAgents();
}

export function claimsSnapshot() { return cached.claims; }
export function itemsSnapshot()  { return cached.items; }
export function agentsSnapshot() { return cached.agents; }
export function cacheSnapshot()  { return cached; }

let filter = { q: '', area: '', type: '', priority: '', view: '', epic: '' };
let meAgentId = '';
export function setMeAgentId(id) { meAgentId = id; }
export function setFilter(partial) {
  filter = { ...filter, ...partial };
  renderBoard();
}
export function currentFilter() { return filter; }

function renderBoard() {
  const { items, claims } = cached;
  const claimByItem = new Map(claims.map((c) => [c.item_id, c]));

  const now = Math.floor(Date.now() / 1000);
  const todayStart = new Date();
  todayStart.setHours(0, 0, 0, 0);
  const todayTs = Math.floor(todayStart.getTime() / 1000);
  const todayYMD = new Date().toISOString().slice(0, 10);

  const matches = (it) => {
    if (filter.q) {
      const q = filter.q.toLowerCase();
      const hay = [it.id, it.title, it.area, it.type].join(' ').toLowerCase();
      if (!hay.includes(q)) return false;
    }
    if (filter.area && it.area !== filter.area) return false;
    if (filter.type && it.type !== filter.type) return false;
    if (filter.priority && it.priority !== filter.priority) return false;
    if (filter.epic && (it.epic || '') !== filter.epic) return false;
    if (filter.view === 'mine') {
      const c = claimByItem.get(it.id);
      if (!c || c.agent_id !== meAgentId) return false;
    }
    if (filter.view === 'stale') {
      const c = claimByItem.get(it.id);
      if (!c) return false;
      const quiet = now - (c.last_touch || c.claimed_at || now);
      if (quiet < 2 * 3600) return false;
    }
    if (filter.view === 'unclaimed') {
      if (claimByItem.has(it.id)) return false;
    }
    if (filter.view === 'blocked') {
      if (it.status !== 'blocked') return false;
    }
    if (filter.view === 'new-today') {
      if (!it.created || it.created !== todayYMD) return false;
    }
    return true;
  };

  const inProgress = claims.map((c) => {
    const it = items.find((i) => i.id === c.item_id) || { id: c.item_id, priority: 'P3', title: '—' };
    return { ...it, claim: c };
  }).filter(matches);
  const ready = items
    .filter((i) => !claimByItem.has(i.id) && i.status !== 'done' && i.status !== 'blocked')
    .filter(matches)
    .sort((a, b) => (a.priority || '').localeCompare(b.priority || ''));
  const blocked = items.filter((i) => i.status === 'blocked').filter(matches);
  const done = items
    .filter((i) => i.status === 'done')
    .filter(matches)
    .sort((a, b) => (b.updated || '').localeCompare(a.updated || ''));

  const buckets = { 'in-progress': inProgress, ready, blocked, done };

  countEls['in-progress'].textContent = inProgress.length;
  countEls.ready.textContent          = ready.length;
  countEls.blocked.textContent        = blocked.length;
  countEls.done.textContent           = done.length;

  const rows = buckets[activeTab] || [];
  boardBody.innerHTML = '';

  if (rows.length === 0) {
    const tr = document.createElement('tr');
    tr.className = 'empty-row';
    tr.innerHTML = `<td colspan="5">no items in ${activeTab}</td>`;
    boardBody.appendChild(tr);
    return;
  }

  if (activeTab === 'ready') {
    // group by priority
    const byPri = {};
    for (const it of rows) (byPri[it.priority] = byPri[it.priority] || []).push(it);
    const order = ['P0','P1','P2','P3'];
    for (const p of order) {
      if (!byPri[p]) continue;
      const hdr = document.createElement('tr');
      hdr.className = 'section-header';
      hdr.innerHTML = `<td colspan="5">${p} — ${byPri[p].length}</td>`;
      boardBody.appendChild(hdr);
      for (const it of byPri[p]) boardBody.appendChild(itemRow(it, claimByItem, multiRepo(rows)));
    }
    // priorities outside P0-P3
    for (const p of Object.keys(byPri)) {
      if (order.includes(p)) continue;
      const hdr = document.createElement('tr');
      hdr.className = 'section-header';
      hdr.innerHTML = `<td colspan="5">${escapeHtml(p || 'unprioritized')} — ${byPri[p].length}</td>`;
      boardBody.appendChild(hdr);
      for (const it of byPri[p]) boardBody.appendChild(itemRow(it, claimByItem, multiRepo(rows)));
    }
  } else {
    for (const it of rows) boardBody.appendChild(itemRow(it, claimByItem, multiRepo(rows)));
  }

  // re-apply selection
  if (selectedId) setSelected(selectedId);
}

// multiRepo returns true when `rows` carry items from more than one
// distinct repo_id. The repo-id badge is rendered on every row only in
// that case so the single-repo dashboard stays uncluttered while the
// daemon-spawned-by-launchd workspace view shows where each item lives.
function multiRepo(rows) {
  const seen = new Set();
  for (const r of rows) {
    if (r && r.repo_id) seen.add(r.repo_id);
    if (seen.size > 1) return true;
  }
  return false;
}

function itemRow(it, claimByItem, showRepo = false) {
  const tr = document.createElement('tr');
  tr.dataset.item = it.id;
  const claim = it.claim || claimByItem.get(it.id);
  if (claim) tr.dataset.claimed = 'true';
  if (selectedId === it.id) tr.dataset.selected = 'true';
  if (activeTab === 'ready' && !claim) {
    tr.draggable = true;
    tr.dataset.draggable = 'ready';
    tr.addEventListener('dragstart', (e) => {
      e.dataTransfer.effectAllowed = 'move';
      e.dataTransfer.setData('text/plain', it.id);
      e.dataTransfer.setData('application/x-squad-item', it.id);
      tr.classList.add('dragging');
      document.body.classList.add('dnd-item-active');
    });
    tr.addEventListener('dragend', () => {
      tr.classList.remove('dragging');
      document.body.classList.remove('dnd-item-active');
      agentsList.querySelectorAll('.agent-row[data-drop-target]').forEach((el) => {
        el.removeAttribute('data-drop-target');
      });
    });
  }

  const pct = it.progress_pct || 0;
  const pri = priClass(it.priority);
  const badges = [];
  if (showRepo && it.repo_id) badges.push(`<span class="row-badge repo" title="repo">${escapeHtml(it.repo_id)}</span>`);
  if (it.epic)              badges.push(`<span class="row-badge epic" title="epic">${escapeHtml(it.epic)}</span>`);
  if (it.parallel)          badges.push(`<span class="row-badge parallel" title="parallel-safe">∥</span>`);
  const evCount = (it.evidence_required || []).length;
  if (evCount)              badges.push(`<span class="row-badge evidence" title="evidence required">📎${evCount}</span>`);
  const titleHtml = `${escapeHtml(it.title || '—')}${badges.length ? ' <span class="row-badges">' + badges.join('') + '</span>' : ''}`;
  tr.innerHTML =
    `<td class="cell-id">${escapeHtml(it.id)}</td>` +
    `<td class="cell-pri"><span class="pri-pip ${pri}">${escapeHtml((it.priority || '').replace(/^P/, ''))}</span></td>` +
    `<td class="cell-title">${titleHtml}</td>` +
    `<td class="cell-claim"></td>` +
    `<td class="cell-prog">${pct ? pct + '%' : ''}</td>`;

  const claimCell = tr.querySelector('.cell-claim');
  if (it.status === 'done') {
    const dateSpan = document.createElement('span');
    dateSpan.className = 'claim-name';
    dateSpan.textContent = it.updated || it.created || '—';
    claimCell.appendChild(dateSpan);
  } else if (claim) {
    const agentId = claim.agent_id || '';
    const pretty = displayName(agentId, claim.display_name || claim.DisplayName);
    const ico = identicon(agentId, { size: 18, name: pretty });
    claimCell.appendChild(ico);
    const nameSpan = document.createElement('span');
    nameSpan.className = 'claim-name';
    nameSpan.textContent = pretty;
    claimCell.appendChild(nameSpan);
    // sparkline (populated lazily by fetchActivity once recentActivityForItem is primed)
    const pctActivity = cachedActivity.get(it.id);
    if (pctActivity && pctActivity.length) {
      const buckets = bucketize(pctActivity, 30 * 60, 10);
      const s = sparkline(buckets, { width: 50, height: 10, color: 'var(--accent)' });
      s.classList.add('row-sparkline');
      claimCell.appendChild(s);
    }
  }

  tr.addEventListener('click', () => onItemClick(it.id));
  return tr;
}

const cachedActivity = new Map();  // item_id -> [ts, ts, ...]
export function recordActivityTs(itemId, ts) {
  if (!itemId) return;
  const arr = cachedActivity.get(itemId) || [];
  arr.push(ts);
  // keep only last hour
  const cutoff = Math.floor(Date.now() / 1000) - 3600;
  const trimmed = arr.filter((t) => t >= cutoff);
  cachedActivity.set(itemId, trimmed);
}
export async function primeActivity(itemIds) {
  await Promise.all(itemIds.map(async (id) => {
    try {
      const events = await fetchJSON('/api/items/' + encodeURIComponent(id) + '/activity?limit=80');
      cachedActivity.set(id, events.map((e) => e.ts).filter(Boolean));
    } catch {}
  }));
}

function renderAgents() {
  const { agents, claims } = cached;
  agentsList.innerHTML = '';
  agentsCount.textContent = agents.length;

  const byAgent = new Map(claims.map((c) => [c.agent_id, c]));

  // sort: active first, then idle, then stopped; active agents with claims first
  const sorted = [...agents].sort((a, b) => {
    const score = (x) => {
      const st = x.status || x.Status || 'offline';
      const hasClaim = !!(x.claim_item || x.ClaimItem);
      if (st === 'active' && hasClaim) return 0;
      if (st === 'active') return 1;
      if (st === 'idle')   return 2;
      return 3;
    };
    return score(a) - score(b);
  });

  for (const a of sorted) {
    const li = document.createElement('li');
    li.className = 'agent-row';
    const agentId = a.agent_id || a.AgentID || '';
    const rawName = a.display_name || a.DisplayName || '';
    const prettyName = displayName(agentId, rawName);
    const status = a.status || a.Status || 'offline';
    const lastTick = a.last_tick || a.LastTick || 0;
    const claimItem = a.claim_item || a.ClaimItem || '';
    const intent = a.intent || a.Intent || '';

    const claim = byAgent.get(agentId) || (claimItem ? { item_id: claimItem, intent } : null);

    let dotState = 'stopped';
    if (status === 'active') dotState = 'active';
    else if (status === 'idle') dotState = 'idle';

    const ago = lastTick ? fmtAgo(lastTick) : '—';

    const icon = identicon(agentId, { size: 22, name: prettyName });
    icon.classList.add('agent-avatar');

    const body = document.createElement('div');
    body.className = 'agent-body';
    body.innerHTML =
      `<div class="agent-line">
        <span class="agent-name" title="${escapeHtml(agentId)}">${escapeHtml(prettyName)}</span>
        <span class="agent-dot" data-state="${dotState}" title="${status}"></span>
      </div>` +
      (claim
        ? `<div class="agent-meta"><span class="agent-claim">${escapeHtml(claim.item_id)}</span> ${escapeHtml(claim.intent || '')}</div>`
        : `<div class="agent-meta">seen ${ago} ago</div>`);

    li.appendChild(icon);
    li.appendChild(body);
    li.dataset.agentId = agentId;
    li.dataset.state = dotState;
    if (dotState === 'idle') {
      wireIdleDrop(li, agentId, prettyName);
    }

    li.addEventListener('click', () => {
      openAgentDetail(a, claim);
    });
    agentsList.appendChild(li);
  }
}

function wireIdleDrop(li, agentId, prettyName) {
  li.addEventListener('dragenter', (e) => {
    if (!isItemDrag(e)) return;
    e.preventDefault();
    li.dataset.dropTarget = 'true';
  });
  li.addEventListener('dragover', (e) => {
    if (!isItemDrag(e)) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = 'move';
  });
  li.addEventListener('dragleave', (e) => {
    if (e.target !== li) return;
    li.removeAttribute('data-drop-target');
  });
  li.addEventListener('drop', (e) => {
    if (!isItemDrag(e)) return;
    e.preventDefault();
    li.removeAttribute('data-drop-target');
    const itemId = e.dataTransfer.getData('application/x-squad-item') ||
                   e.dataTransfer.getData('text/plain');
    if (!itemId) return;
    assignItemToAgent(itemId, agentId, prettyName);
  });
}

function isItemDrag(e) {
  const dt = e.dataTransfer;
  if (!dt) return false;
  return Array.from(dt.types || []).includes('application/x-squad-item');
}

async function assignItemToAgent(itemId, agentId, agentName) {
  try {
    await postJSON('/api/items/' + encodeURIComponent(itemId) + '/assign',
      { agent_id: agentId });
    toast({ kind: 'ok', title: 'Assigned', body: itemId + ' → ' + agentName });
  } catch (err) {
    toast({ kind: 'warn', title: 'Assign failed', body: String(err.message || err) });
  }
}

// ---- filter bar ---------------------------------------------------------

const filtersEl = document.getElementById('board-filters');
const searchEl  = document.getElementById('board-search');

export function renderFilters() {
  if (!filtersEl) return;
  const { items } = cached;
  const areas = [...new Set(items.map((i) => i.area).filter(Boolean))].sort();
  const types = [...new Set(items.map((i) => i.type).filter(Boolean))].sort();
  const pris  = ['P0','P1','P2','P3'];

  const chip = (label, key, value) => {
    const active = filter[key] === value;
    return `<button class="chip${active ? ' active' : ''}" data-key="${key}" data-value="${escapeHtml(value)}">${escapeHtml(label)}</button>`;
  };

  filtersEl.innerHTML =
    `<div class="chip-group"><span class="chip-group-label">PRI</span>${pris.map(p => chip(p, 'priority', p)).join('')}</div>` +
    `<div class="chip-group"><span class="chip-group-label">AREA</span>${areas.map(a => chip(a, 'area', a)).join('')}</div>` +
    `<div class="chip-group"><span class="chip-group-label">TYPE</span>${types.map(t => chip(t, 'type', t)).join('')}</div>` +
    (hasActiveFilter() ? `<button class="chip chip-clear" id="clear-filters">CLEAR</button>` : '');

  filtersEl.querySelectorAll('.chip[data-key]').forEach((b) => {
    b.addEventListener('click', () => {
      const k = b.dataset.key;
      const v = b.dataset.value;
      setFilter({ [k]: filter[k] === v ? '' : v });
      renderFilters();
    });
  });
  filtersEl.querySelector('#clear-filters')?.addEventListener('click', () => {
    setFilter({ q: '', area: '', type: '', priority: '' });
    if (searchEl) searchEl.value = '';
    renderFilters();
  });
}

function hasActiveFilter() {
  return filter.q || filter.area || filter.type || filter.priority;
}

if (searchEl) {
  searchEl.addEventListener('input', () => setFilter({ q: searchEl.value }));
}
