/**
 * Beads Viewer - Static SQL.js WASM-based issue viewer
 *
 * Follows mcp_agent_mail's architecture for client-side sql.js querying with:
 * - OPFS caching for offline support
 * - Chunk reassembly for large databases
 * - FTS5 full-text search
 * - Materialized views for fast queries
 */

// Database state
const DB_STATE = {
  sql: null,          // sql.js library instance
  db: null,           // Database instance
  cacheKey: null,     // OPFS cache key (hash)
  source: 'unknown',  // 'network' | 'cache' | 'chunks'
};

// Graph engine state (WASM)
const GRAPH_STATE = {
  wasm: null,         // WASM module (bv_graph.js)
  graph: null,        // DiGraph instance
  nodeMap: null,      // Map<string, number> - issue ID to node index
  ready: false,       // true when graph is loaded
};

/**
 * Initialize sql.js library
 */
async function initSqlJs() {
  if (DB_STATE.sql) return DB_STATE.sql;

  // Load sql.js from CDN (with WASM)
  const sqlPromise = initSqlJs.cached || (initSqlJs.cached = new Promise(async (resolve, reject) => {
    try {
      let usedLocal = false;
      // Try loading from local vendor first
      let sqlJs;
      try {
        const script = document.createElement('script');
        script.src = './vendor/sql-wasm.js';
        document.head.appendChild(script);
        await new Promise((res, rej) => {
          script.onload = res;
          script.onerror = rej;
        });
        sqlJs = window.initSqlJs;
        usedLocal = true;
      } catch {
        // Fallback to CDN
        const script = document.createElement('script');
        script.src = 'https://cdn.jsdelivr.net/npm/sql.js@1.10.3/dist/sql-wasm.js';
        document.head.appendChild(script);
        await new Promise((res, rej) => {
          script.onload = res;
          script.onerror = rej;
        });
        sqlJs = window.initSqlJs;
        usedLocal = false;
      }

      const SQL = await sqlJs({
        locateFile: file => {
          // Prefer local vendored wasm when available for offline use
          if (usedLocal) {
            return `./vendor/${file}`;
          }
          return `https://cdn.jsdelivr.net/npm/sql.js@1.10.3/dist/${file}`;
        }
      });

      resolve(SQL);
    } catch (err) {
      reject(err);
    }
  }));

  DB_STATE.sql = await sqlPromise;
  return DB_STATE.sql;
}

/**
 * Load database from OPFS cache
 */
async function loadFromOPFS(cacheKey) {
  if (!('storage' in navigator) || !navigator.storage.getDirectory) {
    return null;
  }

  try {
    const root = await navigator.storage.getDirectory();
    const filename = `beads-${cacheKey || 'default'}.sqlite3`;
    const handle = await root.getFileHandle(filename, { create: false });
    const file = await handle.getFile();
    const buffer = await file.arrayBuffer();
    console.log(`[OPFS] Loaded ${buffer.byteLength} bytes from cache`);
    return new Uint8Array(buffer);
  } catch (err) {
    if (err.name !== 'NotFoundError') {
      console.warn('[OPFS] Load failed:', err);
    }
    return null;
  }
}

/**
 * Cache database to OPFS
 */
async function cacheToOPFS(data, cacheKey) {
  if (!('storage' in navigator) || !navigator.storage.getDirectory) {
    return false;
  }

  try {
    const root = await navigator.storage.getDirectory();
    const filename = `beads-${cacheKey || 'default'}.sqlite3`;
    const handle = await root.getFileHandle(filename, { create: true });
    const writable = await handle.createWritable();
    await writable.write(data);
    await writable.close();
    console.log(`[OPFS] Cached ${data.byteLength} bytes`);
    return true;
  } catch (err) {
    console.warn('[OPFS] Cache failed:', err);
    return false;
  }
}

/**
 * Fetch JSON file
 */
async function fetchJSON(url) {
  const response = await fetch(url);
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  return response.json();
}

/**
 * Load database chunks and reassemble
 */
