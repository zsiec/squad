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

// refreshIfOpen re-renders the inbox modal's row list when it's currently
// open. No-op when closed so backgrounded sessions don't fetch on every
// inbox_changed event from peers.
export async function refreshIfOpen() {
  if (!modalEl || modalEl.hidden) return;
  await renderList();
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
  const autoBadge = it.auto_refined_at ? '<span class="inbox-auto-refined" title="body drafted by claude — review before accepting">auto-refined</span>' : '';
  return `
    <div class="inbox-row" data-id="${escapeHtml(it.id)}">
      <div class="inbox-meta">
        <span class="inbox-id">${escapeHtml(it.id)}</span>
        <span class="inbox-by">${escapeHtml(it.captured_by || '?')}</span>
        <span class="inbox-when">${escapeHtml(ago)}</span>
        ${dorPass}
        ${autoBadge}
      </div>
      <div class="inbox-title">${escapeHtml(it.title)}</div>
      <div class="inbox-actions">
        <button type="button" class="action-btn" data-action="details" data-id="${escapeHtml(it.id)}" aria-expanded="false">Details</button>
        <button type="button" class="action-btn warn" data-action="auto-refine" data-id="${escapeHtml(it.id)}">Auto-refine</button>
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
  if (action === 'auto-refine') {
    await runAutoRefine(id);
    return;
  }
  if (action === 'refine') {
    await openRefineComposer(id);
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
      host.querySelectorAll('button[data-action]').forEach((b) => {
        b.addEventListener('click', () => onClick(b.dataset.action, b.dataset.id));
      });
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
    ${bodyHtml}
    <div class="inbox-detail-actions">
      <button type="button" class="action-btn warn" data-action="refine" data-id="${escapeHtml(it.id)}">Send for refinement</button>
    </div>`;
}

async function runAutoRefine(id) {
  if (!modalEl) return;
  const btn = modalEl.querySelector(`button[data-action="auto-refine"][data-id="${cssEscape(id)}"]`);
  if (!btn || btn.disabled) return;
  const originalLabel = btn.textContent;
  btn.disabled = true;
  btn.classList.add('drafting');
  btn.textContent = 'drafting…';
  try {
    const res = await fetch(`/api/items/${encodeURIComponent(id)}/auto-refine`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: '{}',
    });
    const text = await res.text();
    let payload = null;
    try { payload = text ? JSON.parse(text) : null; } catch { payload = null; }
    if (res.ok && payload) {
      replaceRow(id, payload);
      onMutated();
      return;
    }
    autoRefineToastForStatus(res.status, payload);
  } catch (err) {
    toast({ kind: 'error', title: 'Auto-refine failed', body: err.message || String(err) });
  } finally {
    btn.disabled = false;
    btn.classList.remove('drafting');
    btn.textContent = originalLabel;
  }
}

function autoRefineToastForStatus(status, payload) {
  const errMsg = (payload && payload.error) || 'unknown error';
  const stdout = (payload && payload.stdout) || '';
  const stderr = (payload && payload.stderr) || '';
  // claude -p writes most diagnostics — auth failures, MCP init errors,
  // tool denials — to stdout; stderr is rare. Prefer stdout so the
  // operator sees the actual failure text instead of "exit status 1".
  const diag = (stdout || stderr).slice(0, 240);
  switch (status) {
    case 503:
      toast({ kind: 'error', title: 'Claude CLI not found', body: errMsg });
      return;
    case 504:
      toast({ kind: 'error', title: 'Auto-refine timed out', body: errMsg });
      return;
    case 502:
      toast({ kind: 'error', title: 'Claude failed', body: diag || errMsg });
      return;
    case 500:
      toast({ kind: 'error', title: 'Claude exited without drafting', body: diag || errMsg });
      return;
    case 409:
      if (errMsg.includes('already in flight')) {
        toast({ kind: 'warn', title: 'Already drafting', body: errMsg });
      } else {
        toast({ kind: 'warn', title: 'Status is no longer captured', body: errMsg });
      }
      return;
    case 404:
      toast({ kind: 'error', title: 'Item not found', body: errMsg });
      return;
    default:
      toast({ kind: 'error', title: `Auto-refine failed (${status})`, body: errMsg });
  }
}

function replaceRow(id, payload) {
  if (!modalEl) return;
  const rowEl = modalEl.querySelector(`.inbox-row[data-id="${cssEscape(id)}"]`);
  if (!rowEl) return;
  const tmp = document.createElement('div');
  tmp.innerHTML = row(payload);
  const replacement = tmp.firstElementChild;
  if (!replacement) return;
  rowEl.replaceWith(replacement);
  replacement.querySelectorAll('button[data-action]').forEach((b) => {
    b.addEventListener('click', () => onClick(b.dataset.action, b.dataset.id));
  });
}

