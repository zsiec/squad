// actions.js — item mutation buttons + handoff/new-item modals
//
// Server endpoints used: POST /api/items/{id}/{claim,release,done,blocked,
// handoff,touch,force-release}, POST /api/items.
//
// All mutations rely on X-Squad-Agent (set on page load via util.setAgentHeader).
// Visibility rules per cta are encoded in renderItemActions().

import { postJSON, fetchJSON, escapeHtml } from './util.js';
import { toast } from './toasts.js';
import { agentsSnapshot } from './board.js';
import { displayName } from './names.js';

const STALE_CLAIM_SEC = 24 * 3600;

let onMutated = () => {};
export function setOnMutated(fn) { onMutated = fn; }

function meAgentId() {
  return document.getElementById('user')?.dataset?.agentId || '';
}

export function renderItemActions(it) {
  const me = meAgentId();
  if (!me) return '';
  const claim = it.current_claim || null;
  const heldByMe = !!(claim && claim.agent_id === me);
  const unclaimed = !claim;
  const stale = claim && claim.claimed_at &&
    (Math.floor(Date.now() / 1000) - claim.claimed_at) > STALE_CLAIM_SEC;

  const btn = (action, label, cls) =>
    `<button type="button" class="action-btn ${cls || ''}" data-action="${action}" data-id="${escapeHtml(it.id)}">${escapeHtml(label)}</button>`;

  const out = [];
  if (unclaimed)         out.push(btn('claim',         'Claim',         'primary'));
  if (heldByMe)          out.push(btn('release',       'Release'));
  if (heldByMe)          out.push(btn('done',          'Done',          'ok'));
  if (heldByMe)          out.push(btn('blocked',       'Blocked',       'warn'));
  if (heldByMe)          out.push(btn('handoff',       'Handoff'));
  if (heldByMe)          out.push(btn('touch',         'Touch',         'ghost'));
  // Recapture sends a needs-refinement item back to captured. Visible only
  // when the click would actually succeed: caller must be able to claim
  // (unclaimed) or already hold it (heldByMe). A peer's claim hides the
  // button rather than surfacing a 403 on click.
  if (it.status === 'needs-refinement' && (unclaimed || heldByMe)) {
    out.push(btn('recapture', 'Send back to captured', 'warn'));
  }
  if (claim && stale)    out.push(btn('force-release', 'Force release', 'danger'));

  if (!out.length) return '';
  return `<div class="action-toolbar" data-actions>${out.join('')}</div>`;
}

export function wireItemActions(root, item) {
  root.querySelectorAll('button.action-btn[data-action]').forEach((b) => {
    b.addEventListener('click', async () => {
      const action = b.dataset.action;
      const id = b.dataset.id;
      try {
        await runAction(action, id, item);
      } catch (err) {
        toast({ kind: 'error', title: action + ' failed', body: err.message });
      }
    });
  });
}

async function runAction(action, id, item) {
  switch (action) {
    case 'claim': {
      const intent = window.prompt('Intent (one short line):', '');
      if (intent === null) return;
      await postJSON(`/api/items/${encodeURIComponent(id)}/claim`, { intent: intent || '' });
      toast({ kind: 'ok', title: `Claimed ${id}` });
      onMutated(id);
      return;
    }
    case 'release': {
      await postJSON(`/api/items/${encodeURIComponent(id)}/release`, { outcome: 'released' });
      toast({ kind: 'ok', title: `Released ${id}` });
      onMutated(id);
      return;
    }
    case 'done': {
      if (!window.confirm(`Mark ${id} done?`)) return;
      try {
        await postJSON(`/api/items/${encodeURIComponent(id)}/done`, {});
      } catch (err) {
        if (/precondition failed/i.test(err.message) || /412/.test(err.message)) {
          if (window.confirm('Evidence missing. Force done anyway?')) {
            await postJSON(`/api/items/${encodeURIComponent(id)}/done`, { evidence_force: true });
          } else {
            return;
          }
        } else {
          throw err;
        }
      }
      toast({ kind: 'ok', title: `${id} shipped` });
      onMutated(id);
      return;
    }
    case 'blocked': {
      const reason = window.prompt('Blocked on (item id or short reason):', '');
      if (!reason) return;
      await postJSON(`/api/items/${encodeURIComponent(id)}/blocked`, { reason });
      toast({ kind: 'warn', title: `${id} blocked`, body: reason });
      onMutated(id);
      return;
    }
    case 'handoff': {
      openHandoffModal(id);
      return;
    }
    case 'touch': {
      await postJSON(`/api/items/${encodeURIComponent(id)}/touch`, {});
      onMutated(id);
      return;
    }
    case 'force-release': {
      const reason = window.prompt('Reason for force-release (required):', '');
      if (!reason) return;
      if (!window.confirm(`Force-release ${id}?`)) return;
      await postJSON(`/api/items/${encodeURIComponent(id)}/force-release`, { reason });
      toast({ kind: 'ok', title: `Force-released ${id}` });
      onMutated(id);
      return;
    }
    case 'recapture': {
      // Backend recapture requires the caller to hold the claim. If the
      // item is unclaimed, take it under the current agent first; if we
      // already hold it, skip the claim step.
      const claim = item.current_claim || null;
      if (!claim) {
        await postJSON(`/api/items/${encodeURIComponent(id)}/claim`, { intent: 'send back to captured' });
      }
      await postJSON(`/api/items/${encodeURIComponent(id)}/recapture`, {});
      toast({ kind: 'ok', title: `Sent ${id} back to captured` });
      onMutated(id);
      return;
    }
    default:
      throw new Error('unknown action: ' + action);
  }
}