async function loadChunks(config) {
  const chunks = [];
  const totalChunks = config.chunk_count;

  for (let i = 0; i < totalChunks; i++) {
    const chunkPath = `./chunks/${String(i).padStart(5, '0')}.bin`;
    const response = await fetch(chunkPath);
    if (!response.ok) throw new Error(`Failed to load chunk ${i}`);
    const buffer = await response.arrayBuffer();
    chunks.push(new Uint8Array(buffer));
  }

  // Concatenate all chunks
  const totalSize = chunks.reduce((sum, c) => sum + c.length, 0);
  const combined = new Uint8Array(totalSize);
  let offset = 0;
  for (const chunk of chunks) {
    combined.set(chunk, offset);
    offset += chunk.length;
  }

  console.log(`[Chunks] Reassembled ${totalChunks} chunks, ${totalSize} bytes`);
  return combined;
}

/**
 * Load database with caching strategy
 */
async function loadDatabase(updateStatus) {
  const SQL = await initSqlJs();

  updateStatus?.('Checking cache...');

  // Load config to get cache key
  let config = null;
  try {
    config = await fetchJSON('./beads.sqlite3.config.json');
    DB_STATE.cacheKey = config.hash || null;
  } catch {
    // Config file may not exist for small DBs
  }

  // Try OPFS cache first
  if (DB_STATE.cacheKey) {
    const cached = await loadFromOPFS(DB_STATE.cacheKey);
    if (cached) {
      DB_STATE.db = new SQL.Database(cached);
      DB_STATE.source = 'cache';
      return DB_STATE.db;
    }
  }

  updateStatus?.('Loading database...');

  // Check if database is chunked
  let dbData;
  if (config?.chunked) {
    updateStatus?.(`Loading ${config.chunk_count} chunks...`);
    dbData = await loadChunks(config);
    DB_STATE.source = 'chunks';
  } else {
    // Load single file
    const response = await fetch('./beads.sqlite3');
    if (!response.ok) throw new Error(`Database not found: HTTP ${response.status}`);
    const buffer = await response.arrayBuffer();
    dbData = new Uint8Array(buffer);
    DB_STATE.source = 'network';
  }

  DB_STATE.db = new SQL.Database(dbData);

  // Cache for next time
  if (DB_STATE.cacheKey) {
    updateStatus?.('Caching for offline...');
    await cacheToOPFS(DB_STATE.db.export(), DB_STATE.cacheKey);
  }

  return DB_STATE.db;
}

/**
 * Execute a SQL query and return results as array of objects
 */
function execQuery(sql, params = []) {
  if (!DB_STATE.db) throw new Error('Database not loaded');

  try {
    const result = DB_STATE.db.exec(sql, params);
    if (!result.length) return [];

    const { columns, values } = result[0];
    return values.map(row => {
      const obj = {};
      columns.forEach((col, i) => {
        obj[col] = row[i];
      });
      return obj;
    });
  } catch (err) {
    console.error('Query error:', err, sql);
    throw err;
  }
}

/**
 * Get a single value from a query
 */
function execScalar(sql, params = []) {
  const result = execQuery(sql, params);
  if (!result.length) return null;
  return Object.values(result[0])[0];
}

// ============================================================================
// WASM Graph Engine - Live graph calculations
// ============================================================================

/**
 * Initialize the WASM graph engine
 */
async function initGraphEngine() {
  if (GRAPH_STATE.ready) return true;

  try {
    // Dynamic import of the WASM module
    const wasmModule = await import('./vendor/bv_graph.js');
    await wasmModule.default(); // Initialize WASM

    GRAPH_STATE.wasm = wasmModule;
    GRAPH_STATE.graph = new wasmModule.DiGraph();
    GRAPH_STATE.nodeMap = new Map();

    // Load graph data from SQLite
    if (!DB_STATE.db) {
      console.warn('[Graph] Database not loaded yet');
      return false;
    }

    const deps = execQuery(`
      SELECT issue_id, depends_on_id
      FROM dependencies
      WHERE type = 'blocks'
    `);

    for (const row of deps) {
      const from = row.issue_id;
      const to = row.depends_on_id;

      if (!GRAPH_STATE.nodeMap.has(from)) {
        const idx = GRAPH_STATE.graph.addNode(from);
        GRAPH_STATE.nodeMap.set(from, idx);
      }
      if (!GRAPH_STATE.nodeMap.has(to)) {
        const idx = GRAPH_STATE.graph.addNode(to);
        GRAPH_STATE.nodeMap.set(to, idx);
      }

      GRAPH_STATE.graph.addEdge(
        GRAPH_STATE.nodeMap.get(from),
        GRAPH_STATE.nodeMap.get(to)
      );
    }

    GRAPH_STATE.ready = true;
    console.log(`[Graph] Loaded: ${GRAPH_STATE.graph.nodeCount()} nodes, ${GRAPH_STATE.graph.edgeCount()} edges`);
    return true;
  } catch (err) {
    console.warn('[Graph] WASM init failed:', err.message);
    GRAPH_STATE.ready = false;
    return false;
  }
}

