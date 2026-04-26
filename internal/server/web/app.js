// app.js — entry: SSE wiring, bootstrap, cross-module coordination

import { fetchJSON, token, clock, setAgentHeader } from './util.js';
import { displayName } from './names.js';
import { refreshBoard, refreshClaimsAndAgents, setItemClickHandler, setSelected, claimsSnapshot, itemsSnapshot, agentsSnapshot, recordActivityTs, setMeAgentId } from './board.js';
import { refreshMessages, appendLive, populateThreads, setThread, setAgentsProvider } from './chat.js';
import { openItem, closeDrawer, currentItemId, setOnCloseHandler, onEvent, refreshHotness } from './drawer.js';
import { initPalette, open as openPalette } from './palette.js';
import { initToasts, toast } from './toasts.js';
import { initShortcuts } from './shortcuts.js';
import { wireAutolinkClicks } from './autolink.js';
import { initInsights, open as openInsights } from './insights.js';
import { initDepGraph, openAsync as openDepGraph } from './depgraph.js';
import { initSavedViews } from './savedviews.js';
import { openNewItemModal, setOnMutated as setOnActionMutated } from './actions.js';
import { initSidebar } from './sidebar.js';
import { initInbox, refreshCount as refreshInboxCount } from './inbox.js';
import { initLearnings } from './learnings.js';

// clock
clock(document.getElementById('clock'));

// me — resolved before anything that could mutate so X-Squad-Agent is set
let meAgentId = '';
const whoamiReady = (async () => {
  try {
    const me = await fetchJSON('/api/whoami');
    const userEl = document.getElementById('user');
    userEl.textContent = me?.agent_id
      ? displayName(me.agent_id, me.display_name)
      : 'anon';
    if (me?.agent_id) {
      userEl.dataset.agentId = me.agent_id;
      setMeAgentId(me.agent_id);
      setAgentHeader(me.agent_id);
      meAgentId = me.agent_id;
    }
  } catch {
    document.getElementById('user').textContent = 'anon';
  }
})();
export function getMeAgentId() { return meAgentId; }

// wire board → drawer
setItemClickHandler((id) => {
  openItem(id);
  setSelected(id);
});
setOnCloseHandler(() => setSelected(null));

// toasts + shortcuts + palette + autolink + insights + depgraph + views
initToasts();
wireAutolinkClicks();
setAgentsProvider(agentsSnapshot);
initSavedViews();
initInsights({ onOpenItem: (id) => { openItem(id); setSelected(id); } });
initDepGraph({ onOpen: (id) => { openItem(id); setSelected(id); } });
document.getElementById('palette-trigger')?.addEventListener('click', openPalette);
document.getElementById('insights-btn')?.addEventListener('click', openInsights);
document.getElementById('depgraph-btn')?.addEventListener('click', openDepGraph);
document.getElementById('new-item-btn')?.addEventListener('click', openNewItemModal);
initSidebar({ onItem: (id) => { openItem(id); setSelected(id); } });
initInbox({ onChange: () => { refreshBoard().then(refreshChrome); } });
initLearnings();

// after any item mutation: refresh board + drawer; optionally open the new item.
setOnActionMutated(async (id, opts) => {
  await refreshBoard();
  refreshChrome();
  if (id) {
    if (opts?.open) {
      openItem(id);
      setSelected(id);
    } else if (currentItemId() === id) {
      // re-open to refresh drawer body
      openItem(id);
    }
  }
});

// global keys for stats + graph (outside typing context)
document.addEventListener('keydown', (e) => {
  const t = e.target;
  if (t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.isContentEditable)) return;
  if (e.key === 's' && !e.metaKey && !e.ctrlKey) { e.preventDefault(); openInsights(); }
  if (e.key === 'g' && !e.metaKey && !e.ctrlKey) { e.preventDefault(); openDepGraph(); }
});
initShortcuts({
  openPalette,
  openItem: (id) => { openItem(id); setSelected(id); },
  closeDrawer: () => { closeDrawer(); setSelected(null); },
  focusChat: () => document.getElementById('compose-input')?.focus(),
  focusSearch: () => document.getElementById('board-search')?.focus(),
});
initPalette({
  getData: () => ({
    items: itemsSnapshot(),
    agents: agentsSnapshot(),
    actions: [
      { label: 'Open Insights',      hint: 's', run: () => openInsights() },
      { label: 'Open Dependency graph', hint: 'g', run: () => openDepGraph() },
      { label: 'Close drawer',       hint: 'esc', run: () => { closeDrawer(); setSelected(null); } },
      { label: 'Focus chat',         hint: 'c',   run: () => document.getElementById('compose-input')?.focus() },
      { label: 'Focus search',       hint: '/',   run: () => document.getElementById('board-search')?.focus() },
      { label: 'Show shortcuts',     hint: '?',   run: () => document.getElementById('shortcut-overlay')?.removeAttribute('hidden') },
    ],
  }),
  onSelect: (r) => {
    if (r.kind === 'item') { openItem(r.item.id); setSelected(r.item.id); }
    else if (r.kind === 'agent') {
      const claim = r.agent.claim_item || r.agent.ClaimItem;
      if (claim) { openItem(claim); setSelected(claim); }
    }
    else if (r.kind === 'action') { r.action.run(); }
  },
});

