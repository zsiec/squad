// depgraph.js — SVG DAG of the backlog's blocked-by + relates-to edges.
//
// Layout: layered/topological left-to-right. Items with no incoming blocked-by
// edges are in layer 0; each blocked-by pushes the target one layer right.
// relates-to edges are dashed and don't influence layout.

import { fetchJSON, escapeHtml, priClass } from './util.js';

let overlay;
let onOpenItem = () => {};

export function initDepGraph({ onOpen }) {
  onOpenItem = onOpen || (() => {});
  overlay = document.createElement('div');
  overlay.className = 'depgraph';
  overlay.hidden = true;
  overlay.innerHTML = `
    <div class="depgraph-backdrop" data-close></div>
    <section class="depgraph-panel" role="dialog" aria-label="Dependency graph">
      <header class="depgraph-head">
        <span class="depgraph-title">DEPENDENCY GRAPH</span>
        <span class="depgraph-sub" id="depgraph-sub">—</span>
        <button class="icon-btn" data-close aria-label="Close">✕</button>
      </header>
      <div class="depgraph-legend">
        <span><span class="legend-line solid"></span>blocked-by</span>
        <span><span class="legend-line dashed"></span>relates-to</span>
        <span class="legend-note">click any node to open it</span>
      </div>
      <div class="depgraph-scroll" id="depgraph-scroll">
        <div class="depgraph-loading">loading…</div>
      </div>
    </section>
  `;
  document.body.appendChild(overlay);
  overlay.addEventListener('click', (e) => {
    if (e.target.closest('[data-close]')) close();
  });
  document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && !overlay.hidden) close(); });
}

export async function open() {
  overlay.hidden = false;
  requestAnimationFrame(() => overlay.classList.add('show'));

  try {
    const items = await fetchJSON('/api/items');
    const { nodes, edges, layers, stats } = buildDag(items);
    overlay.querySelector('#depgraph-sub').textContent =
      `${stats.itemsWithEdges}/${items.length} items in graph · ${stats.blockEdges} blocks · ${stats.relEdges} relates`;
    overlay.querySelector('#depgraph-scroll').innerHTML = '';
    overlay.querySelector('#depgraph-scroll').appendChild(renderSVG(nodes, edges, layers));
  } catch (err) {
    overlay.querySelector('#depgraph-scroll').innerHTML =
      `<div class="depgraph-error">error: ${escapeHtml(err.message)}</div>`;
  }
}

export function close() {
  overlay.classList.remove('show');
  setTimeout(() => overlay.hidden = true, 240);
}

// ---- DAG construction --------------------------------------------------

async function hydrateEdges(items) {
  // /api/items doesn't include blocked_by/relates_to; fetch details in parallel
  // but only for items that *might* have edges (heuristic: we fetch all).
  // Capped at ~50 concurrent via chunking.
  const out = new Map();
  const chunk = 12;
  for (let i = 0; i < items.length; i += chunk) {
    const slice = items.slice(i, i + chunk);
    const details = await Promise.all(slice.map(async (it) => {
      try {
        return await fetchJSON('/api/items/' + encodeURIComponent(it.id));
      } catch { return null; }
    }));
    for (const d of details) {
      if (d) out.set(d.id, d);
    }
  }
  return out;
}

function buildDag(items) {
  // synchronous scaffold — edges are empty until hydrate resolves.
  // Called from open() which awaits hydrate first.
  return { nodes: [], edges: [], layers: [], stats: { itemsWithEdges: 0, blockEdges: 0, relEdges: 0 } };
}

// Overridden below: open() uses the real builder below.
async function buildDagAsync(items) {
  const details = await hydrateEdges(items);
  const byId = new Map(items.map((it) => [it.id, it]));

  const blockedBy = new Map(); // id -> [other ids that block me]
  const relatesTo = new Map(); // id -> [other ids I relate to]
  for (const [id, d] of details.entries()) {
    if (d.blocked_by?.length) blockedBy.set(id, d.blocked_by.slice());
    if (d.relates_to?.length) relatesTo.set(id, d.relates_to.slice());
  }

  // only keep items that have any edge or are pointed to by an edge
  const referenced = new Set();
  for (const [id, arr] of blockedBy)  { referenced.add(id); arr.forEach((x) => referenced.add(x)); }
  for (const [id, arr] of relatesTo) { referenced.add(id); arr.forEach((x) => referenced.add(x)); }

  const nodes = [];
  for (const id of referenced) {
    const it = byId.get(id);
    nodes.push(it || { id, title: '(unknown)', priority: 'P3', status: '' });
  }

  // layer by topological depth on blocked-by
  // layer[x] = 1 + max(layer[y] for y in blockedBy[x]); cycle-safe via BFS.
  const layer = new Map();
  for (const n of nodes) layer.set(n.id, 0);
  const ids = nodes.map((n) => n.id);
  for (let round = 0; round < ids.length + 2; round++) {
    let changed = false;
    for (const id of ids) {
      const deps = blockedBy.get(id) || [];
      for (const dep of deps) {
        if (!layer.has(dep)) continue;
        const want = (layer.get(dep) || 0) + 1;
        if (want > (layer.get(id) || 0)) {
          layer.set(id, want);
          changed = true;
        }
      }
    }
    if (!changed) break;
  }

  const maxLayer = Math.max(0, ...layer.values());
  const layers = Array.from({length: maxLayer + 1}, () => []);
  for (const n of nodes) layers[layer.get(n.id) || 0].push(n);

  const blockEdges = [];
  const relEdges = [];
  for (const [id, arr] of blockedBy) for (const src of arr) if (layer.has(src)) blockEdges.push([src, id]);
  for (const [id, arr] of relatesTo) for (const other of arr) if (layer.has(other)) relEdges.push([id, other]);

  return {
    nodes, layers,
    edges: { block: blockEdges, relate: relEdges },
    stats: {
      itemsWithEdges: nodes.length,
      blockEdges: blockEdges.length,
      relEdges: relEdges.length,
    },
  };
}

