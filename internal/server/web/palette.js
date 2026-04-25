// palette.js — Cmd/Ctrl-K command palette with fuzzy search over items + agents

import { escapeHtml, priClass, fetchJSON, fmtAgo } from './util.js';
import { displayName } from './names.js';

let paletteEl;
let inputEl;
let resultsEl;
let isOpen = false;
let selected = 0;
let results = [];
let provider = () => ({ items: [], agents: [], actions: [] });
let onAction = () => {};

let searchSeq = 0;
let lastQuery = '';
let serverHits = [];
let searchTimer = null;

export function initPalette({ getData, onSelect }) {
  provider = getData;
  onAction = onSelect;
  mount();
  wireKeys();
}

function mount() {
  paletteEl = document.createElement('div');
  paletteEl.className = 'palette';
  paletteEl.hidden = true;
  paletteEl.innerHTML = `
    <div class="palette-backdrop" data-close></div>
    <div class="palette-panel" role="dialog" aria-label="Command palette">
      <div class="palette-input-wrap">
        <span class="palette-icon">⌘K</span>
        <input id="palette-input" class="palette-input" type="text" placeholder="Search items, agents, actions…" spellcheck="false" autocomplete="off"/>
        <span class="palette-hint">esc</span>
      </div>
      <ul id="palette-results" class="palette-results" role="listbox"></ul>
      <div class="palette-footer">
        <span><kbd>↑</kbd><kbd>↓</kbd> navigate</span>
        <span><kbd>↵</kbd> open</span>
        <span><kbd>esc</kbd> close</span>
      </div>
    </div>
  `;
  document.body.appendChild(paletteEl);

  inputEl = paletteEl.querySelector('#palette-input');
  resultsEl = paletteEl.querySelector('#palette-results');

  paletteEl.querySelector('[data-close]').addEventListener('click', close);
  inputEl.addEventListener('input', () => {
    selected = 0;
    scheduleServerSearch();
    render();
  });
  inputEl.addEventListener('keydown', onInputKey);
  resultsEl.addEventListener('click', (e) => {
    const li = e.target.closest('li[data-idx]');
    if (!li) return;
    selected = Number(li.dataset.idx);
    pickCurrent();
  });
}

function wireKeys() {
  document.addEventListener('keydown', (e) => {
    const cmdK = (e.key === 'k' || e.key === 'K') && (e.metaKey || e.ctrlKey);
    if (cmdK) { e.preventDefault(); toggle(); return; }
    if (e.key === 'Escape' && isOpen) { e.preventDefault(); close(); }
  });
}

export function open() {
  isOpen = true;
  paletteEl.hidden = false;
  inputEl.value = '';
  selected = 0;
  serverHits = [];
  lastQuery = '';
  render();
  requestAnimationFrame(() => inputEl.focus());
}

function scheduleServerSearch() {
  const q = inputEl.value.trim();
  if (q.length < 3) { serverHits = []; lastQuery = q; return; }
  if (q === lastQuery) return;
  lastQuery = q;
  const mySeq = ++searchSeq;
  clearTimeout(searchTimer);
  searchTimer = setTimeout(async () => {
    try {
      const hits = await fetchJSON('/api/search?q=' + encodeURIComponent(q) + '&limit=40');
      if (mySeq !== searchSeq) return;
      serverHits = hits || [];
      render();
    } catch { /* ignore */ }
  }, 120);
}

export function close() {
  isOpen = false;
  paletteEl.hidden = true;
}

function toggle() { isOpen ? close() : open(); }

function onInputKey(e) {
  if (e.key === 'ArrowDown') { e.preventDefault(); selected = Math.min(results.length - 1, selected + 1); updateSelection(); }
  else if (e.key === 'ArrowUp') { e.preventDefault(); selected = Math.max(0, selected - 1); updateSelection(); }
  else if (e.key === 'Enter') { e.preventDefault(); pickCurrent(); }
}

function pickCurrent() {
  const r = results[selected];
  if (!r) return;
  close();
  onAction(r);
}