// let drawer navigate on autolink clicks
document.addEventListener('sf:open-item', (e) => {
  const id = e.detail?.id;
  if (id) { openItem(id); setSelected(id); }
});

// initial loads
(async () => {
  await whoamiReady;
  await refreshBoard();
  populateThreads(claimsSnapshot());
  await refreshMessages();
  updateStatsRibbon();
  // deep link: #BUG-060 opens that item
  const hash = location.hash.replace(/^#/, '');
  if (/^[A-Z]{2,6}-\d{1,5}$/.test(hash)) {
    openItem(hash);
    setSelected(hash);
  }
})();

window.addEventListener('hashchange', () => {
  const hash = location.hash.replace(/^#/, '');
  if (/^[A-Z]{2,6}-\d{1,5}$/.test(hash)) {
    openItem(hash);
    setSelected(hash);
  }
});

function updateStatsRibbon() {
  const el = document.getElementById('stats-ribbon');
  if (!el) return;
  const items = itemsSnapshot();
  const claims = claimsSnapshot();
  const agents = agentsSnapshot();

  const claimedIds = new Set(claims.map((c) => c.item_id));
  const active = agents.filter((a) => (a.status || a.Status) === 'active').length;
  const open   = items.filter((i) => i.status !== 'done' && !claimedIds.has(i.id)).length;
  const blocked = items.filter((i) => i.status === 'blocked').length;
  const p0 = items.filter((i) => i.priority === 'P0' && i.status !== 'done').length;
  const p1 = items.filter((i) => i.priority === 'P1' && i.status !== 'done').length;

  el.innerHTML =
    `<span class="stat"><span class="stat-dot active"></span><span class="stat-val">${active}</span><span class="stat-label">active</span></span>` +
    `<span class="stat"><span class="stat-dot accent"></span><span class="stat-val">${claims.length}</span><span class="stat-label">in flight</span></span>` +
    `<span class="stat"><span class="stat-val">${open}</span><span class="stat-label">open</span></span>` +
    `<span class="stat"><span class="stat-val">${p0}</span><span class="stat-label p0">P0</span></span>` +
    `<span class="stat"><span class="stat-val">${p1}</span><span class="stat-label p1">P1</span></span>` +
    (blocked ? `<span class="stat"><span class="stat-val">${blocked}</span><span class="stat-label danger">blocked</span></span>` : '');
}

// refresh stats on any meaningful change
function refreshChrome() {
  updateStatsRibbon();
}
export { refreshChrome };

// keyboard
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape' && currentItemId()) {
    closeDrawer();
    setSelected(null);
  }
});

// --------- SSE ---------

const connDot = document.getElementById('conn-dot');
const connLabel = document.getElementById('conn-label');

function setConn(state) {
  connDot.dataset.state = state;
  connLabel.textContent = state;
  const banner = document.getElementById('auth-banner');
  if (banner) banner.hidden = state !== 'auth-failed';
}

let authProbing = false;
async function probeAuth() {
  if (authProbing) return;
  // once we've latched auth-failed, EventSource's retry storm will keep firing
  // onerror — don't keep polling whoami until onopen flips us back.
  if (connDot.dataset.state === 'auth-failed') return;
  authProbing = true;
  try {
    const r = await fetch('/api/whoami', { headers: token ? { Authorization: 'Bearer ' + token } : {} });
    if (r.status === 401 || r.status === 403) setConn('auth-failed');
  } catch {
    // probe itself unreachable — stay 'disconnected'.
  } finally {
    authProbing = false;
  }
}

