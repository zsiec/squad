// names.js — client-side pretty-name lookup for opaque agent ids.
// Deterministic: same id always yields the same first+last. Pass-through
// for fallbacks that look human-authored (anything not matching agent-XXXX).

const FIRST = [
  'Jordan', 'Alex', 'Taylor', 'Morgan', 'Riley', 'Casey', 'Avery', 'Quinn',
  'Cameron', 'Jamie', 'Drew', 'Sam', 'Devon', 'Reese', 'Parker', 'Rowan',
  'Priya', 'Ravi', 'Anika', 'Kiran', 'Rohan', 'Meera', 'Arjun', 'Divya',
  'Wei', 'Ling', 'Chen', 'Hiro', 'Yuki', 'Min', 'Jin', 'Mei',
  'Carlos', 'Sofia', 'Diego', 'Lucia', 'Mateo', 'Elena', 'Javier', 'Camila',
  'Andre', 'Maya', 'Malik', 'Zara', 'Tariq', 'Aisha', 'Jamal', 'Nia',
  'Mike', 'Sarah', 'Dan', 'Emma', 'Chris', 'Kate', 'Ben', 'Lily',
  'Tyler', 'Grace', 'Nate', 'Ivy',
];

const LAST = [
  'Chen', 'Patel', 'Rodriguez', 'Johnson', 'Kim', 'Nguyen', 'Garcia', 'Park',
  'Singh', 'Williams', 'Anderson', 'Martinez', 'Thompson', 'Walker', 'Hall', 'Young',
  'Shah', 'Gupta', 'Agarwal', 'Iyer', 'Kumar', 'Reddy', 'Joshi', 'Khan',
  'Wong', 'Liu', 'Zhang', 'Wang', 'Tanaka', 'Sato', 'Suzuki', 'Nakamura',
  'Lopez', 'Hernandez', 'Gonzalez', 'Ramirez', 'Torres', 'Flores', 'Rivera', 'Diaz',
  'Jackson', 'Brooks', 'Washington', 'Carter', 'Robinson', 'Harris', 'Lewis', 'Bell',
  'Ahmed', 'Hassan', 'Ibrahim', 'Mohammed', 'Abbas', 'Farouk',
  'Smith', 'Miller', 'Davis', 'Wilson', 'Moore', 'Taylor',
];

// Two independent FNV-1a 32-bit hashes with different seeds so first/last
// are chosen independently — otherwise Jordan is always paired with Chen.
function hashA(s) {
  let h = 2166136261 >>> 0;
  for (let i = 0; i < s.length; i++) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 16777619);
  }
  return h >>> 0;
}

function hashB(s) {
  let h = 0x811c9dc5 ^ 0x5a5a5a5a;
  for (let i = s.length - 1; i >= 0; i--) {
    h ^= s.charCodeAt(i);
    h = Math.imul(h, 0x01000193);
  }
  return h >>> 0;
}

const AGENT_ID_RE = /^agent-[a-f0-9]+$/i;

function looksHumanAuthored(s) {
  if (!s) return false;
  if (AGENT_ID_RE.test(s)) return false;
  return true;
}

export function displayName(id, fallback) {
  if (looksHumanAuthored(fallback)) return fallback;
  const key = String(id || fallback || '');
  if (!key) return fallback || 'anon';
  const first = FIRST[hashA(key) % FIRST.length];
  const last = LAST[hashB(key) % LAST.length];
  return `${first} ${last}`;
}
