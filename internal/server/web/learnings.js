// learnings.js — recent learnings panel modal with area filter + detail view.

import { fetchJSON, escapeHtml, copyText } from './util.js';
import { renderMarkdown } from './markdown.js';
import { toast } from './toasts.js';

const btnEl = document.getElementById('learnings-btn');
let modalEl;
let cached = [];
let areaFilter = '';

const KIND_ICONS = {
  pattern: '◇',
  recipe:  '☰',
  pitfall: '⚠',
  glossary:'¶',
  decision:'◆',
  reference:'§',
};

export function initLearnings() {
  btnEl?.addEventListener('click', open);
}

function ensureModal() {
  if (modalEl) return modalEl;
  modalEl = document.createElement('div');
  modalEl.className = 'action-modal learnings-modal';
  modalEl.hidden = true;
  modalEl.innerHTML = `
    <div class="action-modal-backdrop" data-close></div>
    <aside class="action-modal-panel learnings-panel" role="dialog" aria-label="Learnings">
      <header class="action-modal-head">
        <span class="action-modal-title">LEARNINGS</span>
        <select id="learnings-area-filter" class="learnings-area-filter">
          <option value="">all areas</option>
        </select>
        <button class="icon-btn" data-close aria-label="Close">✕</button>
      </header>
      <div class="learnings-split">
        <div class="learnings-list" id="learnings-list"></div>
        <div class="learnings-detail" id="learnings-detail" hidden></div>
      </div>
    </aside>`;
  document.body.appendChild(modalEl);
  modalEl.addEventListener('click', (e) => {
    if (e.target.closest('[data-close]')) close();
  });
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && modalEl && !modalEl.hidden) close();
  });
  modalEl.querySelector('#learnings-area-filter').addEventListener('change', (e) => {
    areaFilter = e.target.value;
    renderList();
  });
  return modalEl;
}

export async function open() {
  const el = ensureModal();
  el.hidden = false;
  requestAnimationFrame(() => el.classList.add('show'));
  await loadAndRender();
}

function close() {
  if (!modalEl) return;
  modalEl.classList.remove('show');
  setTimeout(() => { modalEl.hidden = true; }, 180);
}

async function loadAndRender() {
  const list = modalEl.querySelector('#learnings-list');
  list.innerHTML = '<div class="nav-loading">loading…</div>';
  try {
    cached = await fetchJSON('/api/learnings');
    populateAreaFilter();
    renderList();
  } catch (err) {
    list.innerHTML = `<div class="nav-error">${escapeHtml(err.message)}</div>`;
  }
}

function populateAreaFilter() {
  const sel = modalEl.querySelector('#learnings-area-filter');
  const areas = [...new Set(cached.map((l) => l.area).filter(Boolean))].sort();
  const current = sel.value;
  sel.innerHTML = '<option value="">all areas</option>' +
    areas.map((a) => `<option value="${escapeHtml(a)}">${escapeHtml(a)}</option>`).join('');
  if (areas.includes(current)) sel.value = current;
}

function renderList() {
  const list = modalEl.querySelector('#learnings-list');
  const filtered = areaFilter
    ? cached.filter((l) => l.area === areaFilter)
    : cached;
  const recent = filtered.slice(0, 20);
  if (!recent.length) {
    list.innerHTML = '<div class="nav-empty">no learnings yet</div>';
    return;
  }
  // group by area
  const byArea = {};
  for (const l of recent) (byArea[l.area || 'misc'] = byArea[l.area || 'misc'] || []).push(l);
  list.innerHTML = Object.entries(byArea).map(([area, ls]) => `
    <div class="learnings-group">
      <div class="learnings-area-head">${escapeHtml(area)} <span class="count">${ls.length}</span></div>
      ${ls.map(rowHtml).join('')}
    </div>`).join('');
  list.querySelectorAll('.learning-row').forEach((row) => {
    row.addEventListener('click', () => openDetail(row.dataset.slug, row.dataset.repoId));
  });
}

function rowHtml(l) {
  const icon = KIND_ICONS[l.kind] || '·';
  return `
    <div class="learning-row" data-slug="${escapeHtml(l.slug)}" data-repo-id="${escapeHtml(l.repo_id || '')}">
      <span class="learning-icon" title="${escapeHtml(l.kind)}">${icon}</span>
      <span class="learning-slug">${escapeHtml(l.slug)}</span>
      <span class="learning-title">${escapeHtml(l.title || '')}</span>
      <span class="learning-by">${escapeHtml(l.created_by || '')}</span>
    </div>`;
}

async function openDetail(slug, repoID) {
  const detail = modalEl.querySelector('#learnings-detail');
  detail.hidden = false;
  detail.innerHTML = '<div class="nav-loading">loading…</div>';
  try {
    const url = '/api/learnings/' + encodeURIComponent(slug) +
      (repoID ? '?repo_id=' + encodeURIComponent(repoID) : '');
    const l = await fetchJSON(url);
    detail.innerHTML = `
      <div class="learning-detail-head">
        <span class="learning-detail-title">${escapeHtml(l.title || l.slug)}</span>
        <button class="icon-btn" data-detail-close>✕</button>
      </div>
      <div class="learning-detail-meta">
        <span>kind: ${escapeHtml(l.kind || '')}</span>
        <span>area: ${escapeHtml(l.area || '')}</span>
        <span>by: ${escapeHtml(l.created_by || '')}</span>
        <span>state: ${escapeHtml(l.state || '')}</span>
      </div>
      ${(l.paths && l.paths.length) ? `
        <div class="learning-paths">
          <div class="learning-paths-head">paths</div>
          <ul>${l.paths.map((p) => `
            <li><code>${escapeHtml(p)}</code>
              <button type="button" class="learning-copy" data-path="${escapeHtml(p)}">copy</button>
            </li>`).join('')}
          </ul>
        </div>` : ''}
      <div class="md learning-detail-body"></div>`;
    detail.querySelector('.learning-detail-body').innerHTML = renderMarkdown(l.body_markdown || '');
    detail.querySelector('[data-detail-close]')?.addEventListener('click', () => {
      detail.hidden = true;
    });
    detail.querySelectorAll('.learning-copy').forEach((b) => {
      b.addEventListener('click', () => {
        copyText(b.dataset.path);
        toast({ kind: 'ok', title: 'Copied path' });
      });
    });
  } catch (err) {
    detail.innerHTML = `<div class="nav-error">${escapeHtml(err.message)}</div>`;
  }
}