// Persists across server failures so a 502 doesn't lose the operator's
// work; cleared on successful 200.
const commentsByItem = new Map();

// "Send for refinement" drives the claude redraft flow: operator
// comments → claude rewrites the body. For a peer-reviewer pass on the
// markdown itself, use `squad refine <ID>` from the CLI — that path
// goes to the needs-refinement queue and is intentionally CLI-only so
// the SPA surface stays single-purpose.
async function openRefineComposer(id) {
  if (!modalEl) return;
  const host = modalEl.querySelector(`[data-details-for="${cssEscape(id)}"]`);
  if (!host) return;
  if (host.hidden) await toggleDetails(id);

  const existing = host.querySelector('.refine-composer');
  if (existing) {
    existing.scrollIntoView({ block: 'nearest' });
    return;
  }

  const bodyPanel = host.querySelector('.md');
  if (!bodyPanel) {
    toast({ kind: 'warn', title: 'Item has no body to comment on' });
    return;
  }

  if (!commentsByItem.has(id)) commentsByItem.set(id, []);

  const composer = document.createElement('div');
  composer.className = 'refine-composer';
  composer.dataset.itemId = id;
  composer.innerHTML = `
    <div class="refine-composer-help">
      Select text in the body above, then click <strong>Add comment</strong>.
      Send when you're done — claude will use these notes when redrafting.
    </div>
    <ul class="refine-comment-list" data-comment-list></ul>
    <div class="refine-composer-actions">
      <span class="refine-comment-count" data-comment-count>0 comments</span>
      <button type="button" class="action-btn" data-refine-add disabled>Add comment</button>
      <button type="button" class="action-btn ghost" data-refine-cancel>Cancel</button>
      <button type="button" class="action-btn warn" data-refine-send>Send</button>
    </div>`;
  host.appendChild(composer);

  const addBtn = composer.querySelector('[data-refine-add]');
  const sendBtn = composer.querySelector('[data-refine-send]');
  const cancelBtn = composer.querySelector('[data-refine-cancel]');

  const refreshAddEnabled = () => {
    if (!composer.isConnected) {
      document.removeEventListener('selectionchange', refreshAddEnabled);
      return;
    }
    addBtn.disabled = !selectionInside(bodyPanel);
  };
  document.addEventListener('selectionchange', refreshAddEnabled);

  rerenderCommentList(composer, bodyPanel, id);

  addBtn.addEventListener('click', () => {
    const sel = window.getSelection();
    const span = sel ? sel.toString().trim() : '';
    if (!span || !sel.rangeCount) return;
    const range = sel.getRangeAt(0).cloneRange();
    openCommentEditor(composer, bodyPanel, id, { quoted_span: span, comment: '', range });
    sel.removeAllRanges();
    refreshAddEnabled();
  });

  cancelBtn.addEventListener('click', () => {
    commentsByItem.delete(id);
    clearHighlights(bodyPanel);
    composer.remove();
  });

  sendBtn.addEventListener('click', async () => {
    await sendRefine(id, composer, bodyPanel, sendBtn);
  });
}

function selectionInside(host) {
  const sel = window.getSelection();
  if (!sel || sel.isCollapsed || !sel.rangeCount) return false;
  if (!sel.toString().trim()) return false;
  const range = sel.getRangeAt(0);
  return host.contains(range.startContainer) && host.contains(range.endContainer);
}

function clearHighlights(bodyPanel) {
  bodyPanel.querySelectorAll('.refine-highlight').forEach((el) => unwrap(el));
}

function unwrap(el) {
  const parent = el.parentNode;
  if (!parent) return;
  while (el.firstChild) parent.insertBefore(el.firstChild, el);
  parent.removeChild(el);
  parent.normalize();
}

// wrapRange wraps the live Range in a `.refine-highlight` span. Returns
// the span on success or null when the selection straddles element
// boundaries (surroundContents throws for those) — the caller keeps the
// comment in state without a visible mark in that case.
function wrapRange(range, idx) {
  const wrapper = document.createElement('span');
  wrapper.className = 'refine-highlight';
  wrapper.dataset.commentIdx = String(idx);
  try {
    range.surroundContents(wrapper);
    return wrapper;
  } catch {
    return null;
  }
}

function renumberHighlights(bodyPanel) {
  bodyPanel.querySelectorAll('.refine-highlight').forEach((el, i) => {
    el.dataset.commentIdx = String(i);
  });
}

