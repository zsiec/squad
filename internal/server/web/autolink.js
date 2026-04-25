// autolink.js — turn BUG-060 / DEBT-001 / FEAT-007 etc. into clickable chips

import { escapeHtml } from './util.js';

// IDs look like TYPE-DIGITS (2-4 letter type, 1-5 digits). TYPE is one of known types but we
// also accept any 3-6 letter uppercase prefix so new types auto-work.
const ID_RE = /\b([A-Z]{2,6}-\d{1,5})\b/g;

/**
 * Convert a string to HTML where ITEM-IDs become <a data-item-id> chips.
 * Escapes other content.
 */
export function autolinkText(s) {
  const esc = escapeHtml(String(s || ''));
  return esc.replace(ID_RE, (m) => `<a class="item-chip" data-item-id="${m}" href="#${m}">${m}</a>`);
}

/**
 * Walk a rendered HTML DOM subtree and replace bare ITEM-IDs in text nodes with chips.
 * Skips text inside <code>, <pre>, <a>.
 */
export function autolinkDOM(root) {
  const skip = new Set(['A', 'CODE', 'PRE']);
  const walk = document.createTreeWalker(root, NodeFilter.SHOW_TEXT, {
    acceptNode(n) {
      if (!n.nodeValue || !ID_RE.test(n.nodeValue)) return NodeFilter.FILTER_REJECT;
      let p = n.parentElement;
      while (p && p !== root) {
        if (skip.has(p.tagName)) return NodeFilter.FILTER_REJECT;
        p = p.parentElement;
      }
      ID_RE.lastIndex = 0;
      return NodeFilter.FILTER_ACCEPT;
    },
  });
  const victims = [];
  let n;
  while ((n = walk.nextNode())) victims.push(n);
  for (const text of victims) {
    const frag = document.createDocumentFragment();
    const v = text.nodeValue;
    let last = 0;
    ID_RE.lastIndex = 0;
    let m;
    while ((m = ID_RE.exec(v))) {
      if (m.index > last) frag.appendChild(document.createTextNode(v.slice(last, m.index)));
      const a = document.createElement('a');
      a.className = 'item-chip';
      a.dataset.itemId = m[1];
      a.href = '#' + m[1];
      a.textContent = m[1];
      frag.appendChild(a);
      last = m.index + m[0].length;
    }
    if (last < v.length) frag.appendChild(document.createTextNode(v.slice(last)));
    text.parentNode.replaceChild(frag, text);
  }
}

/**
 * Attach a click listener at the document root that fires 'sf:open-item'
 * when any .item-chip is clicked.
 */
export function wireAutolinkClicks() {
  document.addEventListener('click', (e) => {
    const a = e.target.closest('a.item-chip');
    if (!a) return;
    e.preventDefault();
    const id = a.dataset.itemId;
    if (id) document.dispatchEvent(new CustomEvent('sf:open-item', { detail: { id } }));
  });
}
