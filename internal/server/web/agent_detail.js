import { fetchJSON, escapeHtml, fmtAgo, token } from './util.js';
import { displayName } from './names.js';
import { identicon } from './identicon.js';

// MAX_TIMELINE_ROWS bounds the in-memory cache and (transitively) the DOM.
// 500 keeps the drawer responsive even after a long agent session — the
// initial fetch caps at 50, SSE appends thereafter.
const MAX_TIMELINE_ROWS = 500;

let modalEl;
let currentAgentId = null;
let lastEventsCache = null;
let renderTimelineCb = (_events) => {};
let onClaimNavigate = (_itemId) => {};
let liveES = null;

export function setTimelineRenderer(fn) {
  renderTimelineCb = typeof fn === 'function' ? fn : () => {};
  // Replay cached events when the renderer registers after the drawer is
  // already open and the fetch has resolved — first paint without race.
  if (modalEl && !modalEl.hidden && lastEventsCache) {
    const body = modalEl.querySelector('#agent-detail-body');
    if (body) {
      body.innerHTML = '';
      const id = currentAgentId;
      renderTimelineCb(lastEventsCache, body, id, () => fetchAndCache(id));
    }
  }
}

export function setClaimNavigateHandler(fn) {
  onClaimNavigate = typeof fn === 'function' ? fn : () => {};
}

export function openAgentDetail(agent, claim) {
  const el = ensureDrawer();
  const agentId = agent?.agent_id || agent?.AgentID || '';
  if (!agentId) return;
  if (!el.hidden && currentAgentId === agentId) return;

  currentAgentId = agentId;
  lastEventsCache = null;
  renderHeader(el, agent, claim);

  if (el.hidden) {
    el.hidden = false;
    requestAnimationFrame(() => el.classList.add('show'));
  }
  fetchAndCache(agentId);
  startLiveStream(agentId);
}

function close() {
  if (!modalEl) return;
  stopLiveStream();
  modalEl.classList.remove('show');
  setTimeout(() => {
    if (modalEl) modalEl.hidden = true;
  }, 180);
  currentAgentId = null;
}

// startLiveStream opens a dedicated EventSource for the open drawer. The
// drawer owns this connection — it is opened on openAgentDetail and closed
// in stopLiveStream when the drawer closes (matching AC for resource
// cleanup). Auth posture is inherited from /api/sse — same cookie/token
// surface as the global stream.
function startLiveStream(agentId) {
  stopLiveStream();
  if (typeof EventSource === 'undefined') return;
  // Inherit the same /api/events route + token-in-query-string posture the
  // global EventSource uses (app.js). EventSource cannot set Authorization
  // headers, so token-protected dashboards rely on the query-string fallback.
  const url = '/api/events' + (token ? '?token=' + encodeURIComponent(token) : '');
  liveES = new EventSource(url);
  const onSSE = (e, sseKind) => {
    if (currentAgentId !== agentId) return;
    let env;
    try { env = JSON.parse(e.data); } catch { return; }
    const payload = env.Payload || env.payload || env;
    if (!payload || payload.agent_id !== agentId) return;
    const row = toTimelineRow(payload, sseKind);
    if (row) appendLiveRow(row);
  };
  liveES.addEventListener('agent_activity', (e) => onSSE(e, 'agent_activity'));
  liveES.addEventListener('message', (e) => onSSE(e, 'message'));
  liveES.addEventListener('item_changed', (e) => onSSE(e, 'item_changed'));
}

// toTimelineRow converts an SSE event payload into the row shape
// agent_timeline.js's classify() and renderRow() expect. Each pump
// publishes a different schema; the renderer is shared. Returns null when
// the event isn't representable on the timeline.
function toTimelineRow(p, sseKind) {
  if (sseKind === 'agent_activity') {
    // activityPump already emits timeline-row-shaped payloads.
    return p;
  }
  if (sseKind === 'message') {
    if (p.kind === 'progress') {
      return {
        kind: 'progress',
        source: 'progress',
        agent_id: p.agent_id,
        ts: Math.floor(Date.now() / 1000),
        item_id: p.thread || '',
        body: p.body || '',
      };
    }
    return {
      kind: 'chat',
      source: 'chat',
      agent_id: p.agent_id,
      ts: Math.floor(Date.now() / 1000),
      body: p.body || '',
      outcome: p.kind || 'say',
      thread: p.thread || '',
    };
  }
  if (sseKind === 'item_changed') {
    // claimsPump kinds: "claimed" | "released" | "reassigned" | "done" | "blocked"
    if (p.kind === 'blocked') {
      return {
        kind: 'blocked',
        source: 'blocked',
        agent_id: p.agent_id,
        ts: p.released_at || p.claimed_at || Math.floor(Date.now() / 1000),
        item_id: p.item_id || '',
        outcome: p.outcome || '',
      };
    }
    let kind = 'claim';
    if (p.kind === 'released') kind = 'release';
    else if (p.kind === 'done') kind = 'done';
    return {
      kind,
      source: kind === 'release' ? 'release' : 'claim',
      agent_id: p.agent_id,
      ts: p.claimed_at || p.released_at || Math.floor(Date.now() / 1000),
      item_id: p.item_id || '',
      intent: p.intent || '',
      outcome: p.outcome || '',
    };
  }
  return null;
}

