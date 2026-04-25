// toasts.js — slide-in top-right notifications

import { escapeHtml } from './util.js';

let host;

export function initToasts() {
  host = document.createElement('div');
  host.className = 'toasts';
  document.body.appendChild(host);
}

/**
 * toast({ title, body, kind, onClick, ttl })
 *   kind: 'mention' | 'knock' | 'info' | 'ok' | 'warn'
 *   ttl:  ms (default 5000)
 */
export function toast(opts) {
  if (!host) return;
  const el = document.createElement('div');
  el.className = 'toast toast-' + (opts.kind || 'info');
  el.innerHTML = `
    <div class="toast-head">
      <span class="toast-kind">${escapeHtml(opts.kind || 'info')}</span>
      <span class="toast-title">${escapeHtml(opts.title || '')}</span>
      <button class="toast-close" aria-label="Dismiss">×</button>
    </div>
    ${opts.body ? `<div class="toast-body">${escapeHtml(opts.body)}</div>` : ''}
  `;
  if (opts.onClick) {
    el.addEventListener('click', (e) => {
      if (e.target.closest('.toast-close')) return;
      opts.onClick();
      dismiss(el);
    });
    el.classList.add('clickable');
  }
  el.querySelector('.toast-close').addEventListener('click', (e) => {
    e.stopPropagation();
    dismiss(el);
  });
  host.appendChild(el);
  // trigger enter animation
  requestAnimationFrame(() => el.classList.add('show'));
  const ttl = opts.ttl ?? 6000;
  if (ttl > 0) setTimeout(() => dismiss(el), ttl);
}

function dismiss(el) {
  el.classList.remove('show');
  el.classList.add('leaving');
  setTimeout(() => el.remove(), 280);
}