// replace open() with async-aware implementation
export async function openAsync() {
  overlay.hidden = false;
  requestAnimationFrame(() => overlay.classList.add('show'));
  const scroll = overlay.querySelector('#depgraph-scroll');
  scroll.innerHTML = `<div class="depgraph-loading">computing graph…</div>`;
  try {
    const items = await fetchJSON('/api/items');
    const built = await buildDagAsync(items);
    overlay.querySelector('#depgraph-sub').textContent =
      `${built.stats.itemsWithEdges}/${items.length} items in graph · ${built.stats.blockEdges} blocks · ${built.stats.relEdges} relates`;
    scroll.innerHTML = '';
    scroll.appendChild(renderSVG(built.nodes, built.edges, built.layers));
  } catch (err) {
    scroll.innerHTML = `<div class="depgraph-error">error: ${escapeHtml(err.message)}</div>`;
  }
}

// ---- SVG rendering ------------------------------------------------------

function renderSVG(nodes, edges, layers) {
  const NODE_W = 150;
  const NODE_H = 36;
  const HGAP   = 60;
  const VGAP   = 14;

  const positions = new Map();
  let maxRows = 0;
  for (let L = 0; L < layers.length; L++) {
    const col = layers[L];
    maxRows = Math.max(maxRows, col.length);
    // sort by priority then id for stability
    col.sort((a, b) => (a.priority || 'P3').localeCompare(b.priority || 'P3') || a.id.localeCompare(b.id));
    for (let R = 0; R < col.length; R++) {
      positions.set(col[R].id, {
        x: 20 + L * (NODE_W + HGAP),
        y: 20 + R * (NODE_H + VGAP),
      });
    }
  }
  const W = 40 + layers.length * (NODE_W + HGAP);
  const H = 40 + maxRows * (NODE_H + VGAP);

  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('viewBox', `0 0 ${W} ${H}`);
  svg.setAttribute('width', W);
  svg.setAttribute('height', H);
  svg.classList.add('depgraph-svg');

  // edges first (behind nodes)
  const gEdges = document.createElementNS('http://www.w3.org/2000/svg', 'g');
  gEdges.setAttribute('class', 'edges');
  for (const [a, b] of edges.block) {
    const path = edgePath(positions.get(a), positions.get(b), NODE_W, NODE_H);
    if (path) appendEdge(gEdges, path, 'block');
  }
  for (const [a, b] of edges.relate) {
    const path = edgePath(positions.get(a), positions.get(b), NODE_W, NODE_H);
    if (path) appendEdge(gEdges, path, 'relate');
  }
  svg.appendChild(gEdges);

  // arrow marker
  const defs = document.createElementNS('http://www.w3.org/2000/svg', 'defs');
  defs.innerHTML = `
    <marker id="arrow-block" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="5" markerHeight="5" orient="auto">
      <path d="M 0 0 L 8 4 L 0 8 z" fill="#6b8bb0"/>
    </marker>
    <marker id="arrow-relate" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="4" markerHeight="4" orient="auto">
      <path d="M 0 0 L 8 4 L 0 8 z" fill="#5b6474"/>
    </marker>
  `;
  svg.insertBefore(defs, gEdges);

  // nodes
  for (const n of nodes) {
    const pos = positions.get(n.id);
    if (!pos) continue;
    const g = document.createElementNS('http://www.w3.org/2000/svg', 'g');
    g.classList.add('node');
    g.classList.add(priClass(n.priority));
    g.dataset.id = n.id;
    g.setAttribute('transform', `translate(${pos.x}, ${pos.y})`);
    g.innerHTML = `
      <rect class="node-bg" x="0" y="0" width="${NODE_W}" height="${NODE_H}" rx="2"/>
      <rect class="node-pri" x="0" y="0" width="3" height="${NODE_H}"/>
      <text class="node-id"    x="10" y="14">${escapeHtml(n.id)}</text>
      <text class="node-title" x="10" y="28">${escapeHtml((n.title || '').slice(0, 26))}${(n.title||'').length > 26 ? '…' : ''}</text>
    `;
    g.addEventListener('click', () => {
      close();
      onOpenItem(n.id);
    });
    svg.appendChild(g);
  }

  return svg;
}

function edgePath(a, b, nodeW, nodeH) {
  if (!a || !b) return null;
  const x1 = a.x + nodeW;
  const y1 = a.y + nodeH / 2;
  const x2 = b.x;
  const y2 = b.y + nodeH / 2;
  // simple cubic bezier (right-to-left arc if direction reversed)
  const dx = Math.max(20, Math.abs(x2 - x1) / 2);
  return `M ${x1} ${y1} C ${x1+dx} ${y1}, ${x2-dx} ${y2}, ${x2} ${y2}`;
}

function appendEdge(g, d, kind) {
  const p = document.createElementNS('http://www.w3.org/2000/svg', 'path');
  p.setAttribute('d', d);
  p.setAttribute('fill', 'none');
  p.classList.add('edge', 'edge-' + kind);
  p.setAttribute('marker-end', `url(#arrow-${kind})`);
  g.appendChild(p);
}
