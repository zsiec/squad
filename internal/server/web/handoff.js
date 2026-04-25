// handoff.js — render a handoff message body as structured blocks.
//
// Handoff bodies are JSON: { shipped, in_flight, surprised_by, unblocks, note }.
// Text strings inside any field may contain ITEM-IDs we want autolinked.

import { escapeHtml } from './util.js';
import { autolinkText } from './autolink.js';

/**
 * Parse the stored body. Returns null if it doesn't look like a handoff.
 */
export function parseHandoff(body) {
  if (!body) return null;
  const trimmed = String(body).trim();
  if (!trimmed.startsWith('{')) return null;
  try {
    const obj = JSON.parse(trimmed);
    if (!obj || typeof obj !== 'object') return null;
    // must have at least one known key
    const keys = ['shipped', 'in_flight', 'surprised_by', 'unblocks', 'note'];
    if (!keys.some((k) => k in obj)) return null;
    return obj;
  } catch { return null; }
}

/**
 * Return an HTML string for a structured handoff card.
 * Uses autolinkText for item-id highlighting.
 */
export function renderHandoffHTML(h) {
  const sections = [];
  if (h.shipped?.length)      sections.push(block('shipped',      'ok',     h.shipped));
  if (h.in_flight?.length)    sections.push(block('in flight',    'accent', h.in_flight));
  if (h.unblocks?.length)     sections.push(block('unblocks',     'info',   h.unblocks));
  if (h.surprised_by?.length) sections.push(block('surprised by', 'violet', h.surprised_by));
  if (h.note)                 sections.push(noteBlock(h.note));
  if (!sections.length) return '';
  return `<div class="handoff-card">${sections.join('')}</div>`;
}

function block(label, tone, items) {
  return `
    <div class="handoff-block handoff-${tone}">
      <div class="handoff-label">${escapeHtml(label)}</div>
      <ul class="handoff-items">
        ${items.map((s) => `<li>${autolinkText(s)}</li>`).join('')}
      </ul>
    </div>`;
}

function noteBlock(note) {
  return `
    <div class="handoff-block handoff-note">
      <div class="handoff-label">note</div>
      <div class="handoff-note-body">${autolinkText(note)}</div>
    </div>`;
}

/**
 * Short one-line summary, used for activity feed rows where a full card is too much.
 */
export function summarize(h) {
  const parts = [];
  if (h.shipped?.length)      parts.push(`shipped ${h.shipped.length}`);
  if (h.in_flight?.length)    parts.push(`${h.in_flight.length} in flight`);
  if (h.unblocks?.length)     parts.push(`unblocks ${h.unblocks.join(', ')}`);
  if (h.surprised_by?.length) parts.push(`${h.surprised_by.length} surprises`);
  return parts.join(' · ') || (h.note ? 'note' : 'handoff');
}

/**
 * Compact chip row for narrow contexts (chat). Shows the same info as summarize()
 * but as styled pills, and is clickable via its container to open the full modal.
 */
export function renderHandoffCompactHTML(h) {
  const pills = [];
  if (h.shipped?.length)      pills.push(pill('shipped',   String(h.shipped.length),    'ok'));
  if (h.in_flight?.length)    pills.push(pill('in flight', String(h.in_flight.length),  'accent'));
  if (h.unblocks?.length)     pills.push(pill('unblocks',  h.unblocks.join(', '),       'info'));
  if (h.surprised_by?.length) pills.push(pill('surprises', String(h.surprised_by.length),'violet'));
  if (h.note)                 pills.push(pill('note',      '',                          'note'));
  return `<span class="handoff-chip" role="button" tabindex="0">
    ${pills.join('')}
    <span class="handoff-chip-open" aria-hidden="true">↗</span>
  </span>`;
}
function pill(label, value, tone) {
  const text = value ? `${label} ${value}` : label;
  return `<span class="handoff-pill hp-${tone}">${text}</span>`;
}

// Mount one reusable modal; open with the handoff body.
let modalEl;
function ensureModal() {
  if (modalEl) return modalEl;
  modalEl = document.createElement('div');
  modalEl.className = 'handoff-modal';
  modalEl.hidden = true;
  modalEl.innerHTML = `
    <div class="handoff-modal-backdrop" data-close></div>
    <aside class="handoff-modal-panel" role="dialog" aria-label="Handoff">
      <header class="handoff-modal-head">
        <span class="handoff-modal-title">HANDOFF</span>
        <span class="handoff-modal-meta" id="handoff-modal-meta"></span>
        <button class="icon-btn" data-close aria-label="Close">✕</button>
      </header>
      <div class="handoff-modal-body" id="handoff-modal-body"></div>
    </aside>`;
  document.body.appendChild(modalEl);
  modalEl.addEventListener('click', (e) => {
    if (e.target.closest('[data-close]')) closeModal();
  });
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && modalEl && !modalEl.hidden) closeModal();
  });
  return modalEl;
}

export function openHandoffModal(h, meta = {}) {
  const el = ensureModal();
  el.querySelector('#handoff-modal-meta').textContent =
    [meta.agent, meta.ts].filter(Boolean).join(' · ');
  el.querySelector('#handoff-modal-body').innerHTML = renderHandoffHTML(h);
  el.hidden = false;
  requestAnimationFrame(() => el.classList.add('show'));
}

function closeModal() {
  if (!modalEl) return;
  modalEl.classList.remove('show');
  setTimeout(() => modalEl.hidden = true, 180);
}
