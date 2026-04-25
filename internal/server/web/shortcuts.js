// shortcuts.js — keyboard shortcuts + `?` cheatsheet overlay

const SHORTCUTS = [
  { keys: ['⌘K', 'Ctrl+K'], desc: 'Open command palette' },
  { keys: ['j'],            desc: 'Next item in board' },
  { keys: ['k'],            desc: 'Previous item in board' },
  { keys: ['Enter'],        desc: 'Open selected item' },
  { keys: ['Esc'],          desc: 'Close drawer / palette / overlay' },
  { keys: ['/'],            desc: 'Focus board search' },
  { keys: ['c'],            desc: 'Focus chat compose' },
  { keys: ['1', '2', '3'],  desc: 'Switch board tab (In Progress / Ready / Blocked)' },
  { keys: ['?'],            desc: 'Show this cheatsheet' },
];

export function initShortcuts(handlers) {
  const overlay = mountOverlay();

  document.addEventListener('keydown', (e) => {
    // don't intercept if user is typing in an input/textarea/select
    const t = e.target;
    const typing = t && (t.tagName === 'INPUT' || t.tagName === 'TEXTAREA' || t.tagName === 'SELECT' || t.isContentEditable);

    if (e.key === '?' && !typing) { e.preventDefault(); overlay.removeAttribute('hidden'); return; }
    if (e.key === 'Escape') { overlay.setAttribute('hidden', ''); return; }

    if (typing) return;

    if (e.key === '/') { e.preventDefault(); handlers.focusSearch?.(); return; }
    if (e.key === 'c') { e.preventDefault(); handlers.focusChat?.(); return; }
    if (e.key === 'j' || e.key === 'ArrowDown') { e.preventDefault(); moveSelection(+1, handlers); return; }
    if (e.key === 'k' || e.key === 'ArrowUp')   { e.preventDefault(); moveSelection(-1, handlers); return; }
    if (e.key === 'Enter') { const cur = focusedRow(); if (cur) { handlers.openItem(cur.dataset.item); } return; }
    if (e.key === '1' || e.key === '2' || e.key === '3') {
      const map = { '1': 'in-progress', '2': 'ready', '3': 'blocked' };
      const tab = document.querySelector(`.board-tab[data-section="${map[e.key]}"]`);
      tab?.click();
    }
  });
}

function mountOverlay() {
  const el = document.createElement('div');
  el.id = 'shortcut-overlay';
  el.className = 'shortcut-overlay';
  el.hidden = true;
  el.innerHTML = `
    <div class="shortcut-backdrop" data-close></div>
    <div class="shortcut-panel">
      <div class="shortcut-head">
        <span>Keyboard shortcuts</span>
        <button class="icon-btn" data-close>✕</button>
      </div>
      <ul class="shortcut-list">
        ${SHORTCUTS.map(s => `
          <li class="shortcut-row">
            <span class="shortcut-keys">${s.keys.map(k => `<kbd>${k}</kbd>`).join('')}</span>
            <span class="shortcut-desc">${s.desc}</span>
          </li>`).join('')}
      </ul>
    </div>`;
  document.body.appendChild(el);
  el.addEventListener('click', (e) => {
    if (e.target.closest('[data-close]')) el.setAttribute('hidden', '');
  });
  return el;
}

function moveSelection(delta, handlers) {
  const rows = Array.from(document.querySelectorAll('#board-body tr[data-item]'));
  if (!rows.length) return;
  const curIdx = rows.findIndex(r => r.dataset.selected === 'true');
  let next = curIdx + delta;
  if (next < 0) next = 0;
  if (next >= rows.length) next = rows.length - 1;
  const row = rows[next];
  rows.forEach(r => r.dataset.selected = 'false');
  row.dataset.selected = 'true';
  row.scrollIntoView({ block: 'nearest' });
}

function focusedRow() {
  return document.querySelector('#board-body tr[data-item][data-selected="true"]');
}
