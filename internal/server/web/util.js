// util.js — shared helpers, fetch

let cachedAgentId = '';
export function setAgentHeader(id) { cachedAgentId = id || ''; }
export function agentHeader() {
  return cachedAgentId ? { 'X-Squad-Agent': cachedAgentId } : {};
}

export async function fetchJSON(path) {
  const r = await fetch(path, { headers: { ...agentHeader() } });
  if (!r.ok) throw new Error(path + ': ' + r.status);
  return r.json();
}

export async function postJSON(path, body) {
  const r = await fetch(path, {
    method: 'POST',
    headers: { ...agentHeader(), 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!r.ok) {
    let msg = path + ': ' + r.status;
    try {
      const j = await r.json();
      if (j?.error) msg += ' — ' + j.error;
    } catch {}
    throw new Error(msg);
  }
  return r.status === 204 ? null : r.json().catch(() => null);
}

export function escapeHtml(s) {
  return String(s || '').replace(/[&<>"']/g, (c) => ({
    '&': '&amp;',
    '<': '&lt;',
    '>': '&gt;',
    '"': '&quot;',
    "'": '&#39;',
  })[c]);
}

export function fmtTs(unix) {
  if (!unix) return '—';
  return new Date(unix * 1000).toTimeString().slice(0, 8);
}

export function fmtHM(unix) {
  if (!unix) return '—';
  return new Date(unix * 1000).toTimeString().slice(0, 5);
}

export function fmtDay(unix) {
  if (!unix) return '';
  const d = new Date(unix * 1000);
  const months = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
  return months[d.getMonth()] + ' ' + String(d.getDate()).padStart(2, '0');
}

export function fmtDate(iso) {
  if (!iso) return '—';
  return String(iso).slice(0, 10);
}

export function fmtAgo(unix) {
  if (!unix) return '—';
  const s = Math.max(0, Math.floor(Date.now() / 1000 - unix));
  if (s < 45)          return s + 's';
  if (s < 3600)        return Math.round(s / 60) + 'm';
  if (s < 86400)       return Math.round(s / 3600) + 'h';
  return Math.round(s / 86400) + 'd';
}

export function priClass(pri) {
  const p = String(pri || '').toLowerCase();
  if (/^p[0-3]$/.test(p)) return p;
  return 'p-unknown';
}

export function debounce(fn, ms) {
  let t;
  return (...a) => {
    clearTimeout(t);
    t = setTimeout(() => fn(...a), ms);
  };
}

export function clock(el) {
  const tick = () => {
    el.textContent = new Date().toTimeString().slice(0, 8);
  };
  tick();
  setInterval(tick, 1000);
}

export function copyText(s) {
  if (navigator.clipboard?.writeText) {
    return navigator.clipboard.writeText(s);
  }
  const ta = document.createElement('textarea');
  ta.value = s;
  ta.style.position = 'fixed';
  ta.style.opacity = '0';
  document.body.appendChild(ta);
  ta.select();
  try { document.execCommand('copy'); } finally { ta.remove(); }
  return Promise.resolve();
}
