import { fetchJSON, escapeHtml, fmtAgo } from './util.js';
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
  startDiffPoll(agentId);
}

function close() {
  if (!modalEl) return;
  stopLiveStream();
  stopDiffPoll();
  modalEl.classList.remove('show');
  setTimeout(() => {
    if (modalEl) modalEl.hidden = true;
  }, 180);
  currentAgentId = null;
}

let diffPollTimer = null;
const DIFF_POLL_MS = 20_000;

// startDiffPoll fetches /api/agents/<id>/diff immediately, then every
// 20s while the drawer is open. The watcher follows the agent's
// progress without leaving the browser; a heavier real-time path
// (SSE-driven) was considered and dropped — 20s is fine for babysitting
// and keeps the implementation surface small.
function startDiffPoll(agentId) {
  stopDiffPoll();
  fetchAndRenderDiff(agentId);
  diffPollTimer = setInterval(() => {
    if (currentAgentId !== agentId) {
      stopDiffPoll();
      return;
    }
    fetchAndRenderDiff(agentId);
  }, DIFF_POLL_MS);
}

function stopDiffPoll() {
  if (diffPollTimer) {
    clearInterval(diffPollTimer);
    diffPollTimer = null;
  }
}

async function fetchAndRenderDiff(agentId) {
  const filesEl = modalEl?.querySelector('#agent-detail-diff-files');
  const metaEl = modalEl?.querySelector('#agent-detail-diff-meta');
  const targetEl = modalEl?.querySelector('#agent-detail-diff-target');
  if (!filesEl) return;
  try {
    const res = await fetchJSON(`/api/agents/${encodeURIComponent(agentId)}/diff`);
    if (currentAgentId !== agentId) return;
    if (targetEl && res.merge_target) targetEl.textContent = res.merge_target;
    const files = Array.isArray(res.files) ? res.files : [];
    if (metaEl) metaEl.textContent = files.length ? `${files.length} file${files.length === 1 ? '' : 's'}` : 'no changes';
    // The drawer widens when there's anything to look at. Without files
    // the timeline-only width is plenty; with files the unified diff
    // wraps painfully under 720px.
    const panel = modalEl?.querySelector('.action-modal-panel');
    if (panel) panel.dataset.mode = files.length ? 'diff' : '';
    if (files.length === 0) {
      filesEl.innerHTML = `<div class="agent-detail-diff-empty">no changes vs ${escapeHtml(res.merge_target || 'main')}</div>`;
      return;
    }
    filesEl.innerHTML = files.map(renderDiffFile).join('');
    filesEl.querySelectorAll('[data-toggle-file]').forEach((btn) => {
      btn.addEventListener('click', (e) => {
        const block = e.currentTarget.closest('.agent-detail-diff-file');
        if (block) block.classList.toggle('collapsed');
      });
    });
  } catch (err) {
    if (currentAgentId !== agentId) return;
    filesEl.innerHTML = `<div class="agent-detail-diff-error">couldn't load diff: ${escapeHtml(err.message || String(err))}</div>`;
  }
}

function renderDiffFile(f) {
  const status = f.status || 'modified';
  const statusClass = `agent-detail-diff-status status-${escapeHtml(status)}`;
  return `<div class="agent-detail-diff-file" data-status="${escapeHtml(status)}">
    <button class="agent-detail-diff-filehead" data-toggle-file aria-label="Toggle file">
      <span class="${statusClass}">${escapeHtml(status)}</span>
      <span class="agent-detail-diff-filepath">${escapeHtml(f.path || '')}</span>
      <span class="agent-detail-diff-toggle" aria-hidden="true">▾</span>
    </button>
    <div class="agent-detail-diff-hunks">${renderDiffLines(f.hunks || '')}</div>
  </div>`;
}

// renderDiffLines splits the unified-diff body into per-line elements
// keyed by leading character. The class drives the +/- background tint
// and the hunk/file-header accents the watcher uses to scan the diff at
// a glance. Empty body is rendered as a single dim "(empty)" placeholder
// rather than collapsing the whole block — keeps the layout stable when
// a file row exists but its hunks string is unexpectedly missing.
function renderDiffLines(body) {
  if (!body) {
    return '<div class="diff-line diff-empty">(empty)</div>';
  }
  return body.split('\n').map(diffLineHTML).join('');
}

function diffLineHTML(line) {
  const klass = diffLineClass(line);
  return `<div class="diff-line ${klass}">${escapeHtml(line || ' ')}</div>`;
}

function diffLineClass(line) {
  if (line.startsWith('diff --git') ||
      line.startsWith('index ') ||
      line.startsWith('--- ') ||
      line.startsWith('+++ ') ||
      line.startsWith('new file mode') ||
      line.startsWith('deleted file mode') ||
      line.startsWith('similarity index') ||
      line.startsWith('rename from') ||
      line.startsWith('rename to')) {
    return 'diff-file-header';
  }
  if (line.startsWith('@@')) return 'diff-hunk-header';
  if (line.startsWith('+')) return 'diff-addition';
  if (line.startsWith('-')) return 'diff-deletion';
  return 'diff-context';
}

// startLiveStream opens a dedicated EventSource for the open drawer. The
// drawer owns this connection — it is opened on openAgentDetail and closed
// in stopLiveStream when the drawer closes (matching AC for resource
// cleanup).
function startLiveStream(agentId) {
  stopLiveStream();
  if (typeof EventSource === 'undefined') return;
  liveES = new EventSource('/api/events');
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
      <section class="agent-detail-diff" id="agent-detail-diff" aria-label="Worktree diff">
        <header class="agent-detail-diff-head">
          <span class="agent-detail-diff-title">Worktree diff vs <code id="agent-detail-diff-target">main</code></span>
          <span class="agent-detail-diff-meta" id="agent-detail-diff-meta"></span>
        </header>
        <div class="agent-detail-diff-files" id="agent-detail-diff-files">
          <div class="agent-detail-diff-empty">loading diff…</div>
        </div>
      </section>
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
