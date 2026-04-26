// inbox.js — captured-state items modal with accept/reject actions.

import { fetchJSON, postJSON, escapeHtml, fmtAgo, fmtDate } from './util.js';
import { renderMarkdown } from './markdown.js';
import { toast } from './toasts.js';

const btnEl   = document.getElementById('inbox-btn');
const countEl = document.getElementById('inbox-count');

let onMutated = () => {};
let modalEl;

export function initInbox({ onChange } = {}) {
  if (typeof onChange === 'function') onMutated = onChange;
  btnEl?.addEventListener('click', open);
  refreshCount();
}

export async function refreshCount() {
  try {
    const inbox = await fetchJSON('/api/inbox');
    setCount(inbox.length);
  } catch { /* ignore */ }
}

function setCount(n) {
  if (!countEl) return;
  if (n > 0) {
    countEl.textContent = String(n);
    countEl.hidden = false;
  } else {
    countEl.hidden = true;
  }
}

function ensureModal() {
  if (modalEl) return modalEl;
  modalEl = document.createElement('div');
  modalEl.className = 'action-modal inbox-modal';
  modalEl.hidden = true;
  modalEl.innerHTML = `
    <div class="action-modal-backdrop" data-close></div>
    <aside class="action-modal-panel inbox-panel" role="dialog" aria-label="Inbox">
      <header class="action-modal-head">
        <span class="action-modal-title">INBOX</span>
        <button class="icon-btn" data-close aria-label="Close">✕</button>
      </header>
      <div class="action-modal-body inbox-body" id="inbox-list"></div>
    </aside>`;
  document.body.appendChild(modalEl);
  modalEl.addEventListener('click', (e) => {
    if (e.target.closest('[data-close]')) close();
  });
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && modalEl && !modalEl.hidden) close();
  });
  return modalEl;
}

export async function open() {
  const el = ensureModal();
  el.hidden = false;
  requestAnimationFrame(() => el.classList.add('show'));
  await renderList();
}

function close() {
  if (!modalEl) return;
  modalEl.classList.remove('show');
  setTimeout(() => { modalEl.hidden = true; }, 180);
}

async function renderList() {
  const host = modalEl.querySelector('#inbox-list');
  host.innerHTML = '<div class="nav-loading">loading…</div>';
  try {
    const inbox = await fetchJSON('/api/inbox');
    setCount(inbox.length);
    if (!inbox.length) {
      host.innerHTML = '<div class="nav-empty">inbox empty</div>';
      return;
    }
    host.innerHTML = inbox.map(row).join('');
    host.querySelectorAll('button[data-action]').forEach((b) => {
      b.addEventListener('click', () => onClick(b.dataset.action, b.dataset.id));
    });
  } catch (err) {
    host.innerHTML = `<div class="nav-error">${escapeHtml(err.message)}</div>`;
  }
}

function row(it) {
  const ago = it.captured_at ? fmtAgo(it.captured_at) + ' ago' : '';
  const dorPass = it.dor_pass ? '<span class="inbox-dor ok">DoR ✓</span>' : '<span class="inbox-dor bad">DoR ✗</span>';
  return `
    <div class="inbox-row" data-id="${escapeHtml(it.id)}">
      <div class="inbox-meta">
        <span class="inbox-id">${escapeHtml(it.id)}</span>
        <span class="inbox-by">${escapeHtml(it.captured_by || '?')}</span>
        <span class="inbox-when">${escapeHtml(ago)}</span>
        ${dorPass}
      </div>
      <div class="inbox-title">${escapeHtml(it.title)}</div>
      <div class="inbox-actions">
        <button type="button" class="action-btn" data-action="details" data-id="${escapeHtml(it.id)}" aria-expanded="false">Details</button>
        <button type="button" class="action-btn ok" data-action="accept" data-id="${escapeHtml(it.id)}">Accept</button>
        <button type="button" class="action-btn danger" data-action="reject" data-id="${escapeHtml(it.id)}">Reject</button>
      </div>
      <div class="inbox-details" data-details-for="${escapeHtml(it.id)}" hidden></div>
    </div>`;
}