function stopLiveStream() {
  if (liveES) {
    liveES.close();
    liveES = null;
  }
}

// appendLiveRow prepends a freshly-arrived row to the cached event list,
// caps at MAX_TIMELINE_ROWS, and asks the registered renderer to repaint.
// The renderer's existing filter-chip state is preserved — a hidden chip
// for the new row's kind keeps it hidden until the operator toggles back.
function appendLiveRow(payload) {
  if (!modalEl || modalEl.hidden) return;
  const body = modalEl.querySelector('#agent-detail-body');
  if (!body) return;
  if (!Array.isArray(lastEventsCache)) lastEventsCache = [];
  lastEventsCache = [payload, ...lastEventsCache];
  if (lastEventsCache.length > MAX_TIMELINE_ROWS) {
    lastEventsCache = lastEventsCache.slice(0, MAX_TIMELINE_ROWS);
  }
  body.innerHTML = '';
  renderTimelineCb(lastEventsCache, body, currentAgentId, () => fetchAndCache(currentAgentId));
}

function ensureDrawer() {
  if (modalEl) return modalEl;
  modalEl = document.createElement('div');
  modalEl.className = 'action-modal agent-detail-drawer';
  modalEl.hidden = true;
  modalEl.innerHTML = `
    <div class="action-modal-backdrop" data-close></div>
    <aside class="action-modal-panel agent-detail-panel" role="dialog" aria-label="Agent detail">
      <header class="action-modal-head">
        <div class="agent-detail-head" id="agent-detail-head"></div>
        <button class="icon-btn" data-close aria-label="Close">✕</button>
      </header>
      <div class="action-modal-body agent-detail-body" id="agent-detail-body">
        <div class="agent-detail-spinner" aria-busy="true">loading…</div>
      </div>
    </aside>`;
  document.body.appendChild(modalEl);
  modalEl.addEventListener('click', (e) => {
    if (e.target.closest('[data-close]')) {
      close();
      return;
    }
    const navEl = e.target.closest('[data-claim-id]');
    if (navEl) {
      const itemId = navEl.getAttribute('data-claim-id');
      if (itemId) {
        close();
        onClaimNavigate(itemId);
      }
    }
  });
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && modalEl && !modalEl.hidden) close();
  });
  return modalEl;
}

function renderHeader(el, agent, claim) {
  const head = el.querySelector('#agent-detail-head');
  if (!head) return;
  const agentId = agent?.agent_id || agent?.AgentID || '';
  const rawName = agent?.display_name || agent?.DisplayName || '';
  const prettyName = displayName(agentId, rawName);
  const lastTick = agent?.last_tick_at || agent?.last_tick || agent?.LastTick || 0;
  const ago = lastTick ? fmtAgo(lastTick) + ' ago' : 'never';

  const claimHtml = claim?.item_id
    ? `<a class="agent-detail-claim" href="#" data-claim-id="${escapeHtml(claim.item_id)}">${escapeHtml(claim.item_id)}${claim.intent ? ' — ' + escapeHtml(claim.intent) : ''}</a>`
    : `<span class="agent-detail-claim unclaimed">unclaimed</span>`;

  head.innerHTML = '';
  head.appendChild(identicon(agentId, { size: 22, name: prettyName }));
  const meta = document.createElement('div');
  meta.className = 'agent-detail-meta';
  meta.innerHTML =
    `<div class="agent-detail-name" title="${escapeHtml(agentId)}">${escapeHtml(prettyName)}</div>` +
    `<div class="agent-detail-sub">${claimHtml} · seen ${escapeHtml(ago)}</div>`;
  head.appendChild(meta);
}

async function fetchAndCache(agentId) {
  const body = modalEl.querySelector('#agent-detail-body');
  if (!body) return;
  body.innerHTML = '<div class="agent-detail-spinner" aria-busy="true">loading…</div>';
  try {
    const res = await fetchJSON(`/api/agents/${encodeURIComponent(agentId)}/timeline?limit=50`);
    if (currentAgentId !== agentId) return;
    lastEventsCache = res?.timeline || [];
    body.innerHTML = '';
    renderTimelineCb(lastEventsCache, body, agentId, () => fetchAndCache(agentId));
  } catch (err) {
    if (currentAgentId !== agentId) return;
    body.innerHTML = `
      <div class="agent-detail-error">
        couldn't load timeline: ${escapeHtml(err.message || String(err))}
        <button class="action-btn" data-detail-retry>Retry</button>
      </div>`;
    body.querySelector('[data-detail-retry]')?.addEventListener('click', () => fetchAndCache(agentId));
  }
}
