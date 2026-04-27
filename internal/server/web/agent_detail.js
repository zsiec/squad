import { fetchJSON, escapeHtml, fmtAgo } from './util.js';
import { displayName } from './names.js';
import { identicon } from './identicon.js';

let modalEl;
let currentAgentId = null;
let lastEventsCache = null;
let renderTimelineCb = (_events) => {};
let onClaimNavigate = (_itemId) => {};

export function setTimelineRenderer(fn) {
  renderTimelineCb = typeof fn === 'function' ? fn : () => {};
  // Replay cached events when the renderer registers after the drawer is
  // already open and the fetch has resolved — first paint without race.
  if (modalEl && !modalEl.hidden && lastEventsCache) {
    renderTimelineCb(lastEventsCache);
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
}

function close() {
  if (!modalEl) return;
  modalEl.classList.remove('show');
  setTimeout(() => {
    if (modalEl) modalEl.hidden = true;
  }, 180);
  currentAgentId = null;
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
    const res = await fetchJSON(`/api/agents/${encodeURIComponent(agentId)}/events?limit=50`);
    if (currentAgentId !== agentId) return;
    lastEventsCache = res?.events || [];
    renderPlaceholder(body, lastEventsCache);
    renderTimelineCb(lastEventsCache);
  } catch (err) {
    if (currentAgentId !== agentId) return;
    body.innerHTML = `<div class="agent-detail-error">${escapeHtml(err.message || String(err))}</div>`;
  }
}

function renderPlaceholder(host, events) {
  if (!events.length) {
    host.innerHTML = '<div class="agent-detail-empty">no events yet</div>';
    return;
  }
  host.innerHTML = `<div class="agent-detail-placeholder">events ready (${events.length}) — timeline renders next</div>`;
}