/**
 * Build closed set array from database
 * Returns Uint8Array where 1 = closed, 0 = open
 */
function buildClosedSet() {
  if (!GRAPH_STATE.ready) return null;

  const n = GRAPH_STATE.graph.nodeCount();
  const closed = new Uint8Array(n);

  const closedIssues = execQuery(`
    SELECT id FROM issues WHERE status = 'closed'
  `);

  for (const row of closedIssues) {
    const idx = GRAPH_STATE.nodeMap.get(row.id);
    if (idx !== undefined) {
      closed[idx] = 1;
    }
  }

  return closed;
}

/**
 * Recalculate graph metrics for a filtered set of issues
 */
function recalculateMetrics(issueIds) {
  if (!GRAPH_STATE.ready) return null;

  const start = performance.now();
  const indices = issueIds
    .map(id => GRAPH_STATE.nodeMap.get(id))
    .filter(idx => idx !== undefined);

  if (indices.length === 0) return null;

  // Extract subgraph for filtered issues
  const subgraph = GRAPH_STATE.graph.subgraph(new Uint32Array(indices));

  const result = {
    nodeCount: subgraph.nodeCount(),
    edgeCount: subgraph.edgeCount(),
    pagerank: subgraph.pagerankDefault(),
    betweenness: subgraph.betweenness(),
    hasCycles: subgraph.hasCycles(),
    criticalPath: subgraph.criticalPathHeights(),
  };

  const elapsed = performance.now() - start;
  console.log(`[Graph] Recalculated metrics in ${elapsed.toFixed(1)}ms`);

  return result;
}

/**
 * What-if analysis: compute cascade impact of closing an issue
 */
function whatIfClose(issueId) {
  if (!GRAPH_STATE.ready) return null;

  const idx = GRAPH_STATE.nodeMap.get(issueId);
  if (idx === undefined) return null;

  const closedSet = buildClosedSet();
  const result = GRAPH_STATE.graph.whatIfClose(idx, closedSet);

  // Convert node indices back to issue IDs
  if (result && result.cascade_ids) {
    result.cascade_issue_ids = result.cascade_ids
      .map(i => GRAPH_STATE.graph.nodeId(i))
      .filter(Boolean);
  }

  return result;
}

/**
 * Get top issues by cascade impact
 */
function topWhatIf(limit = 10) {
  if (!GRAPH_STATE.ready) return [];

  const closedSet = buildClosedSet();
  const results = GRAPH_STATE.graph.topWhatIf(closedSet, limit);

  // Enrich with issue IDs
  return (results || []).map(item => ({
    ...item,
    issueId: GRAPH_STATE.graph.nodeId(item.node),
    result: item.result,
  }));
}

/**
 * Get actionable issues (all blockers closed)
 */
function getActionableIssues() {
  if (!GRAPH_STATE.ready) return [];

  const closedSet = buildClosedSet();
  const indices = GRAPH_STATE.graph.actionableNodes(closedSet);

  return (indices || [])
    .map(idx => GRAPH_STATE.graph.nodeId(idx))
    .filter(Boolean);
}

/**
 * Find cycle break suggestions
 */
function getCycleBreakSuggestions(limit = 5) {
  if (!GRAPH_STATE.ready) return null;

  const result = GRAPH_STATE.graph.cycleBreakSuggestions(limit, 100);
  return result;
}

/**
 * Get greedy top-k issues to maximize unblocks
 */