async function onClick(action, id) {
  if (action === 'details') {
    await toggleDetails(id);
    return;
  }
  if (action === 'accept') {
    try {
      await postJSON(`/api/items/${encodeURIComponent(id)}/accept`, {});
      toast({ kind: 'ok', title: `Accepted ${id}` });
      onMutated();
      await renderList();
    } catch (err) {
      toast({ kind: 'error', title: 'Accept failed', body: err.message });
    }
    return;
  }
  if (action === 'reject') {
    const reason = window.prompt('Reject reason (required):', '');
    if (!reason) return;
    try {
      await postJSON(`/api/items/${encodeURIComponent(id)}/reject`, { reason });
      toast({ kind: 'warn', title: `Rejected ${id}` });
      onMutated();
      await renderList();
    } catch (err) {
      toast({ kind: 'error', title: 'Reject failed', body: err.message });
    }
  }
}

async function toggleDetails(id) {
  if (!modalEl) return;
  const btn = modalEl.querySelector(`button[data-action="details"][data-id="${cssEscape(id)}"]`);
  const host = modalEl.querySelector(`[data-details-for="${cssEscape(id)}"]`);
  if (!btn || !host) return;
  if (!host.hidden) {
    host.hidden = true;
    btn.setAttribute('aria-expanded', 'false');
    btn.textContent = 'Details';
    return;
  }
  if (!host.dataset.loaded) {
    host.innerHTML = '<div class="nav-loading">loading…</div>';
    host.hidden = false;
    btn.setAttribute('aria-expanded', 'true');
    btn.textContent = 'Hide';
    try {
      const it = await fetchJSON('/api/items/' + encodeURIComponent(id));
      host.innerHTML = renderDetails(it);
      host.dataset.loaded = '1';
    } catch (err) {
      host.innerHTML = `<div class="nav-error">${escapeHtml(err.message)}</div>`;
    }
    return;
  }
  host.hidden = false;
  btn.setAttribute('aria-expanded', 'true');
  btn.textContent = 'Hide';
}

function cssEscape(s) {
  if (window.CSS?.escape) return window.CSS.escape(s);
  return String(s).replace(/[^a-zA-Z0-9_-]/g, (c) => '\\' + c);
}

function renderDetails(it) {
  const meta = [
    ['Type',     it.type],
    ['Priority', it.priority],
    ['Area',     it.area],
    ['Estimate', it.estimate],
    ['Risk',     it.risk],
    ['Created',  fmtDate(it.created)],
    ['Updated',  fmtDate(it.updated)],
  ].filter(([, v]) => v);

  const ac = it.ac || [];
  const acDone = ac.filter((a) => a.checked).length;
  const acHtml = ac.length ? `
    <div class="inbox-detail-section">
      <div class="inbox-detail-head">Acceptance <span class="count">${acDone}/${ac.length}</span></div>
      <ul class="ac-list">
        ${ac.map((a) => `
          <li class="ac-item${a.checked ? ' checked' : ''}">
            <span class="ac-glyph">${a.checked ? '▣' : '□'}</span>
            <span class="ac-text">${escapeHtml(a.text).replace(/\n/g, '<br>')}</span>
          </li>`).join('')}
      </ul>
    </div>` : '';

  const bodyHtml = it.body_markdown ? `
    <div class="inbox-detail-section">
      <div class="inbox-detail-head">Body</div>
      <div class="md">${renderMarkdown(it.body_markdown)}</div>
    </div>` : '';

  return `
    <div class="inbox-detail-meta">
      ${meta.map(([k, v]) => `<span class="inbox-detail-meta-cell"><span class="inbox-detail-meta-k">${escapeHtml(k)}</span> ${escapeHtml(v)}</span>`).join('')}
    </div>
    ${acHtml}
    ${bodyHtml}`;
}
