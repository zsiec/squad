// markdown.js — lightweight markdown renderer with proper heading hierarchy

import { escapeHtml } from './util.js';

const INLINE_CODE = /`([^`]+)`/g;
const BOLD        = /\*\*([^*]+)\*\*/g;
const ITAL        = /(^|[^*])\*([^*\n]+)\*/g;
const LINK        = /\[([^\]]+)\]\(([^)]+)\)/g;
// file:line references like server/foo/bar.go:123 or bar.go:12-40
const FILEREF     = /(\b[\w./-]+?\.(?:go|ts|tsx|js|md|cu|metal|m|mm|h|c|cpp|sh|yml|yaml|toml|json)(?::\d+(?:[-–]\d+)?)?)/g;

const SAFE_SCHEME = /^(?:https?:|mailto:)/i;

function isSafeURL(u) {
  const s = u.trim();
  if (!s) return false;
  // protocol-relative (//host/...) inherits the page scheme — treat as off-allowlist.
  if (s.startsWith('//')) return false;
  const colon = s.indexOf(':');
  if (colon < 0) return true;
  const slash = s.indexOf('/');
  const hash = s.indexOf('#');
  const question = s.indexOf('?');
  // relative URL if a path/hash/query delimiter precedes the first colon
  for (const idx of [slash, hash, question]) {
    if (idx >= 0 && idx < colon) return true;
  }
  return SAFE_SCHEME.test(s);
}

function inline(s) {
  return escapeHtml(s)
    .replace(INLINE_CODE, '<code>$1</code>')
    .replace(BOLD, '<strong>$1</strong>')
    .replace(ITAL, '$1<em>$2</em>')
    .replace(LINK, (_, text, url) => {
      if (isSafeURL(url)) {
        return `<a href="${url}" target="_blank" rel="noopener">${text}</a>`;
      }
      return `[${text}](${url})`;
    });
}

export function renderMarkdown(md) {
  const src = String(md || '');
  const lines = src.split('\n');
  const out = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    // fenced code
    const fence = /^```(\w+)?\s*$/.exec(line);
    if (fence) {
      const lang = fence[1] || '';
      const buf = [];
      i++;
      while (i < lines.length && !/^```\s*$/.test(lines[i])) {
        buf.push(lines[i]);
        i++;
      }
      i++; // skip closing
      out.push(
        '<pre><code' + (lang ? ` class="lang-${escapeHtml(lang)}"` : '') + '>' +
        escapeHtml(buf.join('\n')) +
        '</code></pre>'
      );
      continue;
    }

    // headings
    const h = /^(#{1,5})\s+(.*)$/.exec(line);
    if (h) {
      const level = h[1].length;
      out.push(`<h${level}>${inline(h[2])}</h${level}>`);
      i++;
      continue;
    }

    // blockquote
    if (line.startsWith('> ')) {
      const buf = [];
      while (i < lines.length && lines[i].startsWith('> ')) {
        buf.push(lines[i].slice(2));
        i++;
      }
      out.push('<blockquote>' + buf.map(inline).join('<br>') + '</blockquote>');
      continue;
    }

    // horizontal rule
    if (/^---+\s*$/.test(line)) { out.push('<hr>'); i++; continue; }

    // bullet list
    if (/^\s*[-*]\s+/.test(line)) {
      const buf = [];
      while (i < lines.length && /^\s*[-*]\s+/.test(lines[i])) {
        buf.push(lines[i].replace(/^\s*[-*]\s+/, ''));
        i++;
      }
      out.push('<ul>' + buf.map((b) => '<li>' + inline(b) + '</li>').join('') + '</ul>');
      continue;
    }

    // ordered list
    if (/^\s*\d+\.\s+/.test(line)) {
      const buf = [];
      while (i < lines.length && /^\s*\d+\.\s+/.test(lines[i])) {
        buf.push(lines[i].replace(/^\s*\d+\.\s+/, ''));
        i++;
      }
      out.push('<ol>' + buf.map((b) => '<li>' + inline(b) + '</li>').join('') + '</ol>');
      continue;
    }

    // blank line
    if (line.trim() === '') { i++; continue; }

    // paragraph — collect consecutive non-blank non-list non-heading lines
    const buf = [line];
    i++;
    while (i < lines.length) {
      const nxt = lines[i];
      if (nxt.trim() === '') break;
      if (/^#{1,5}\s+/.test(nxt)) break;
      if (/^\s*[-*]\s+/.test(nxt)) break;
      if (/^\s*\d+\.\s+/.test(nxt)) break;
      if (/^```/.test(nxt)) break;
      if (nxt.startsWith('> ')) break;
      buf.push(nxt);
      i++;
    }
    out.push('<p>' + buf.map(inline).join(' ') + '</p>');
  }

  return out.join('\n');
}

export function splitBodySections(md) {
  const sections = [];
  const re = /^##\s+(.+)$/gm;
  let match, lastEnd = 0, lastTitle = null;
  while ((match = re.exec(md)) !== null) {
    if (lastTitle !== null) {
      sections.push({ title: lastTitle, content: md.slice(lastEnd, match.index).trim() });
    }
    lastTitle = match[1].trim();
    lastEnd = re.lastIndex;
  }
  if (lastTitle !== null) {
    sections.push({ title: lastTitle, content: md.slice(lastEnd).trim() });
  }
  return sections;
}