function getTopKSet(k = 5) {
  if (!GRAPH_STATE.ready) return null;

  const closedSet = buildClosedSet();
  const result = GRAPH_STATE.graph.topkSet(closedSet, k);

  // Enrich with issue IDs
  if (result && result.items) {
    result.items = result.items.map(item => ({
      ...item,
      issueId: GRAPH_STATE.graph.nodeId(item.node),
      unblocked_issue_ids: (item.unblocked_ids || [])
        .map(i => GRAPH_STATE.graph.nodeId(i))
        .filter(Boolean),
    }));
  }

  return result;
}

// ============================================================================
// Query Layer - Using materialized views for performance
// ============================================================================

/**
 * Query issues with filters, sorting, and pagination
 */
function queryIssues(filters = {}, sort = 'priority', limit = 50, offset = 0) {
  let sql = `SELECT * FROM issue_overview_mv WHERE 1=1`;
  const params = [];

  if (filters.status) {
    sql += ` AND status = ?`;
    params.push(filters.status);
  }

  if (filters.type) {
    sql += ` AND issue_type = ?`;
    params.push(filters.type);
  }

  if (filters.priority !== undefined && filters.priority !== '') {
    sql += ` AND priority = ?`;
    params.push(parseInt(filters.priority));
  }

  if (filters.search) {
    // Try FTS search first, fallback to LIKE
    sql += ` AND (title LIKE ? OR description LIKE ? OR id LIKE ?)`;
    const searchTerm = `%${filters.search}%`;
    params.push(searchTerm, searchTerm, searchTerm);
  }

  if (filters.labels?.length) {
    sql += ` AND (${filters.labels.map(() => `labels LIKE ?`).join(' OR ')})`;
    params.push(...filters.labels.map(l => `%"${l}"%`));
  }

  // Sorting
  const sortMap = {
    'priority': 'priority ASC, triage_score DESC',
    'updated': 'updated_at DESC',
    'score': 'triage_score DESC',
    'blocks': 'blocks_count DESC',
    'created': 'created_at DESC',
  };
  sql += ` ORDER BY ${sortMap[sort] || sortMap.priority}`;
  sql += ` LIMIT ? OFFSET ?`;
  params.push(limit, offset);

  return execQuery(sql, params);
}

/**
 * Count issues matching filters
 */
function countIssues(filters = {}) {
  let sql = `SELECT COUNT(*) as count FROM issue_overview_mv WHERE 1=1`;
  const params = [];

  if (filters.status) {
    sql += ` AND status = ?`;
    params.push(filters.status);
  }

  if (filters.type) {
    sql += ` AND issue_type = ?`;
    params.push(filters.type);
  }

  if (filters.priority !== undefined && filters.priority !== '') {
    sql += ` AND priority = ?`;
    params.push(parseInt(filters.priority));
  }

  if (filters.search) {
    sql += ` AND (title LIKE ? OR description LIKE ? OR id LIKE ?)`;
    const searchTerm = `%${filters.search}%`;
    params.push(searchTerm, searchTerm, searchTerm);
  }

  return execScalar(sql, params) || 0;
}

/**
 * Get a single issue by ID
 */
function getIssue(id) {
  const results = execQuery(`SELECT * FROM issue_overview_mv WHERE id = ?`, [id]);
  return results[0] || null;
}

/**
 * Full-text search using FTS5 (if available)
 */
function searchIssues(term, limit = 50) {
  // Try FTS5 first
  try {
    const sql = `
      SELECT id, title,
             snippet(issues_fts, 2, '<mark>', '</mark>', '...', 32) as snippet,
             bm25(issues_fts) as rank
      FROM issues_fts
      WHERE issues_fts MATCH ?
      ORDER BY rank
      LIMIT ?
    `;
    return execQuery(sql, [term + '*', limit]);
  } catch {
    // Fallback to LIKE search
    return queryIssues({ search: term }, 'score', limit, 0);
  }
}

/**
 * Get project statistics
 */
