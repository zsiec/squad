import { setTimelineRenderer } from './agent_detail.js';
import { escapeHtml, fmtAgo } from './util.js';

const FILTER_KEY = 'squad.timeline.filters';
const CHIPS = [
  { id: 'chat',           label: 'chat'   },
  { id: 'claim',          label: 'claim'  },
  { id: 'progress',       label: 'progress' },
  { id: 'blocked',        label: 'blocked'  },
  { id: 'commit',         label: 'commit' },
  { id: 'attestation',    label: 'attest' },
  { id: 'pre_tool',       label: 'pre'    },
  { id: 'post_tool',      label: 'post'   },
  { id: 'subagent_start', label: 'sub-start' },
  { id: 'subagent_stop',  label: 'sub-stop'  },
  { id: 'read',           label: 'Read'   },
];
const TARGET_TRUNC = 80;

function loadFilters() {
  try {
    const raw = localStorage.getItem(FILTER_KEY);
    if (raw) return { ...defaults(), ...JSON.parse(raw) };
  } catch { /* ignore parse errors */ }
  return defaults();
}

function saveFilters(state) {
  try { localStorage.setItem(FILTER_KEY, JSON.stringify(state)); } catch { /* quota */ }
}

function defaults() {
  const out = {};
  for (const c of CHIPS) out[c.id] = true;
  return out;
}

// classify returns the two filter buckets a row belongs to: a primary kind
// (always one of the chips) and an optional secondary tool-specific kind
// ('read' for pre_tool+Read rows). A row is shown when both are enabled.
function classify(row) {
  const kind = row.kind || '';
  if (kind === 'chat')        return ['chat', null];
  if (kind === 'claim')       return ['claim', null];
  if (kind === 'release')     return ['claim', null];
  if (kind === 'done')        return ['claim', null];
  if (kind === 'progress')    return ['progress', null];
  if (kind === 'blocked')     return ['blocked', null];
  if (kind === 'commit')      return ['commit', null];
  if (kind === 'attestation') return ['attestation', null];
  if (kind === 'event') {
    const ek = row.event_kind || '';
    if (ek === 'PreToolUse')      return ['pre_tool',  row.tool === 'Read' ? 'read' : null];
    if (ek === 'PostToolUse')     return ['post_tool', null];
    if (ek === 'subagent_start')  return ['subagent_start', null];
    if (ek === 'subagent_stop')   return ['subagent_stop',  null];
  }
  return [null, null];
}

function trunc(s, n) {
  if (!s) return '';
  return s.length > n ? s.slice(0, n) + '…' : s;
}

function fmtDuration(ms) {
  if (ms < 1000) return ms + 'ms';
  if (ms < 60_000) return (ms / 1000).toFixed(1) + 's';
  return Math.round(ms / 1000) + 's';
}

function renderRow(row, primary, secondary) {
  const ts = row.ts ? fmtAgo(row.ts) + ' ago' : '—';
  const kind = row.kind || '';
  const dataAttrs = `data-classify="1" data-primary="${primary}"${secondary ? ` data-secondary="${secondary}"` : ''}`;
  switch (kind) {
    case 'chat':
      return `<div class="tl-row" ${dataAttrs}>
        <span class="tl-time">${escapeHtml(ts)}</span>
        <span class="tl-badge tl-chat">${escapeHtml(row.outcome || 'say')}</span>
        <span class="tl-body">${escapeHtml(trunc(row.body || '', 280))}</span>
      </div>`;
    case 'claim':
    case 'release':
    case 'done': {
      const detail = row.outcome ? ` (${escapeHtml(row.outcome)})` : '';
      return `<div class="tl-row" ${dataAttrs}>
        <span class="tl-time">${escapeHtml(ts)}</span>
        <span class="tl-badge tl-${kind}">${escapeHtml(kind)}</span>
        <span class="tl-body">${escapeHtml(row.item_id || '')}${detail}${row.intent ? ' — ' + escapeHtml(row.intent) : ''}</span>
      </div>`;
    }
    case 'progress':
      return `<div class="tl-row" ${dataAttrs}>
        <span class="tl-time">${escapeHtml(ts)}</span>
        <span class="tl-badge tl-progress">progress</span>
        <span class="tl-body">${escapeHtml(row.item_id ? row.item_id + ' — ' : '')}${escapeHtml(trunc(row.body || '', 280))}</span>
      </div>`;
    case 'blocked':
      return `<div class="tl-row" ${dataAttrs}>
        <span class="tl-time">${escapeHtml(ts)}</span>
        <span class="tl-badge tl-blocked">blocked</span>
        <span class="tl-body">${escapeHtml(row.item_id || '')}${row.outcome ? ' — ' + escapeHtml(row.outcome) : ''}</span>
      </div>`;
    case 'commit':
      return `<div class="tl-row" ${dataAttrs}>
        <span class="tl-time">${escapeHtml(ts)}</span>
        <span class="tl-badge tl-commit">commit</span>
        <span class="tl-body"><code>${escapeHtml((row.sha || '').slice(0, 7))}</code> ${escapeHtml(trunc(row.subject || '', 120))}</span>
      </div>`;
    case 'attestation':
      return `<div class="tl-row" ${dataAttrs}>
        <span class="tl-time">${escapeHtml(ts)}</span>
        <span class="tl-badge tl-attestation">${escapeHtml(row.attestation_kind || 'attest')}</span>
        <span class="tl-body">${escapeHtml(row.item_id || '')}${row.exit_code != null ? ' · exit ' + row.exit_code : ''}</span>
      </div>`;
    case 'event': {
      const ek = row.event_kind || '';
      const tool = row.tool || '';
      const exit = row.exit_code != null && row.exit_code !== 0 ? ` exit=${row.exit_code}` : '';
      const dur = ek === 'PostToolUse' && row.duration_ms > 0 ? ` <span class="tl-duration" title="post-tool duration">${escapeHtml(fmtDuration(row.duration_ms))}</span>` : '';
      const sessionTitle = row.session_id ? ` data-session="${escapeHtml(row.session_id)}" title="session: ${escapeHtml(row.session_id)}"` : '';
      return `<div class="tl-row" ${dataAttrs}${sessionTitle}>
        <span class="tl-time">${escapeHtml(ts)}</span>
        <span class="tl-badge tl-event">${escapeHtml(ek || 'event')}</span>
        <span class="tl-body"><strong>${escapeHtml(tool)}</strong> ${escapeHtml(trunc(row.target || '', TARGET_TRUNC))}${escapeHtml(exit)}${dur}</span>
      </div>`;
    }
    default:
      return '';
  }
}

