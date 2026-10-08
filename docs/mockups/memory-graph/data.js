// Snapshot of the Memory POC workspace: 13 ADR Memories, 20 Issues copied from
// the b9s database, and the 28 informational Links written for the citation
// experiment. Every mockup reads this file, so they all show the same graph.

const MEMORIES = [
  { id: 'adr-0004', n: '0004', status: 'active', date: '2026-09-11', title: 'Limit plain fuzzy search to primary fields' },
  { id: 'adr-0007', n: '0007', status: 'active', date: '2026-09-13', title: 'Read shared Beads through one SELECT-only catalog account' },
  { id: 'adr-0009', n: '0009', status: 'superseded', date: '2026-09-14', title: "Open entity views from a ':' command prompt backed by a closed alias table" },
  { id: 'adr-0010', n: '0010', status: 'active', date: '2026-09-14', title: 'Take the tree sort from the user config, never from project state' },
  { id: 'adr-0011', n: '0011', status: 'active', date: '2026-09-14', title: 'Bulk actions act on the marked set, otherwise on the cursor row' },
  { id: 'adr-0014', n: '0014', status: 'active', date: '2026-09-23', title: 'Map actors to identities through a b9s-owned alias list in bd config' },
  { id: 'adr-0018', n: '0018', status: 'active', date: '2026-09-25', title: 'Show epics as a rail or as rows, and hide the closed column' },
  { id: 'adr-0019', n: '0019', status: 'active', date: '2026-09-26', title: 'Serve a mobile web UI from b9s web with an embedded SPA' },
  { id: 'adr-0020', n: '0020', status: 'active', date: '2026-09-26', title: 'Pair every b9s web browser with a token that persists until renewed' },
  { id: 'adr-0025', n: '0025', status: 'active', date: '2026-09-27', title: 'Read embedded Dolt through bd export, never by opening the store' },
  { id: 'adr-0026', n: '0026', status: 'proposed', date: '2026-09-28', title: 'Model a release as a milestone issue that depends on its epics' },
  { id: 'adr-0027', n: '0027', status: 'active', date: '2026-09-28', title: 'Show a flat list for type queries and on t' },
  { id: 'adr-0029', n: '0029', status: 'proposed', date: '2026-09-28', title: 'Serve a public, writable demo from b9s web --public' },
];

const ISSUES = [
  { id: 'vd9g', type: 'epic', status: 'open', parent: null, title: 'v1.4 release' },
  { id: 'vd9g.1', type: 'epic', status: 'open', parent: 'vd9g', title: 'BD Events' },
  { id: 'vd9g.1.6', type: 'feature', status: 'open', parent: 'vd9g.1', title: 'Add the :activity view to the TUI' },
  { id: 'hffz', type: 'epic', status: 'open', parent: null, title: 'v1.3 release: web UI in the binary and embedded Dolt' },
  { id: 'hffz.16', type: 'feature', status: 'open', parent: 'hffz', title: "Offer close anyway when bd refuses another user's issue" },
  { id: 'hffz.18', type: 'feature', status: 'open', parent: 'hffz', title: 'Public demo of b9s web' },
  { id: 'xx9w', type: 'epic', status: 'open', parent: null, title: 'Releases as a first-class entity' },
  { id: 'xx9w.1', type: 'feature', status: 'open', parent: 'xx9w', title: 'Read releases into b9s' },
  { id: 'xx9w.2', type: 'feature', status: 'open', parent: 'xx9w', title: 'Show releases in the TUI' },
  { id: 'xx9w.3', type: 'feature', status: 'open', parent: 'xx9w', title: 'Plan releases from b9s' },
  { id: 'xx9w.4', type: 'feature', status: 'open', parent: 'xx9w', title: 'Show releases in the web UI' },
  { id: 'v5q1', type: 'feature', status: 'open', parent: null, title: 'Cycle search history with Up/Down in the / search bar' },
  { id: 'rcsh', type: 'feature', status: 'open', parent: null, title: 'Remember the :layout choice across restarts' },
  { id: 'bjqc.4', type: 'feature', status: 'open', parent: null, title: 'Bulk priority and label edits over marked issues' },
  { id: 'n0wb', type: 'feature', status: 'in_progress', parent: null, title: 'Migrate from Go flag to cobra/pflag for double-dash options' },
  { id: '36s', type: 'feature', status: 'in_progress', parent: null, title: 'Agenda view: time-horizon grouping' },
  { id: 'e5u3', type: 'epic', status: 'open', parent: null, title: 'Redesign the board TUI' },
  { id: 't8j5', type: 'epic', status: 'open', parent: null, title: 'Add attachments to beads' },
  { id: 'db5q', type: 'epic', status: 'open', parent: null, title: 'Visualize Memory Beads in b9s and evaluate preview' },
  { id: 'gmaz', type: 'epic', status: 'open', parent: null, title: 'Improve README and prune stale docs' },
];