function getStats() {
  const stats = {};

  // Count by status
  const statusCounts = execQuery(`
    SELECT status, COUNT(*) as count
    FROM issue_overview_mv
    GROUP BY status
  `);
  statusCounts.forEach(row => {
    stats[row.status] = row.count;
  });

  // Count blocked (has blocked_by_ids and status is open/in_progress)
  stats.blocked = execScalar(`
    SELECT COUNT(*) FROM issue_overview_mv
    WHERE blocked_by_ids IS NOT NULL
    AND blocked_by_ids != ''
    AND status IN ('open', 'in_progress')
  `) || 0;

  // Total
  stats.total = execScalar(`SELECT COUNT(*) FROM issue_overview_mv`) || 0;

  return stats;
}

/**
 * Get top issues by triage score
 */
function getTopPicks(limit = 5) {
  return execQuery(`
    SELECT * FROM issue_overview_mv
    WHERE status IN ('open', 'in_progress')
    ORDER BY triage_score DESC
    LIMIT ?
  `, [limit]);
}

/**
 * Get recent issues by update time
 */
function getRecentIssues(limit = 10) {
  return execQuery(`
    SELECT * FROM issue_overview_mv
    ORDER BY updated_at DESC
    LIMIT ?
  `, [limit]);
}

/**
 * Get top issues by PageRank
 */
function getTopByPageRank(limit = 10) {
  return execQuery(`
    SELECT * FROM issue_overview_mv
    WHERE pagerank > 0
    ORDER BY pagerank DESC
    LIMIT ?
  `, [limit]);
}

/**
 * Get top issues by triage score
 */
function getTopByTriageScore(limit = 10) {
  return execQuery(`
    SELECT * FROM issue_overview_mv
    WHERE triage_score > 0
    ORDER BY triage_score DESC
    LIMIT ?
  `, [limit]);
}

/**
 * Get top blocking issues
 */
function getTopBlockers(limit = 10) {
  return execQuery(`
    SELECT * FROM issue_overview_mv
    WHERE blocks_count > 0
    ORDER BY blocks_count DESC
    LIMIT ?
  `, [limit]);
}

/**
 * Get export metadata
 */
function getMeta() {
  const meta = {};
  const rows = execQuery(`SELECT key, value FROM export_meta`);
  rows.forEach(row => {
    meta[row.key] = row.value;
  });
  return meta;
}

/**
 * Get dependencies for an issue
 */
function getIssueDependencies(id) {
  const blocks = execQuery(`
    SELECT i.* FROM issue_overview_mv i
    JOIN dependencies d ON i.id = d.depends_on_id
    WHERE d.issue_id = ? AND d.type = 'blocks'
  `, [id]);

  const blockedBy = execQuery(`
    SELECT i.* FROM issue_overview_mv i
    JOIN dependencies d ON i.id = d.issue_id
    WHERE d.depends_on_id = ? AND d.type = 'blocks'
  `, [id]);

  return { blocks, blockedBy };
}

// ============================================================================
// Alpine.js Application
// ============================================================================

/**
 * Format ISO date to readable string
 */
function formatDate(isoString) {
  if (!isoString) return '';
  try {
    const date = new Date(isoString);
    return date.toLocaleDateString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return isoString;
  }
}

/**
 * Render markdown safely
 */
function renderMarkdown(text) {
  if (!text) return '';
  try {
    const html = marked.parse(text);
    return DOMPurify.sanitize(html);
  } catch {
    return DOMPurify.sanitize(text);
  }
}

/**
 * Main Alpine.js application component
 */
