// identicon.js — deterministic GitHub-style 5x5 symmetric SVG avatar.
// Same id always produces the same pattern + color. Wrapped in a <span>
// so existing .identicon CSS (sizing, border, tooltip) keeps working.

const PALETTE = [
  '#e0a860', '#6b8bb0', '#8a7fb0', '#6cb074', '#d88fa0', '#c0b07a', '#7ab0c0', '#b08a6c',
  '#9fb06b', '#b06bc0', '#6b9fc0', '#c0907a', '#7ac09f', '#a07ab0', '#b0a06c', '#8bb06c',
];

function hash(s) {
  let h = 2166136261 >>> 0;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return h >>> 0;
}

export function identicon(id, opts = {}) {
  const size = opts.size || 22;
  const name = opts.name || id || '?';
  const h = hash(String(id || name));

  const grid = 5;
  // Background color from high bits, foreground from low bits so they
  // don't covary with the mask.
  const bg = PALETTE[(h >>> 24) % PALETTE.length];
  const fg = readableFg(bg);

  const cell = size / grid;
  // 15 bits across left three columns (5 rows x 3 cols). Mirror cols 0,1
  // onto cols 3,4 so the pattern is horizontally symmetric.
  const mask = h & 0x7fff;
  let rects = '';
  for (let row = 0; row < grid; row++) {
    for (let col = 0; col < 3; col++) {
      const bit = (mask >> (row * 3 + col)) & 1;
      if (!bit) continue;
      const mirror = col === 2 ? [col] : [col, grid - 1 - col];
      for (const c of mirror) {
        rects += `<rect x="${c * cell}" y="${row * cell}" width="${cell}" height="${cell}"/>`;
      }
    }
  }

  const span = document.createElement('span');
  span.className = 'identicon';
  span.style.width = size + 'px';
  span.style.height = size + 'px';
  span.style.background = bg;
  span.style.color = fg;
  span.title = name;
  span.innerHTML = `<svg viewBox="0 0 ${size} ${size}" width="${size}" height="${size}" style="display:block;fill:${fg};">${rects}</svg>`;
  return span;
}

function readableFg(hex) {
  const [r, g, b] = hexToRgb(hex);
  const y = 0.2126 * r + 0.7152 * g + 0.0722 * b;
  return y > 140 ? '#0a0d14' : '#f0f3f8';
}

function hexToRgb(hex) {
  const n = parseInt(hex.replace('#', ''), 16);
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}