// follows: the Issue applies this decision. cites: the Issue refers to it.
// supersedes: one Memory replaces another. The preview stores only a note on
// an edge; the version an agent read is not recorded.
const EDGES = [
  { from: 'hffz.18', to: 'adr-0029', kind: 'follows' },
  { from: 'hffz.18', to: 'adr-0019', kind: 'cites' },
  { from: 'hffz.18', to: 'adr-0020', kind: 'cites' },
  { from: 'vd9g.1.6', to: 'adr-0009', kind: 'follows', note: 'superseded by 0027' },
  { from: 'vd9g.1.6', to: 'adr-0025', kind: 'cites' },
  { from: 'vd9g.1.6', to: 'adr-0007', kind: 'cites' },
  { from: 'v5q1', to: 'adr-0010', kind: 'follows' },
  { from: 'v5q1', to: 'adr-0004', kind: 'cites' },
  { from: 'xx9w.1', to: 'adr-0026', kind: 'follows' },
  { from: 'xx9w.1', to: 'adr-0025', kind: 'follows' },
  { from: 'xx9w.2', to: 'adr-0026', kind: 'follows' },
  { from: 'xx9w.2', to: 'adr-0027', kind: 'cites' },
  { from: 'xx9w.3', to: 'adr-0026', kind: 'follows' },
  { from: 'xx9w.4', to: 'adr-0026', kind: 'follows' },
  { from: 'xx9w.4', to: 'adr-0019', kind: 'cites' },
  { from: 'hffz.16', to: 'adr-0014', kind: 'cites' },
  { from: 'hffz.16', to: 'adr-0011', kind: 'cites' },
  { from: 'rcsh', to: 'adr-0010', kind: 'follows' },
  { from: 'rcsh', to: 'adr-0018', kind: 'cites' },
  { from: 'bjqc.4', to: 'adr-0011', kind: 'follows' },
  { from: 'adr-0027', to: 'adr-0009', kind: 'supersedes' },
];

const G = (() => {
  const nodes = new Map();
  MEMORIES.forEach(m => nodes.set(m.id, { ...m, kind: 'memory' }));
  ISSUES.forEach(i => nodes.set(i.id, { ...i, kind: 'issue' }));
  const edges = EDGES.slice();
  ISSUES.forEach(i => { if (i.parent) edges.push({ from: i.id, to: i.parent, kind: 'child-of' }); });

  const out = id => edges.filter(e => e.from === id);
  const inc = id => edges.filter(e => e.to === id);
  const neighbours = id => [
    ...out(id).map(e => ({ edge: e, other: e.to, dir: 'out' })),
    ...inc(id).map(e => ({ edge: e, other: e.from, dir: 'in' })),
  ];
  const decisionEdges = id => out(id).filter(e => e.kind === 'follows' || e.kind === 'cites');

  // A finite verdict per Issue, so every mockup flags the same problems.
  function health(id) {
    const n = nodes.get(id);
    if (!n || n.kind !== 'issue') return null;
    const d = decisionEdges(id);
    if (d.some(e => e.kind === 'follows' && nodes.get(e.to).status === 'superseded')) return 'dead';
    if (n.type === 'epic') return 'epic';
    if (d.length === 0) return 'none';
    if (d.some(e => e.kind === 'follows' && nodes.get(e.to).status === 'proposed')) return 'proposed';
    return 'ok';
  }
  const HEALTH_TEXT = {
    dead: 'Follows a superseded decision',
    none: 'No decision recorded',
    proposed: 'Follows a decision that is still proposed',
    ok: 'Follows active decisions',
    epic: 'Epic',
  };
  const label = id => { const n = nodes.get(id); return n.kind === 'memory' ? 'ADR ' + n.n : id; };
  const followers = id => inc(id).filter(e => e.kind === 'follows').map(e => e.from);
  const citers = id => inc(id).filter(e => e.kind === 'cites').map(e => e.from);

  return { nodes, edges, out, inc, neighbours, health, HEALTH_TEXT, label, followers, citers, decisionEdges };
})();