let currentES = null;
function disconnectSSE() {
  if (currentES) {
    try { currentES.close(); } catch {}
    currentES = null;
  }
}

function connectSSE() {
  if (currentES) return;
  const url = '/api/events' + (token ? '?token=' + encodeURIComponent(token) : '');
  const es = new EventSource(url);
  currentES = es;
  setConn('connecting');

  es.onopen = () => setConn('connected');
  es.onerror = () => {
    setConn('disconnected');
    // 401/403 from /api/events looks identical to a network drop on the
    // EventSource API. Probe whoami so we can flip to 'auth-failed' when
    // it really is auth — leave 'disconnected' for transient drops so
    // EventSource's built-in retry is what the user sees.
    probeAuth();
  };

  // Server publishes item lifecycle as a single `item_changed` envelope with
  // payload.kind ∈ {claimed, released, done, blocked, force_released, reassigned}.
  // Map to the SPA's drawer/board vocabulary (claim/release/done/blocked/reassign).
  const ITEM_CHANGED_TO_SPA = {
    claimed:        'claim',
    released:       'release',
    done:           'done',
    blocked:        'blocked',
    force_released: 'release',
    reassigned:     'reassign',
  };

  es.addEventListener('item_changed', (e) => {
    const d = JSON.parse(e.data);
    const p = d.payload || {};
    const inner = p.kind || '';
    const spaKind = ITEM_CHANGED_TO_SPA[inner] || inner;
    onEvent(p, spaKind);
    if (p.item_id) recordActivityTs(p.item_id, p.ts || Math.floor(Date.now() / 1000));
    refreshBoard().then(refreshChrome);
    if (spaKind === 'claim') {
      populateThreadsSoon();
      toastClaim(p);
    } else if (spaKind === 'release') {
      populateThreadsSoon();
    } else if (spaKind === 'done') {
      toastDone(p);
    }
  });

  es.addEventListener('agent_status', () => {
    refreshClaimsAndAgents();
  });

  es.addEventListener('inbox_changed', () => {
    refreshInboxCount();
  });

  es.addEventListener('message', (e) => {
    const d = JSON.parse(e.data);
    const kind = d.payload.kind || 'say';
    appendLive(d.payload);
    onEvent(d.payload, kind);
    if (d.payload.item_id) recordActivityTs(d.payload.item_id, d.payload.ts || Math.floor(Date.now() / 1000));
    if (d.payload.thread && d.payload.thread !== 'global') recordActivityTs(d.payload.thread, d.payload.ts || Math.floor(Date.now() / 1000));
    if (kind === 'knock') toastKnock(d.payload);
    else                  toastMessage(d.payload);
  });
}

function toastClaim(p) {
  toast({
    kind: 'info',
    title: `${p.agent_id} claimed ${p.item_id}`,
    body: p.intent || '',
    onClick: () => { openItem(p.item_id); setSelected(p.item_id); },
  });
}
function toastDone(p) {
  toast({
    kind: 'ok',
    title: `${p.item_id} shipped`,
    body: `by ${p.agent_id}`,
    onClick: () => { openItem(p.item_id); setSelected(p.item_id); },
  });
}
function toastMessage(p) {
  const me = document.getElementById('user')?.textContent?.trim();
  if (!me) return;
  const body = p.body || '';
  const mentioned = (body.match(/@(\S+)/g) || []).some((t) => t.slice(1) === me);
  if (!mentioned) return;
  toast({
    kind: 'mention',
    title: `@${me} · ${p.agent_id} in #${p.thread}`,
    body,
    onClick: () => {
      if (p.thread && p.thread !== 'global') { openItem(p.thread); setSelected(p.thread); }
    },
  });
}
function toastKnock(p) {
  toast({
    kind: 'knock',
    title: `KNOCK from ${p.agent_id}`,
    body: p.body,
    ttl: 15000,
  });
}

let threadRefreshTimer = null;
function populateThreadsSoon() {
  clearTimeout(threadRefreshTimer);
  threadRefreshTimer = setTimeout(() => populateThreads(claimsSnapshot()), 200);
}

connectSSE();

window.addEventListener('beforeunload', disconnectSSE);
document.addEventListener('visibilitychange', () => {
  if (document.visibilityState === 'hidden') disconnectSSE();
  else connectSSE();
});
