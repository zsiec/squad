// mention.js — @-autocomplete for compose inputs
//
// attachMentionAutocomplete(inputEl, () => agents[])
//   agents: [{ agent_id, display_name }]
//
// When user types "@" the dropdown appears below the caret-ish anchor (the input).
// Arrow keys navigate, Enter inserts "@agent-id ", Esc closes.

import { escapeHtml } from './util.js';
import { displayName } from './names.js';

export function attachMentionAutocomplete(inputEl, getAgents) {
  const dd = document.createElement('div');
  dd.className = 'mention-dropdown';
  dd.hidden = true;
  document.body.appendChild(dd);

  let open = false;
  let matches = [];
  let selected = 0;

  function close() { open = false; dd.hidden = true; }

  function currentTrigger() {
    const v = inputEl.value;
    const pos = inputEl.selectionStart ?? v.length;
    const upto = v.slice(0, pos);
    const m = /(^|\s)@([\w-]*)$/.exec(upto);
    if (!m) return null;
    return { start: pos - m[2].length - 1, query: m[2], end: pos };
  }

  function render() {
    if (!matches.length) { close(); return; }
    dd.innerHTML = matches.map((a, i) => {
      const id = a.agent_id || a.AgentID || '';
      const name = displayName(id, a.display_name || a.DisplayName);
      return `<div class="mention-item${i === selected ? ' selected' : ''}" data-idx="${i}">
        <span class="mention-id">${escapeHtml(id)}</span>
        <span class="mention-name">${escapeHtml(name)}</span>
      </div>`;
    }).join('');
    positionDropdown();
    dd.hidden = false;
    open = true;
  }

  function positionDropdown() {
    const r = inputEl.getBoundingClientRect();
    dd.style.left = r.left + 'px';
    dd.style.top  = (r.top - dd.offsetHeight - 4) + 'px';
    dd.style.minWidth = Math.max(220, r.width * 0.5) + 'px';
  }

  function commit() {
    const trig = currentTrigger();
    if (!trig || !matches[selected]) return;
    const a = matches[selected];
    const id = a.agent_id || a.AgentID || '';
    const v = inputEl.value;
    inputEl.value = v.slice(0, trig.start) + '@' + id + ' ' + v.slice(trig.end);
    const caret = trig.start + id.length + 2;
    inputEl.setSelectionRange(caret, caret);
    close();
    inputEl.focus();
  }

  inputEl.addEventListener('input', () => {
    const trig = currentTrigger();
    if (!trig) { close(); return; }
    const q = trig.query.toLowerCase();
    const agents = getAgents();
    matches = agents.filter((a) => {
      const agentId = a.agent_id || a.AgentID || '';
      const id = agentId.toLowerCase();
      const rawName = (a.display_name || a.DisplayName || '').toLowerCase();
      const pretty = displayName(agentId, a.display_name || a.DisplayName).toLowerCase();
      return !q || id.includes(q) || rawName.includes(q) || pretty.includes(q);
    }).slice(0, 8);
    selected = 0;
    render();
  });

  inputEl.addEventListener('keydown', (e) => {
    if (!open) return;
    if (e.key === 'ArrowDown') { e.preventDefault(); selected = (selected + 1) % matches.length; render(); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); selected = (selected - 1 + matches.length) % matches.length; render(); }
    else if (e.key === 'Enter' || e.key === 'Tab') { e.preventDefault(); commit(); }
    else if (e.key === 'Escape') { e.preventDefault(); close(); }
  });

  inputEl.addEventListener('blur', () => setTimeout(close, 120));
  window.addEventListener('scroll', () => open && close(), true);

  dd.addEventListener('mousedown', (e) => {
    const row = e.target.closest('.mention-item');
    if (!row) return;
    e.preventDefault();
    selected = Number(row.dataset.idx);
    commit();
  });
}
