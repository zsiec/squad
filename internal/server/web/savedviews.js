// savedviews.js — preset filter chips above the board

import { setFilter } from './board.js';

const VIEWS = [
  { id: 'all',         label: 'All',          apply: { q: '', area: '', type: '', priority: '', view: '' } },
  { id: 'mine',        label: 'My claims',    apply: { view: 'mine' } },
  { id: 'stale',       label: 'Stale >2h',    apply: { view: 'stale' } },
  { id: 'unclaimed-p1',label: 'Unclaimed P1', apply: { priority: 'P1', view: 'unclaimed' } },
  { id: 'blocked',     label: 'Blocked',      apply: { view: 'blocked' } },
  { id: 'today',       label: 'New today',    apply: { view: 'new-today' } },
];

export function initSavedViews() {
  const host = document.getElementById('saved-views');
  if (!host) return;
  let active = 'all';
  host.innerHTML = VIEWS.map((v) =>
    `<button class="view-chip${v.id === active ? ' active' : ''}" data-id="${v.id}">${v.label}</button>`
  ).join('');
  host.addEventListener('click', (e) => {
    const btn = e.target.closest('.view-chip');
    if (!btn) return;
    active = btn.dataset.id;
    host.querySelectorAll('.view-chip').forEach((b) => b.classList.toggle('active', b === btn));
    const v = VIEWS.find((x) => x.id === active);
    if (v) setFilter(v.apply);
  });
}

export const SAVED_VIEWS = VIEWS;
