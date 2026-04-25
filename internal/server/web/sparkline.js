// sparkline.js — tiny inline SVG bar chart for per-claim activity velocity

/**
 * sparkline(buckets: number[], { width, height, color })
 * buckets: equal-width counts, newest last.
 */
export function sparkline(buckets, opts = {}) {
  const w = opts.width || 64;
  const h = opts.height || 12;
  const color = opts.color || 'currentColor';
  const max = Math.max(1, ...buckets);
  const n = Math.max(1, buckets.length);
  const barW = w / n;
  const pad = 1;
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('viewBox', `0 0 ${w} ${h}`);
  svg.setAttribute('width', w);
  svg.setAttribute('height', h);
  svg.setAttribute('class', 'sparkline');
  for (let i = 0; i < n; i++) {
    const v = buckets[i];
    const bh = (v / max) * (h - 2);
    const rect = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    rect.setAttribute('x', (i * barW) + pad / 2);
    rect.setAttribute('y', h - bh);
    rect.setAttribute('width', barW - pad);
    rect.setAttribute('height', bh || 1);
    rect.setAttribute('fill', color);
    rect.setAttribute('opacity', v === 0 ? '0.15' : (0.35 + 0.65 * (v / max)).toFixed(2));
    svg.appendChild(rect);
  }
  return svg;
}

// Bucket timestamps (unix seconds) into N equal buckets over the last `windowSec` seconds.
export function bucketize(timestamps, windowSec = 30 * 60, bins = 10) {
  const now = Math.floor(Date.now() / 1000);
  const start = now - windowSec;
  const buckets = new Array(bins).fill(0);
  const size = windowSec / bins;
  for (const ts of timestamps) {
    if (ts < start || ts > now) continue;
    const idx = Math.min(bins - 1, Math.floor((ts - start) / size));
    buckets[idx] += 1;
  }
  return buckets;
}