function render() {
  const q = inputEl.value.trim().toLowerCase();
  const data = provider();
  const scored = [];

  // items
  for (const it of data.items) {
    const hay = [it.id, it.title, it.area, it.type, it.priority].join(' ').toLowerCase();
    const s = score(q, hay, it.id.toLowerCase());
    if (s > 0 || q === '') {
      scored.push({ kind: 'item', score: s, item: it });
    }
  }
  // agents
  for (const a of data.agents) {
    const id = a.agent_id || a.AgentID || '';
    const name = displayName(id, a.display_name || a.DisplayName);
    const claim = a.claim_item || a.ClaimItem || '';
    const hay = [id, name, claim].join(' ').toLowerCase();
    const s = score(q, hay, name.toLowerCase());
    if (s > 0 || q === '') {
      scored.push({ kind: 'agent', score: s, agent: a });
    }
  }
  // built-in actions
  for (const a of data.actions) {
    const hay = a.label.toLowerCase();
    const s = score(q, hay, hay);
    if (s > 0 || q === '') {
      scored.push({ kind: 'action', score: s, action: a });
    }
  }

  // server full-text hits (body/message content) — merge as a secondary class
  // avoid duplicating items already in the local list
  const seen = new Set(scored.filter(x => x.kind === 'item').map((x) => x.item.id));
  for (const h of serverHits) {
    if (h.kind === 'item' && !seen.has(h.id)) {
      scored.push({ kind: 'body-hit', score: h.score, hit: h });
    } else if (h.kind === 'message') {
      scored.push({ kind: 'msg-hit', score: h.score, hit: h });
    }
  }

  scored.sort((a, b) => b.score - a.score);
  results = scored.slice(0, 40);
  if (selected >= results.length) selected = Math.max(0, results.length - 1);
  paint();
}

function paint() {
  if (!results.length) {
    resultsEl.innerHTML = `<li class="palette-empty">no matches</li>`;
    return;
  }
  resultsEl.innerHTML = results.map((r, i) => row(r, i)).join('');
}

function row(r, i) {
  const sel = i === selected ? ' selected' : '';
  if (r.kind === 'item') {
    const it = r.item;
    const pri = priClass(it.priority);
    return `
      <li class="palette-row item${sel}" data-idx="${i}" role="option">
        <span class="palette-pri pri-pip ${pri}">${escapeHtml((it.priority || '').replace(/^P/, ''))}</span>
        <span class="palette-mono">${escapeHtml(it.id)}</span>
        <span class="palette-title">${escapeHtml(it.title || '')}</span>
        <span class="palette-meta">${escapeHtml(it.area || '')}</span>
      </li>`;
  }
  if (r.kind === 'agent') {
    const id = r.agent.agent_id || r.agent.AgentID || '';
    const name = displayName(id, r.agent.display_name || r.agent.DisplayName);
    const claim = r.agent.claim_item || r.agent.ClaimItem || '';
    const status = r.agent.status || r.agent.Status || 'offline';
    return `
      <li class="palette-row agent${sel}" data-idx="${i}" role="option">
        <span class="palette-kind">AGENT</span>
        <span class="palette-title">${escapeHtml(name)}</span>
        <span class="palette-meta">${status}${claim ? ' · ' + escapeHtml(claim) : ''}</span>
      </li>`;
  }
  if (r.kind === 'body-hit') {
    const h = r.hit;
    return `
      <li class="palette-row body-hit${sel}" data-idx="${i}" role="option">
        <span class="palette-kind">BODY</span>
        <span class="palette-mono">${escapeHtml(h.id)}</span>
        <span class="palette-title">${escapeHtml(h.title)}</span>
        <span class="palette-snip">${escapeHtml(h.snippet)}</span>
      </li>`;
  }
  if (r.kind === 'msg-hit') {
    const h = r.hit;
    return `
      <li class="palette-row msg-hit${sel}" data-idx="${i}" role="option">
        <span class="palette-kind">MSG</span>
        <span class="palette-title">${escapeHtml(h.title)}</span>
        <span class="palette-snip">${escapeHtml(h.snippet)}</span>
      </li>`;
  }
  const a = r.action;
  return `
    <li class="palette-row action${sel}" data-idx="${i}" role="option">
      <span class="palette-kind">CMD</span>
      <span class="palette-title">${escapeHtml(a.label)}</span>
      <span class="palette-meta">${escapeHtml(a.hint || '')}</span>
    </li>`;
}

function updateSelection() {
  resultsEl.querySelectorAll('li[data-idx]').forEach((li) => {
    const isSel = Number(li.dataset.idx) === selected;
    li.classList.toggle('selected', isSel);
    if (isSel) li.scrollIntoView({ block: 'nearest' });
  });
}

// --- fuzzy score: prefix on primary > subsequence score on haystack -----
function score(q, hay, primary) {
  if (!q) return 0.5;
  if (primary.startsWith(q)) return 100 + q.length;
  if (hay.startsWith(q))     return 50 + q.length;
  if (hay.includes(q))       return 20 + q.length;
  // subsequence match
  let i = 0, matched = 0;
  for (const ch of hay) {
    if (ch === q[i]) { matched++; i++; if (i === q.length) break; }
  }
  if (i < q.length) return 0;
  return 5 + matched;
}