function applyFilters(host, state) {
  const rows = host.querySelectorAll('.tl-row[data-classify]');
  let visible = 0;
  rows.forEach((r) => {
    const primary = r.dataset.primary;
    const secondary = r.dataset.secondary;
    const session = r.dataset.session || '';
    const sessionOK = !state.session || state.session === session;
    const ok = state[primary] !== false && (!secondary || state[secondary] !== false) && sessionOK;
    r.hidden = !ok;
    if (ok) visible++;
  });
  const empty = host.querySelector('[data-tl-empty]');
  if (empty) empty.hidden = visible !== 0 || rows.length === 0;
  const noActivity = host.querySelector('[data-tl-no-activity]');
  if (noActivity) noActivity.hidden = rows.length !== 0;
}

function renderTimeline(items, host, _agentId, _refetch) {
  const state = loadFilters();
  const sorted = (items || []).slice().sort((a, b) => (b.ts || 0) - (a.ts || 0));

  const sessions = collectSessions(sorted);
  if (state.session && !sessions.has(state.session)) state.session = '';

  const chipsHtml = CHIPS.map((c) =>
    `<button type="button" class="tl-chip${state[c.id] !== false ? ' on' : ''}" data-chip="${c.id}">${escapeHtml(c.label)}</button>`
  ).join('');

  const sessionPickerHtml = sessions.size > 1
    ? `<select class="tl-session" aria-label="Filter by session">
         <option value="">all sessions</option>
         ${[...sessions].map((s) => `<option value="${escapeHtml(s)}"${state.session === s ? ' selected' : ''}>${escapeHtml(s)}</option>`).join('')}
       </select>`
    : '';

  const rowsHtml = sorted.map((row) => {
    const [primary, secondary] = classify(row);
    if (!primary) return '';
    return renderRow(row, primary, secondary);
  }).join('');

  host.innerHTML = `
    <div class="tl-filters" role="toolbar" aria-label="Timeline filters">${chipsHtml}${sessionPickerHtml}</div>
    <div class="tl-list">
      ${rowsHtml || '<div class="agent-detail-empty" data-tl-no-activity>no activity yet — agent is registered but hasn\'t done anything we record.</div>'}
      <div class="agent-detail-empty" data-tl-empty hidden>everything is filtered out — toggle a chip back on.</div>
    </div>`;

  host.querySelectorAll('.tl-chip').forEach((btn) => {
    btn.addEventListener('click', () => {
      const id = btn.dataset.chip;
      state[id] = !(state[id] !== false);
      btn.classList.toggle('on', state[id] !== false);
      saveFilters(state);
      applyFilters(host, state);
    });
  });
  const sessionEl = host.querySelector('.tl-session');
  if (sessionEl) {
    sessionEl.addEventListener('change', () => {
      state.session = sessionEl.value || '';
      saveFilters(state);
      applyFilters(host, state);
    });
  }

  applyFilters(host, state);
}

function collectSessions(rows) {
  const set = new Set();
  for (const r of rows) {
    if (r.session_id) set.add(r.session_id);
  }
  return set;
}

setTimelineRenderer(renderTimeline);
