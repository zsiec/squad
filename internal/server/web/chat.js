// chat.js — right-side chat panel with anchor-to-bottom + unread pill

import { fetchJSON, postJSON, escapeHtml, fmtTs } from './util.js';
import { attachScrollAnchor } from './scroll-anchor.js';
import { autolinkText } from './autolink.js';
import { attachMentionAutocomplete } from './mention.js';
import { parseHandoff, renderHandoffCompactHTML, openHandoffModal } from './handoff.js';
import { displayName } from './names.js';

let agentsProvider = () => [];
export function setAgentsProvider(fn) { agentsProvider = fn; }

const msgEl        = document.getElementById('messages');
const threadSelect = document.getElementById('thread-select');
const composeForm  = document.getElementById('compose');
const composeInput = document.getElementById('compose-input');
const pillEl       = document.getElementById('chat-scroll-end');

const anchor = attachScrollAnchor(msgEl, pillEl);

const KIND_TAGS = new Set([
  'thinking', 'stuck', 'milestone', 'fyi', 'ask', 'review_req', 'handoff',
]);

function currentUser() {
  const el = document.getElementById('user');
  return el?.dataset?.agentId || el?.textContent?.trim() || '';
}

const LONG_BODY = 280;

function messageLi(m) {
  const ts = fmtTs(m.ts || m.TS);
  const agent = m.agent_id || m.AgentID || '';
  const agentPretty = displayName(agent, m.display_name || m.DisplayName);
  const body = m.body || m.Body || '';
  const kind = (m.kind || '').toLowerCase();

  const me = currentUser();
  const mentioned = me && (body.match(/@(\S+)/g) || []).some((t) => t.slice(1) === me);

  // special case: handoff payload is JSON, render compact in chat + open modal on click
  const handoff = kind === 'handoff' ? parseHandoff(body) : null;

  const kindTag = KIND_TAGS.has(kind)
    ? `<span class="kind-tag ${kind}">${kind}</span>`
    : '';

  const li = document.createElement('li');
  if (mentioned) li.classList.add('is-mention');
  if (handoff) li.classList.add('has-handoff');

  const isLong = !handoff && (body.length > LONG_BODY || (body.match(/\n/g) || []).length >= 3);
  if (isLong) li.classList.add('collapsed');

  const bodyHtml = handoff ? renderHandoffCompactHTML(handoff) : renderBody(body);

  li.innerHTML =
    `<span class="ts">${ts}</span>` +
    `<span class="body-col"><span class="agent" title="${escapeHtml(agent)}">${escapeHtml(agentPretty)}</span>${kindTag}${bodyHtml}</span>`;

  if (handoff) {
    li.addEventListener('click', (e) => {
      if (e.target.tagName === 'A') return;
      openHandoffModal(handoff, { agent: agentPretty, ts });
    });
    li.style.cursor = 'pointer';
  } else if (isLong) {
    li.addEventListener('click', (e) => {
      if (e.target.tagName === 'A') return;
      li.classList.toggle('collapsed');
    });
  }
  return li;
}

function renderBody(body) {
  // highlight @mentions + autolink ITEM-IDs; otherwise escape
  const parts = String(body).split(/(@\S+)/);
  return parts
    .map((p) => (p.startsWith('@') ? `<strong class="mention">${escapeHtml(p)}</strong>` : autolinkText(p)))
    .join('');
}

export async function refreshMessages() {
  const thread = threadSelect.value;
  const messages = await fetchJSON('/api/messages?thread=' + encodeURIComponent(thread) + '&limit=50');
  msgEl.innerHTML = '';
  for (const m of messages.reverse()) msgEl.appendChild(messageLi(m));
  anchor.resetUnread();
  // scroll to bottom on thread load regardless of prior scroll position
  requestAnimationFrame(() => anchor.stick());
}

export function appendLive(m) {
  const thread = threadSelect.value;
  if (m.thread !== thread) return;
  msgEl.appendChild(messageLi(m));
  anchor.onContentChange();
}

export function populateThreads(claims) {
  const current = threadSelect.value;
  threadSelect.innerHTML =
    '<option value="global">#global</option>' +
    claims.map((c) => `<option value="${escapeHtml(c.item_id)}">#${escapeHtml(c.item_id)}</option>`).join('');
  threadSelect.value = claims.find((c) => c.item_id === current) ? current : 'global';
}

export function setThread(value) {
  threadSelect.value = value;
  refreshMessages();
}

threadSelect.addEventListener('change', refreshMessages);

attachMentionAutocomplete(composeInput, () => agentsProvider());

const composeError = document.createElement('div');
composeError.className = 'compose-error';
composeError.hidden = true;
composeError.setAttribute('role', 'alert');
composeForm.insertAdjacentElement('afterend', composeError);
composeInput.addEventListener('input', () => { composeError.hidden = true; });

composeForm.addEventListener('submit', async (e) => {
  e.preventDefault();
  const body = composeInput.value.trim();
  if (!body) return;
  try {
    await postJSON('/api/messages', { thread: threadSelect.value, body });
    composeInput.value = '';
    composeError.hidden = true;
    composeInput.focus();
  } catch (err) {
    composeError.textContent = 'send failed: ' + (err.message || String(err));
    composeError.hidden = false;
    composeInput.focus();
  }
});
