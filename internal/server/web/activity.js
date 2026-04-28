// activity.js — shared activity-feed widget (item drawer)
//
// Groups rows by day, renders monospace columns: ts · agent · kind · body.

import { escapeHtml, fmtHM, fmtDay } from './util.js';
import { parseHandoff, renderHandoffHTML, summarize } from './handoff.js';
import { displayName } from './names.js';

export class ActivityFeed {
  constructor(hostEl) {
    this.host = hostEl;
    this.rows = [];          // newest first in the host DOM (prepended)
    this.oldestTs = null;
    this.renderedDays = new Set();
  }

  clear() {
    this.host.innerHTML = '';
    this.rows = [];
    this.oldestTs = null;
    this.renderedDays.clear();
  }

  append(events) {
    // append at the bottom (used for initial load, which comes newest-first — we reverse so oldest lands at top)
    const ordered = [...events].sort((a, b) => a.ts - b.ts);
    for (const e of ordered) {
      this._insertRow(e, { prepend: false });
    }
  }

  prepend(e) {
    // real-time event: newest, insert at bottom (host displays oldest → newest top-to-bottom)
    this._insertRow(e, { prepend: false });
  }

  loadOlder(events) {
    // older batch fetched via /activity?before=... — prepend to top
    const ordered = [...events].sort((a, b) => b.ts - a.ts);
    for (const e of ordered) {
      this._insertRow(e, { prepend: true });
    }
  }

  _insertRow(e, { prepend }) {
    const dayKey = fmtDay(e.ts);
    let dayEl = this.host.querySelector(`[data-day="${dayKey}"]`);
    const row = activityRow(e);

    if (!dayEl) {
      dayEl = document.createElement('div');
      dayEl.className = 'activity-day';
      dayEl.dataset.day = dayKey;
      dayEl.textContent = dayKey;
      if (prepend) {
        this.host.insertBefore(dayEl, this.host.firstChild);
      } else {
        this.host.appendChild(dayEl);
      }
    }

    if (prepend) {
      dayEl.after(row); // row goes right after the day header at the top
    } else {
      this.host.appendChild(row);
    }

    if (this.oldestTs === null || e.ts < this.oldestTs) this.oldestTs = e.ts;
  }
}

function activityRow(e) {
  const div = document.createElement('div');
  div.className = 'activity-row';
  const kindClass = e.kind === 'say' ? 'say chat' : e.kind;

  // handoff: full card, spanning the body column
  if (e.kind === 'handoff') {
    const h = parseHandoff(e.body);
    if (h) {
      div.classList.add('activity-row-handoff');
      div.innerHTML =
        `<span class="a-ts">${fmtHM(e.ts)}</span>` +
        `<span class="a-agent" title="${escapeHtml(e.agent_id || '')}">${escapeHtml(displayName(e.agent_id, e.display_name))}</span>` +
        `<span class="a-kind ${escapeHtml(kindClass)}">${escapeHtml(e.kind)}</span>` +
        `<span class="a-body">${renderHandoffHTML(h)}</span>`;
      return div;
    }
  }

  const detail = pickDetail(e);
  div.innerHTML =
    `<span class="a-ts">${fmtHM(e.ts)}</span>` +
    `<span class="a-agent" title="${escapeHtml(e.agent_id || '')}">${escapeHtml(displayName(e.agent_id, e.display_name))}</span>` +
    `<span class="a-kind ${escapeHtml(kindClass)}">${escapeHtml(e.kind)}</span>` +
    `<span class="a-body">${escapeHtml(detail)}</span>`;
  return div;
}

function pickDetail(e) {
  switch (e.kind) {
    case 'claim':    return e.detail ? '"' + e.detail + '"' : '';
    case 'release':  return e.outcome || '';
    case 'progress': return (e.pct != null ? e.pct + '% ' : '') + (e.detail || '');
    case 'touch':
    case 'untouch':  return e.path || '';
    case 'say':
    case 'chat':
    case 'thinking':
    case 'stuck':
    case 'milestone':
    case 'fyi':
    case 'ask':
    case 'review_req':
                     return e.body || '';
    default:         return e.body || e.detail || '';
  }
}