function beadsApp() {
  return {
    // State
    loading: true,
    loadingMessage: 'Initializing...',
    error: null,
    view: 'dashboard',
    darkMode: localStorage.getItem('darkMode') === 'true',

    // Data
    stats: {},
    meta: {},
    dbSource: 'loading',

    // Issues list
    issues: [],
    totalIssues: 0,
    page: 1,
    pageSize: 20,

    // Filters
    filters: {
      status: '',
      type: '',
      priority: '',
    },
    sort: 'priority',
    searchQuery: '',

    // Dashboard data
    topPicks: [],
    recentIssues: [],
    topByPageRank: [],
    topByTriageScore: [],
    topBlockers: [],

    // Selected issue
    selectedIssue: null,

    // Graph engine state
    graphReady: false,
    graphMetrics: null,
    whatIfResult: null,
    topKSet: null,

    /**
     * Initialize the application
     */
    async init() {
      // Apply dark mode
      if (this.darkMode) {
        document.documentElement.classList.add('dark');
      }

      try {
        this.loadingMessage = 'Loading sql.js...';
        await loadDatabase((msg) => {
          this.loadingMessage = msg;
        });

        this.dbSource = DB_STATE.source;
        this.loadingMessage = 'Loading data...';

        // Load initial data
        this.meta = getMeta();
        this.stats = getStats();
        this.topPicks = getTopPicks(5);
        this.recentIssues = getRecentIssues(10);
        this.topByPageRank = getTopByPageRank(10);
        this.topByTriageScore = getTopByTriageScore(10);
        this.topBlockers = getTopBlockers(10);

        // Load issues for list view
        this.loadIssues();

        // Initialize WASM graph engine (non-blocking)
        this.loadingMessage = 'Loading graph engine...';
        this.graphReady = await initGraphEngine();
        if (this.graphReady) {
          this.topKSet = getTopKSet(5);
        }

        this.loading = false;
      } catch (err) {
        console.error('Init failed:', err);
        this.error = err.message || 'Failed to load database';
        this.loading = false;
      }
    },

    /**
     * Load issues based on current filters
     */
    loadIssues() {
      const offset = (this.page - 1) * this.pageSize;
      const filters = {
        ...this.filters,
        search: this.searchQuery,
      };

      this.issues = queryIssues(filters, this.sort, this.pageSize, offset);
      this.totalIssues = countIssues(filters);
    },

    /**
     * Search issues
     */
    search() {
      this.page = 1;
      this.loadIssues();
    },

    /**
     * Pagination
     */
    nextPage() {
      if (this.page * this.pageSize < this.totalIssues) {
        this.page++;
        this.loadIssues();
      }
    },

    prevPage() {
      if (this.page > 1) {
        this.page--;
        this.loadIssues();
      }
    },

    /**
     * Show issue detail
     */
    showIssue(id) {
      this.selectedIssue = getIssue(id);
    },

    /**
     * Toggle dark mode
     */
    toggleDarkMode() {
      this.darkMode = !this.darkMode;
      localStorage.setItem('darkMode', this.darkMode);
      document.documentElement.classList.toggle('dark', this.darkMode);
    },

    // ========================================================================
    // Graph Engine Methods
    // ========================================================================

    /**
     * Recalculate graph metrics for currently filtered issues
     */
    recalculateForFilter() {
      if (!this.graphReady) return;
      const ids = this.issues.map(i => i.id);
      this.graphMetrics = recalculateMetrics(ids);
    },

    /**
     * Compute what-if cascade impact for an issue
     */
    computeWhatIf(issueId) {
      if (!this.graphReady) return;
      this.whatIfResult = whatIfClose(issueId);
    },

    /**
     * Clear what-if result
     */
    clearWhatIf() {
      this.whatIfResult = null;
    },

    /**
     * Refresh top-k set
     */
    refreshTopKSet() {
      if (!this.graphReady) return;
      this.topKSet = getTopKSet(5);
    },

    /**
     * Get top issues by cascade impact
     */
    getTopImpact(limit = 10) {
      if (!this.graphReady) return [];
      return topWhatIf(limit);
    },

    /**
     * Get cycle break suggestions
     */
    getCycleBreaks(limit = 5) {
      if (!this.graphReady) return null;
      return getCycleBreakSuggestions(limit);
    },

    /**
     * Format date helper
     */
    formatDate,

    /**
     * Render markdown helper
     */
    renderMarkdown,
  };
}

// Export for use in graph integration
window.beadsViewer = {
  // Database
  DB_STATE,
  loadDatabase,
  execQuery,
  queryIssues,
  getIssue,
  getIssueDependencies,
  getStats,
  getMeta,

  // Graph Engine
  GRAPH_STATE,
  initGraphEngine,
  buildClosedSet,
  recalculateMetrics,
  whatIfClose,
  topWhatIf,
  getActionableIssues,
  getCycleBreakSuggestions,
  getTopKSet,
};