// ---- handoff modal ------------------------------------------------------

let handoffModal;
function ensureHandoffModal() {
  if (handoffModal) return handoffModal;
  handoffModal = document.createElement('div');
  handoffModal.className = 'action-modal handoff-action-modal';
  handoffModal.hidden = true;
  handoffModal.innerHTML = `
    <div class="action-modal-backdrop" data-close></div>
    <aside class="action-modal-panel" role="dialog" aria-label="Handoff">
      <header class="action-modal-head">
        <span class="action-modal-title">HANDOFF</span>
        <button class="icon-btn" data-close aria-label="Close">✕</button>
      </header>
      <form class="action-modal-body" id="handoff-form">
        <label>Item <input type="text" name="item_id" readonly></label>
        <label>To agent
          <select name="to_agent" required></select>
        </label>
        <label>Intent
          <textarea name="intent" rows="5" placeholder="What still needs doing? Context for the next holder…" required></textarea>
        </label>
        <label class="checkbox">
          <input type="checkbox" name="include_progress">
          include current AC progress
        </label>
        <div class="action-modal-foot">
          <button type="button" class="action-btn ghost" data-close>Cancel</button>
          <button type="submit" class="action-btn primary">Hand off</button>
        </div>
      </form>
    </aside>`;
  document.body.appendChild(handoffModal);
  handoffModal.addEventListener('click', (e) => {
    if (e.target.closest('[data-close]')) closeHandoffModal();
  });
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && handoffModal && !handoffModal.hidden) closeHandoffModal();
  });
  handoffModal.querySelector('#handoff-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const fd = new FormData(e.target);
    const itemId = fd.get('item_id');
    const payload = {
      to_agent: fd.get('to_agent'),
      intent: fd.get('intent'),
      include_progress: fd.get('include_progress') === 'on',
    };
    try {
      await postJSON(`/api/items/${encodeURIComponent(itemId)}/handoff`, payload);
      toast({ kind: 'ok', title: `Handed off ${itemId} to ${payload.to_agent}` });
      closeHandoffModal();
      onMutated(itemId);
    } catch (err) {
      toast({ kind: 'error', title: 'Handoff failed', body: err.message });
    }
  });
  return handoffModal;
}

export function openHandoffModal(itemId) {
  const el = ensureHandoffModal();
  const me = meAgentId();
  el.querySelector('input[name=item_id]').value = itemId;
  const sel = el.querySelector('select[name=to_agent]');
  const agents = agentsSnapshot();
  sel.innerHTML = agents
    .filter((a) => (a.agent_id || a.AgentID) !== me)
    .map((a) => {
      const id = a.agent_id || a.AgentID || '';
      const label = displayName(id, a.display_name || a.DisplayName);
      return `<option value="${escapeHtml(id)}">${escapeHtml(label)}</option>`;
    }).join('');
  el.querySelector('textarea[name=intent]').value = '';
  el.querySelector('input[name=include_progress]').checked = false;
  el.hidden = false;
  requestAnimationFrame(() => el.classList.add('show'));
  setTimeout(() => sel.focus(), 50);
}
function closeHandoffModal() {
  if (!handoffModal) return;
  handoffModal.classList.remove('show');
  setTimeout(() => { handoffModal.hidden = true; }, 180);
}

