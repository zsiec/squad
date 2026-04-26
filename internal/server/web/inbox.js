// inbox.js — captured-state items modal with accept/reject actions.

import { fetchJSON, postJSON, escapeHtml, fmtAgo } from './util.js';
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
        <button type="button" class="action-btn ok" data-action="accept" data-id="${escapeHtml(it.id)}">Accept</button>
        <button type="button" class="action-btn danger" data-action="reject" data-id="${escapeHtml(it.id)}">Reject</button>
      </div>
    </div>`;
}

async function onClick(action, id) {
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