const EDGE_TEXT = {
  out: { follows: 'Follows', cites: 'Cites', supersedes: 'Supersedes', 'child-of': 'Part of' },
  in: { follows: 'Followed by', cites: 'Cited by', supersedes: 'Superseded by', 'child-of': 'Contains' },
};

function esc(s) {
  return String(s).replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' }[c]));
}

// The side panel every mockup shares: one node and its typed neighbours.
function detailHTML(id) {
  const n = G.nodes.get(id);
  const h = G.health(id);
  let html = `<div class="kicker">${n.kind === 'memory' ? 'Memory' : n.type === 'epic' ? 'Epic' : 'Issue'} · ${esc(G.label(id))}</div>
    <h2>${esc(n.title)}</h2>
    <div class="meta"><span class="pill ${n.kind === 'memory' ? n.status : 'issue'}">${esc(n.status.replace('_', ' '))}</span>
    ${n.date ? `<span class="muted">${n.date}</span>` : ''}</div>`;
  if (h && h !== 'ok' && h !== 'epic') html += `<div class="callout ${h}">${G.HEALTH_TEXT[h]}</div>`;
  if (n.kind === 'memory' && n.status === 'superseded') {
    const by = G.inc(id).find(e => e.kind === 'supersedes');
    const open = G.followers(id);
    html += `<div class="callout dead">Replaced by ${by ? G.label(by.from) : 'a later decision'}.${open.length ? ` ${open.length} open Issue${open.length > 1 ? 's' : ''} still follow${open.length > 1 ? '' : 's'} it.` : ''}</div>`;
  }
  const groups = {};
  G.neighbours(id).forEach(x => {
    const key = EDGE_TEXT[x.dir][x.edge.kind];
    (groups[key] = groups[key] || []).push(x);
  });
  const order = ['Follows', 'Cites', 'Followed by', 'Cited by', 'Supersedes', 'Superseded by', 'Part of', 'Contains'];
  order.filter(k => groups[k]).forEach(k => {
    html += `<h3>${k} <span class="muted">${groups[k].length}</span></h3><ul class="nb">`;
    groups[k].forEach(x => {
      const o = G.nodes.get(x.other);
      const tag = o.kind === 'memory' ? `<span class="dot ${o.status}"></span>` : `<span class="dot h-${G.health(o.id) || 'ok'}"></span>`;
      html += `<li data-id="${o.id}">${tag}<b>${esc(G.label(o.id))}</b> ${esc(o.title)}${x.edge.note ? `<div class="note">note: ${esc(x.edge.note)}</div>` : ''}</li>`;
    });
    html += '</ul>';
  });
  if (G.neighbours(id).length === 0) html += '<p class="muted">No Links. Nothing records which decisions apply here.</p>';
  if (n.kind === 'issue' && G.decisionEdges(id).length) html += '<p class="fine">The preview does not record which version of a Memory the Issue read.</p>';
  return html;
}

const MOCKUPS = [
  ['1-constellation.html', 'Constellation'],
  ['2-two-shores.html', 'Two shores'],
  ['3-focus.html', 'Focus'],
  ['4-metro.html', 'Decision metro'],
  ['5-terminal.html', 'Terminal'],
];

function renderNav(current) {
  const nav = document.querySelector('nav.top');
  nav.innerHTML = `<a href="index.html" class="home">Memory graph mockups</a>` +
    MOCKUPS.map(([href, name], i) => `<a href="${href}" class="${href === current ? 'on' : ''}">${i + 1}. ${name}</a>`).join('');
}
