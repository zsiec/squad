// sidebar.js — left navigation: specs → epics → items hierarchy.
//
// Lazy expansion (epics on spec click, items on epic click). Clicking a
// spec opens a markdown detail panel. Clicking an item opens the drawer.

import { fetchJSON, escapeHtml } from './util.js';
import { renderMarkdown } from './markdown.js';
import { setFilter } from './board.js';

const sidebarEl = document.getElementById('nav-sidebar');
const treeEl    = document.getElementById('nav-tree');
const detailEl  = document.getElementById('nav-detail');
const closeBtn  = document.getElementById('nav-close');
const toggleBtn = document.getElementById('nav-toggle-btn');

let onOpenItem = () => {};
let loaded = false;

export function initSidebar({ onItem }) {
  if (typeof onItem === 'function') onOpenItem = onItem;
  toggleBtn?.addEventListener('click', toggle);
  closeBtn?.addEventListener('click', close);
}

export function toggle() {
  if (sidebarEl.hidden) open();
  else close();
}

export async function open() {
  sidebarEl.hidden = false;
  if (!loaded) {
    loaded = true;
    await renderSpecs();
  }
}

export function close() {
  sidebarEl.hidden = true;
}

async function renderSpecs() {
  treeEl.innerHTML = '<div class="nav-loading">loading…</div>';
  try {
    const specs = await fetchJSON('/api/specs');
    if (!specs.length) {
      treeEl.innerHTML = '<div class="nav-empty">no specs yet</div>';
      return;
    }
    treeEl.innerHTML = specs.map(specRow).join('');
    treeEl.querySelectorAll('.nav-spec').forEach((row) => wireSpec(row));
  } catch (err) {
    treeEl.innerHTML = `<div class="nav-error">${escapeHtml(err.message)}</div>`;
  }
}

function specRow(s) {
  return `
    <details class="nav-spec" data-name="${escapeHtml(s.name)}" data-repo-id="${escapeHtml(s.repo_id || '')}">
      <summary>
        <span class="nav-caret">▸</span>
        <span class="nav-spec-name">${escapeHtml(s.name)}</span>
        <button type="button" class="nav-spec-view" title="View spec">view</button>
      </summary>
      <div class="nav-epics" data-loaded="false"></div>
    </details>`;
}

function wireSpec(row) {
  const name = row.dataset.name;
  const repoID = row.dataset.repoId || '';
  const epicsHost = row.querySelector('.nav-epics');
  row.addEventListener('toggle', async () => {
    if (!row.open) return;
    if (epicsHost.dataset.loaded === 'true') return;
    epicsHost.innerHTML = '<div class="nav-loading">loading…</div>';
    try {
      const epics = await fetchJSON('/api/epics?spec=' + encodeURIComponent(name));
      if (!epics.length) {
        epicsHost.innerHTML = '<div class="nav-empty-inline">no epics</div>';
      } else {
        epicsHost.innerHTML = epics.map(epicRow).join('');
        epicsHost.querySelectorAll('.nav-epic').forEach(wireEpic);
      }
      epicsHost.dataset.loaded = 'true';
    } catch (err) {
      epicsHost.innerHTML = `<div class="nav-error">${escapeHtml(err.message)}</div>`;
    }
  });
  row.querySelector('.nav-spec-view')?.addEventListener('click', async (e) => {
    e.preventDefault();
    e.stopPropagation();
    await openSpecDetail(name, repoID);
  });
}

function epicRow(e) {
  return `
    <details class="nav-epic" data-name="${escapeHtml(e.name)}">
      <summary>
        <span class="nav-caret">▸</span>
        <span class="nav-epic-name">${escapeHtml(e.name)}</span>
        <span class="nav-epic-status">${escapeHtml(e.status || 'open')}</span>
      </summary>
      <div class="nav-items" data-loaded="false"></div>
    </details>`;
}

function wireEpic(row) {
  const name = row.dataset.name;
  const itemsHost = row.querySelector('.nav-items');
  row.addEventListener('toggle', async () => {
    if (!row.open) return;
    setFilter({ epic: name });
    if (itemsHost.dataset.loaded === 'true') return;
    itemsHost.innerHTML = '<div class="nav-loading">loading…</div>';
    try {
      const items = await fetchJSON('/api/items?epic=' + encodeURIComponent(name));
      if (!items.length) {
        itemsHost.innerHTML = '<div class="nav-empty-inline">no items</div>';
      } else {
        itemsHost.innerHTML = items.map(itemRow).join('');
        itemsHost.querySelectorAll('.nav-item').forEach((li) => {
          li.addEventListener('click', () => onOpenItem(li.dataset.id));
        });
      }
      itemsHost.dataset.loaded = 'true';
    } catch (err) {
      itemsHost.innerHTML = `<div class="nav-error">${escapeHtml(err.message)}</div>`;
    }
  });
}

function itemRow(it) {
  return `
    <div class="nav-item" data-id="${escapeHtml(it.id)}">
      <span class="nav-item-id">${escapeHtml(it.id)}</span>
      <span class="nav-item-title">${escapeHtml(it.title || '')}</span>
      <span class="nav-item-status">${escapeHtml(it.status || '')}</span>
    </div>`;
}

async function openSpecDetail(name, repoID) {
  detailEl.hidden = false;
  detailEl.innerHTML = '<div class="nav-loading">loading…</div>';
  try {
    const url = '/api/specs/' + encodeURIComponent(name) +
      (repoID ? '?repo_id=' + encodeURIComponent(repoID) : '');
    const sp = await fetchJSON(url);
    detailEl.innerHTML = `
      <div class="nav-detail-head">
        <span class="nav-detail-title">${escapeHtml(sp.title || sp.name)}</span>
        <button class="icon-btn" data-detail-close>✕</button>
      </div>
      <div class="md nav-detail-body"></div>`;
    detailEl.querySelector('.nav-detail-body').innerHTML =
      renderMarkdown(sp.body_markdown || sp.motivation || '');
    detailEl.querySelector('[data-detail-close]')?.addEventListener('click', () => {
      detailEl.hidden = true;
      detailEl.innerHTML = '';
    });
  } catch (err) {
    detailEl.innerHTML = `<div class="nav-error">${escapeHtml(err.message)}</div>`;
  }
}