function rerenderCommentList(composer, bodyPanel, id) {
  const list = composer.querySelector('[data-comment-list]');
  const count = composer.querySelector('[data-comment-count]');
  const comments = commentsByItem.get(id) || [];
  list.innerHTML = comments.map((c, idx) => `
    <li class="refine-comment-item" data-idx="${idx}">
      <div class="refine-comment-quote">${escapeHtml(c.quoted_span)}</div>
      <div class="refine-comment-text">${c.comment ? escapeHtml(c.comment) : '<em>(no note yet)</em>'}</div>
      <div class="refine-comment-row-actions">
        <button type="button" class="action-btn small" data-comment-edit="${idx}">Edit</button>
        <button type="button" class="action-btn small danger" data-comment-delete="${idx}">Delete</button>
      </div>
    </li>`).join('');
  count.textContent = comments.length === 1 ? '1 comment' : `${comments.length} comments`;

  list.querySelectorAll('[data-comment-edit]').forEach((b) => {
    b.addEventListener('click', () => {
      const idx = Number(b.dataset.commentEdit);
      openCommentEditor(composer, bodyPanel, id, { ...comments[idx], _idx: idx });
    });
  });
  list.querySelectorAll('[data-comment-delete]').forEach((b) => {
    b.addEventListener('click', () => {
      const idx = Number(b.dataset.commentDelete);
      const target = bodyPanel.querySelector(`.refine-highlight[data-comment-idx="${idx}"]`);
      if (target) unwrap(target);
      comments.splice(idx, 1);
      renumberHighlights(bodyPanel);
      rerenderCommentList(composer, bodyPanel, id);
    });
  });
  bodyPanel.querySelectorAll('.refine-highlight').forEach((el) => {
    el.addEventListener('click', () => {
      const idx = Number(el.dataset.commentIdx);
      const item = list.querySelector(`.refine-comment-item[data-idx="${idx}"]`);
      item?.scrollIntoView({ block: 'nearest' });
      list.querySelectorAll('.refine-comment-item.focused').forEach((x) => x.classList.remove('focused'));
      item?.classList.add('focused');
    });
  });
}

function openCommentEditor(composer, bodyPanel, id, draft) {
  composer.querySelectorAll('.refine-comment-editor').forEach((e) => e.remove());
  const editor = document.createElement('div');
  editor.className = 'refine-comment-editor';
  editor.innerHTML = `
    <div class="refine-comment-quote">${escapeHtml(draft.quoted_span)}</div>
    <textarea rows="2" placeholder="What should change about this passage?"></textarea>
    <div class="refine-comment-editor-actions">
      <button type="button" class="action-btn ghost small" data-editor-cancel>Cancel</button>
      <button type="button" class="action-btn small" data-editor-save>Save</button>
    </div>`;
  composer.querySelector('[data-comment-list]').insertAdjacentElement('afterend', editor);
  const ta = editor.querySelector('textarea');
  ta.value = draft.comment || '';
  setTimeout(() => ta.focus(), 30);

  editor.querySelector('[data-editor-cancel]').addEventListener('click', () => {
    editor.remove();
  });
  editor.querySelector('[data-editor-save]').addEventListener('click', () => {
    const note = ta.value.trim();
    if (!note) {
      ta.focus();
      return;
    }
    const list = commentsByItem.get(id);
    if (typeof draft._idx === 'number') {
      list[draft._idx] = { quoted_span: draft.quoted_span, comment: note };
    } else {
      const idx = list.length;
      list.push({ quoted_span: draft.quoted_span, comment: note });
      if (draft.range) wrapRange(draft.range, idx);
    }
    editor.remove();
    rerenderCommentList(composer, bodyPanel, id);
  });
}

async function sendRefine(id, composer, bodyPanel, sendBtn) {
  const comments = commentsByItem.get(id) || [];
  const payload = comments.length > 0
    ? { comments: comments.map((c) => ({ quoted_span: c.quoted_span, comment: c.comment })) }
    : {};
  sendBtn.disabled = true;
  try {
    const res = await fetch(`/api/items/${encodeURIComponent(id)}/auto-refine`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    const text = await res.text();
    let body = null;
    try { body = text ? JSON.parse(text) : null; } catch { body = null; }
    if (res.ok && body) {
      commentsByItem.delete(id);
      clearHighlights(bodyPanel);
      composer.remove();
      replaceRow(id, body);
      toast({ kind: 'ok', title: `Refined ${id}` });
      onMutated();
      return;
    }
    autoRefineToastForStatus(res.status, body);
  } catch (err) {
    toast({ kind: 'error', title: 'Refine failed', body: err.message || String(err) });
  } finally {
    sendBtn.disabled = false;
  }
}