// ---- new item modal -----------------------------------------------------

let newItemModal;
function ensureNewItemModal() {
  if (newItemModal) return newItemModal;
  newItemModal = document.createElement('div');
  newItemModal.className = 'action-modal new-item-modal';
  newItemModal.hidden = true;
  newItemModal.innerHTML = `
    <div class="action-modal-backdrop" data-close></div>
    <aside class="action-modal-panel" role="dialog" aria-label="New item">
      <header class="action-modal-head">
        <span class="action-modal-title">NEW ITEM</span>
        <button class="icon-btn" data-close aria-label="Close">✕</button>
      </header>
      <form class="action-modal-body" id="new-item-form">
        <label data-repo-slot hidden></label>
        <label>Type
          <select name="type" required>
            <option value="BUG">BUG</option>
            <option value="FEAT">FEAT</option>
            <option value="TASK">TASK</option>
            <option value="CHORE">CHORE</option>
          </select>
        </label>
        <label>Title
          <input type="text" name="title" required maxlength="240" autocomplete="off" placeholder="One-line summary">
        </label>
        <label>Area
          <input type="text" name="area" maxlength="60" autocomplete="off" placeholder="e.g. server, web, claims">
        </label>
        <label>Priority
          <select name="priority">
            <option value="P2" selected>P2</option>
            <option value="P0">P0</option>
            <option value="P1">P1</option>
            <option value="P3">P3</option>
          </select>
        </label>
        <div class="action-modal-foot">
          <button type="button" class="action-btn ghost" data-close>Cancel</button>
          <button type="submit" class="action-btn primary">Create</button>
        </div>
      </form>
    </aside>`;
  document.body.appendChild(newItemModal);
  newItemModal.addEventListener('click', (e) => {
    if (e.target.closest('[data-close]')) closeNewItemModal();
  });
  document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && newItemModal && !newItemModal.hidden) closeNewItemModal();
  });
  newItemModal.querySelector('#new-item-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const fd = new FormData(e.target);
    const payload = {
      type: fd.get('type'),
      title: fd.get('title'),
      area: fd.get('area') || '',
      priority: fd.get('priority') || 'P2',
    };
    const repoID = (fd.get('repo_id') || '').toString();
    const url = '/api/items' + (repoID ? '?repo_id=' + encodeURIComponent(repoID) : '');
    try {
      const out = await postJSON(url, payload);
      toast({ kind: 'ok', title: 'Created ' + (out?.id || payload.type) });
      closeNewItemModal();
      if (out?.id) onMutated(out.id, { open: true });
      else onMutated('', { open: false });
    } catch (err) {
      toast({ kind: 'error', title: 'Create failed', body: err.message });
    }
  });
  return newItemModal;
}

// renderRepoSlot fetches /api/repos and, when the workspace spans more
// than one repo, paints a labelled <select name="repo_id"> into the
// reserved slot. With exactly one repo (or zero, in degenerate setups)
// the slot stays hidden so the single-repo modal looks unchanged —
// matches the multi-repo gate that board.js uses for the row badge.
async function renderRepoSlot(form) {
  const slot = form.querySelector('[data-repo-slot]');
  slot.hidden = true;
  slot.innerHTML = '';
  let repos;
  try {
    repos = await fetchJSON('/api/repos');
  } catch {
    return;
  }
  if (!Array.isArray(repos) || repos.length < 2) return;
  const opts = repos.map((r, i) => {
    const id = r?.repo_id || '';
    const sel = i === 0 ? ' selected' : '';
    return `<option value="${escapeHtml(id)}"${sel}>${escapeHtml(id)}</option>`;
  }).join('');
  slot.innerHTML = `Repo
    <select name="repo_id" required>${opts}</select>`;
  slot.hidden = false;
}

export function openNewItemModal() {
  const el = ensureNewItemModal();
  const form = el.querySelector('form');
  form.reset();
  renderRepoSlot(form);
  el.hidden = false;
  requestAnimationFrame(() => el.classList.add('show'));
  setTimeout(() => el.querySelector('input[name=title]')?.focus(), 50);
}
function closeNewItemModal() {
  if (!newItemModal) return;
  newItemModal.classList.remove('show');
  setTimeout(() => { newItemModal.hidden = true; }, 180);
}

