package export

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Dicklesworthstone/beads_viewer/pkg/analysis"
	"github.com/Dicklesworthstone/beads_viewer/pkg/model"
)

// InteractiveGraphOptions configures HTML graph generation
type InteractiveGraphOptions struct {
	Issues      []model.Issue
	Stats       *analysis.GraphStats
	Title       string
	DataHash    string
	Path        string // Output path - if empty, auto-generates based on project
	ProjectName string // Project name for auto-naming
}

// graphNode represents a node in the interactive graph
type graphNode struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Status          string   `json:"status"`
	Priority        int      `json:"priority"`
	Type            string   `json:"type"`
	Labels          []string `json:"labels"`
	PageRank        float64  `json:"pagerank"`
	Betweenness     float64  `json:"betweenness"`
	Eigenvector     float64  `json:"eigenvector"`
	Hub             float64  `json:"hub"`
	Authority       float64  `json:"authority"`
	CriticalPath    float64  `json:"critical_path"`
	InDegree        int      `json:"in_degree"`
	OutDegree       int      `json:"out_degree"`
	CoreNumber      int      `json:"core_number"`
	Slack           float64  `json:"slack"`
	IsArticulation  bool     `json:"is_articulation"`
	PageRankRank    int      `json:"pagerank_rank"`
	BetweennessRank int      `json:"betweenness_rank"`
}

// graphLink represents an edge in the interactive graph
type graphLink struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Type     string `json:"type"`
	Critical bool   `json:"critical"`
}

// GenerateInteractiveGraphFilename creates an auto-generated filename
// Format: {project}_{YYYYMMDD}_{HHMMSS}_{gitshort}.html
func GenerateInteractiveGraphFilename(projectName string) string {
	now := time.Now()
	dateStr := now.Format("20060102_150405")

	// Get short git commit hash
	gitShort := "nogit"
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	if output, err := cmd.Output(); err == nil {
		gitShort = strings.TrimSpace(string(output))
	}

	// Clean project name
	safeName := strings.ReplaceAll(projectName, " ", "_")
	safeName = strings.ReplaceAll(safeName, "/", "_")

	return fmt.Sprintf("%s_%s_%s.html", safeName, dateStr, gitShort)
}

// GenerateInteractiveGraphHTML creates a self-contained HTML file with force-graph visualization
func GenerateInteractiveGraphHTML(opts InteractiveGraphOptions) (string, error) {
	if len(opts.Issues) == 0 {
		return "", fmt.Errorf("no issues to export")
	}

	// Build graph data with all metrics
	nodes := make([]graphNode, 0, len(opts.Issues))
	links := make([]graphLink, 0)

	// Create issue map for dependency lookup
	issueMap := make(map[string]bool)
	for _, iss := range opts.Issues {
		issueMap[iss.ID] = true
	}

	// Get all metrics if available
	var pageRank, betweenness, eigenvector, hubs, authorities, criticalPath, slack map[string]float64
	var coreNumber map[string]int
	var articulation []string
	var pageRankRank, betweennessRank map[string]int
	var inDegree, outDegree map[string]int

	if opts.Stats != nil {
		pageRank = opts.Stats.PageRank()
		betweenness = opts.Stats.Betweenness()
		eigenvector = opts.Stats.Eigenvector()
		hubs = opts.Stats.Hubs()
		authorities = opts.Stats.Authorities()
		criticalPath = opts.Stats.CriticalPathScore()
		slack = opts.Stats.Slack()
		coreNumber = opts.Stats.CoreNumber()
		articulation = opts.Stats.ArticulationPoints()
		pageRankRank = opts.Stats.PageRankRank()
		betweennessRank = opts.Stats.BetweennessRank()
		inDegree = opts.Stats.InDegree
		outDegree = opts.Stats.OutDegree
	}

	// Create articulation set for O(1) lookup
	articulationSet := make(map[string]bool)
	for _, id := range articulation {
		articulationSet[id] = true
	}

	// Build nodes
	for _, iss := range opts.Issues {
		node := graphNode{
			ID:              iss.ID,
			Title:           iss.Title,
			Status:          string(iss.Status),
			Priority:        iss.Priority,
			Type:            string(iss.IssueType),
			Labels:          iss.Labels,
			PageRank:        pageRank[iss.ID],
			Betweenness:     betweenness[iss.ID],
			Eigenvector:     eigenvector[iss.ID],
			Hub:             hubs[iss.ID],
			Authority:       authorities[iss.ID],
			CriticalPath:    criticalPath[iss.ID],
			InDegree:        inDegree[iss.ID],
			OutDegree:       outDegree[iss.ID],
			CoreNumber:      coreNumber[iss.ID],
			Slack:           slack[iss.ID],
			IsArticulation:  articulationSet[iss.ID],
			PageRankRank:    pageRankRank[iss.ID],
			BetweennessRank: betweennessRank[iss.ID],
		}
		nodes = append(nodes, node)

		// Build links from dependencies
		for _, dep := range iss.Dependencies {
			if dep == nil || !issueMap[dep.DependsOnID] {
				continue
			}
			isCritical := slack[iss.ID] == 0 && slack[dep.DependsOnID] == 0
			link := graphLink{
				Source:   iss.ID,
				Target:   dep.DependsOnID,
				Type:     string(dep.Type),
				Critical: isCritical,
			}
			links = append(links, link)
		}
	}

	// Sort nodes by ID for determinism
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	graphData := map[string]interface{}{
		"nodes": nodes,
		"links": links,
	}

	dataJSON, err := json.Marshal(graphData)
	if err != nil {
		return "", fmt.Errorf("marshal graph data: %w", err)
	}

	title := opts.Title
	if title == "" {
		title = "Dependency Graph"
	}

	// Generate filename if not provided
	outputPath := opts.Path
	if outputPath == "" {
		projectName := opts.ProjectName
		if projectName == "" {
			projectName = "graph"
		}
		outputPath = GenerateInteractiveGraphFilename(projectName)
	}

	// Ensure .html extension
	if !strings.HasSuffix(strings.ToLower(outputPath), ".html") {
		outputPath = strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".html"
	}

	html := generateUltimateHTML(title, opts.DataHash, string(dataJSON), len(nodes), len(links), opts.ProjectName)

	// Ensure directory exists
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", fmt.Errorf("create dir: %w", err)
		}
	}

	if err := os.WriteFile(outputPath, []byte(html), 0644); err != nil {
		return "", err
	}

	return outputPath, nil
}

func generateUltimateHTML(title, dataHash, graphDataJSON string, nodeCount, edgeCount int, projectName string) string {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s | bv Graph</title>
    <link href="https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;500;600;700&display=swap" rel="stylesheet">
    <style>
        :root {
            --bg: #282a36;
            --bg-secondary: #44475a;
            --bg-tertiary: #21222c;
            --bg-elevated: #373a4f;
            --fg: #f8f8f2;
            --fg-muted: #6272a4;
            --purple: #bd93f9;
            --pink: #ff79c6;
            --cyan: #8be9fd;
            --green: #50fa7b;
            --orange: #ffb86c;
            --red: #ff5555;
            --yellow: #f1fa8c;
            --shadow: 0 4px 20px rgba(0,0,0,0.5);
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: 'JetBrains Mono', monospace;
            background: var(--bg);
            color: var(--fg);
            height: 100vh;
            display: flex;
            flex-direction: column;
            overflow: hidden;
        }
        header {
            background: linear-gradient(135deg, var(--bg-tertiary), var(--bg-secondary));
            padding: 0.6rem 1.25rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
            border-bottom: 2px solid var(--purple);
            z-index: 100;
            box-shadow: var(--shadow);
        }
        .logo { display: flex; align-items: center; gap: 0.6rem; }
        .logo-icon {
            width: 32px; height: 32px;
            background: linear-gradient(135deg, var(--purple), var(--pink));
            border-radius: 8px;
            display: flex; align-items: center; justify-content: center;
            font-weight: 700; font-size: 14px;
            box-shadow: 0 2px 8px rgba(189,147,249,0.4);
        }
        h1 { font-size: 1.1rem; font-weight: 600; }
        h1 span { background: linear-gradient(90deg, var(--purple), var(--pink)); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
        .toolbar { display: flex; gap: 0.6rem; align-items: center; flex-wrap: wrap; }
        .toolbar-group {
            display: flex; gap: 0.25rem;
            padding: 0.25rem;
            background: var(--bg);
            border-radius: 8px;
            border: 1px solid var(--bg-secondary);
        }
        button, select {
            font-family: inherit;
            font-size: 0.7rem;
            padding: 0.4rem 0.7rem;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            transition: all 0.15s ease;
        }
        button { background: transparent; color: var(--fg-muted); }
        button:hover { background: var(--bg-elevated); color: var(--fg); }
        button.active { background: linear-gradient(135deg, var(--purple), var(--pink)); color: var(--bg); }
        select { background: var(--bg); color: var(--fg); border: 1px solid var(--bg-secondary); }
        select:focus { outline: none; border-color: var(--purple); }
        .search-box { position: relative; }
        .search-box input {
            font-family: inherit;
            font-size: 0.7rem;
            padding: 0.4rem 0.7rem 0.4rem 1.8rem;
            background: var(--bg);
            color: var(--fg);
            border: 1px solid var(--bg-secondary);
            border-radius: 6px;
            width: 180px;
        }
        .search-box input:focus { outline: none; border-color: var(--purple); box-shadow: 0 0 0 2px rgba(189,147,249,0.15); }
        .search-box::before { content: '\1F50D'; position: absolute; left: 0.5rem; top: 50%%; transform: translateY(-50%%); font-size: 0.65rem; opacity: 0.6; }
        main { flex: 1; display: flex; overflow: hidden; position: relative; }
        #graph-container { flex: 1; position: relative; background: radial-gradient(ellipse at center, var(--bg) 0%%, var(--bg-tertiary) 100%%); }
        .overlay-stats {
            position: absolute; top: 0.75rem; left: 0.75rem;
            background: var(--bg-secondary); padding: 0.5rem 0.75rem;
            border-radius: 8px; font-size: 0.65rem; color: var(--fg-muted);
            z-index: 10; display: flex; gap: 1rem; border: 1px solid var(--bg-elevated);
            box-shadow: var(--shadow);
        }
        .overlay-stats .stat { display: flex; align-items: center; gap: 0.25rem; }
        .overlay-stats .stat-value { color: var(--cyan); font-weight: 600; }
        .minimap {
            position: absolute; bottom: 0.75rem; left: 0.75rem;
            width: 150px; height: 100px;
            background: var(--bg-tertiary); border: 1px solid var(--purple);
            border-radius: 8px; overflow: hidden; z-index: 10;
            box-shadow: var(--shadow);
        }
        .minimap canvas { width: 100%%; height: 100%%; }
        #sidebar {
            width: 300px;
            background: linear-gradient(180deg, var(--bg-secondary) 0%%, var(--bg) 100%%);
            border-left: 2px solid var(--purple);
            overflow-y: auto; padding: 1rem;
            display: flex; flex-direction: column; gap: 1rem;
        }
        .stats-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 0.5rem; }
        .stat-card {
            background: var(--bg-tertiary); padding: 0.65rem;
            border-radius: 8px; text-align: center;
            border: 1px solid var(--bg-elevated);
            transition: all 0.2s ease;
        }
        .stat-card:hover { border-color: var(--purple); transform: translateY(-1px); }
        .stat-value {
            font-size: 1.5rem; font-weight: 700;
            background: linear-gradient(90deg, var(--green), var(--cyan));
            -webkit-background-clip: text; -webkit-text-fill-color: transparent;
        }
        .stat-value.warning { background: linear-gradient(90deg, var(--orange), var(--red)); -webkit-background-clip: text; -webkit-text-fill-color: transparent; }
        .stat-label { font-size: 0.55rem; color: var(--fg-muted); text-transform: uppercase; letter-spacing: 0.5px; margin-top: 0.15rem; }
        .panel { background: var(--bg-tertiary); border-radius: 10px; padding: 0.75rem; border: 1px solid var(--bg-elevated); }
        .panel-title {
            font-size: 0.6rem; font-weight: 600; color: var(--purple);
            text-transform: uppercase; letter-spacing: 1px; margin-bottom: 0.6rem;
            display: flex; align-items: center; gap: 0.4rem;
        }
        .panel-title::before { content: ''; width: 3px; height: 10px; background: var(--purple); border-radius: 2px; }
        .legend { display: flex; flex-wrap: wrap; gap: 0.5rem; }
        .legend-item { display: flex; align-items: center; gap: 0.35rem; font-size: 0.6rem; color: var(--fg-muted); padding: 0.15rem 0.35rem; background: var(--bg); border-radius: 4px; }
        .legend-dot { width: 10px; height: 10px; border-radius: 50%%; box-shadow: 0 0 4px currentColor; }
        #node-detail { display: none; }
        #node-detail.visible { display: block; }
        .detail-header { display: flex; align-items: flex-start; justify-content: space-between; margin-bottom: 0.5rem; }
        .detail-id { font-size: 0.9rem; font-weight: 700; color: var(--cyan); }
        .detail-priority { padding: 0.2rem 0.5rem; border-radius: 4px; font-size: 0.55rem; font-weight: 700; text-transform: uppercase; }
        .detail-name { font-size: 0.7rem; color: var(--fg); line-height: 1.4; margin-bottom: 0.5rem; }
        .detail-badges { display: flex; gap: 0.35rem; flex-wrap: wrap; margin-bottom: 0.65rem; }
        .badge { font-size: 0.5rem; padding: 0.15rem 0.45rem; border-radius: 4px; text-transform: uppercase; font-weight: 600; letter-spacing: 0.3px; }
        .badge-open { background: var(--green); color: var(--bg); }
        .badge-in_progress { background: var(--orange); color: var(--bg); }
        .badge-blocked { background: var(--red); color: var(--bg); }
        .badge-closed { background: var(--fg-muted); color: var(--bg); }
        .badge-type { background: var(--purple); color: var(--bg); }
        .badge-articulation { background: linear-gradient(90deg, var(--pink), var(--purple)); color: var(--bg); animation: pulse 2s infinite; }
        .badge-critical { background: linear-gradient(90deg, var(--red), var(--orange)); color: var(--bg); }
        @keyframes pulse { 0%%, 100%% { opacity: 1; } 50%% { opacity: 0.7; } }
        .detail-metrics { display: grid; grid-template-columns: repeat(2, 1fr); gap: 0.35rem; font-size: 0.55rem; }
        .metric-item { display: flex; justify-content: space-between; padding: 0.25rem 0; border-bottom: 1px solid var(--bg-elevated); }
        .metric-label { color: var(--fg-muted); }
        .metric-value { color: var(--fg); font-weight: 500; }
        .metric-value.highlight { color: var(--green); }
        .no-selection { text-align: center; padding: 1.5rem 0.75rem; color: var(--fg-muted); font-size: 0.65rem; }
        .no-selection-icon { font-size: 1.5rem; margin-bottom: 0.5rem; opacity: 0.4; }
        .keyboard-hints { font-size: 0.55rem; color: var(--fg-muted); line-height: 1.7; }
        .keyboard-hints kbd { display: inline-block; background: var(--bg); padding: 0.15rem 0.35rem; border-radius: 3px; margin: 0 0.15rem; border: 1px solid var(--bg-elevated); }
        footer {
            background: var(--bg-tertiary); padding: 0.4rem 1rem;
            font-size: 0.55rem; color: var(--fg-muted);
            display: flex; justify-content: space-between; align-items: center;
            border-top: 1px solid var(--bg-secondary);
        }
        footer a { color: var(--cyan); text-decoration: none; }
        footer a:hover { text-decoration: underline; }
        .toast {
            position: fixed; bottom: 80px; left: 50%%; transform: translateX(-50%%);
            background: var(--bg-elevated); border: 1px solid var(--purple);
            padding: 0.6rem 1.2rem; border-radius: 8px; font-size: 0.7rem;
            z-index: 1000; box-shadow: var(--shadow);
            opacity: 0; transition: opacity 0.3s ease;
        }
        .toast.visible { opacity: 1; }
        .context-menu {
            position: fixed; background: var(--bg-elevated);
            border: 1px solid var(--purple); border-radius: 8px;
            padding: 0.35rem 0; z-index: 1000; min-width: 160px;
            box-shadow: var(--shadow); display: none;
        }
        .context-menu.visible { display: block; }
        .context-menu-item {
            padding: 0.45rem 0.85rem; font-size: 0.65rem; cursor: pointer;
            display: flex; align-items: center; gap: 0.5rem;
        }
        .context-menu-item:hover { background: var(--bg-secondary); }
        .context-menu-divider { height: 1px; background: var(--bg-secondary); margin: 0.25rem 0; }
        .path-highlight { position: absolute; top: 0.75rem; right: 0.75rem; z-index: 10; }
        .path-highlight button {
            background: var(--bg-secondary); border: 1px solid var(--purple);
            color: var(--fg); padding: 0.4rem 0.7rem; font-size: 0.6rem;
        }
        .fullscreen-btn {
            position: absolute; top: 0.75rem; right: 0.75rem;
            background: var(--bg-secondary); border: 1px solid var(--purple);
            color: var(--fg); padding: 0.4rem; border-radius: 6px;
            z-index: 10; cursor: pointer; font-size: 0.8rem;
        }
        .fullscreen-btn:hover { background: var(--bg-elevated); }
        .top-nodes-panel {
            position: absolute; top: 50px; right: 0.75rem;
            background: var(--bg-secondary); border: 1px solid var(--purple);
            border-radius: 8px; padding: 0.5rem; z-index: 10; max-height: 200px;
            overflow-y: auto; width: 180px; display: none;
        }
        .top-nodes-panel.visible { display: block; }
        .top-node-item {
            padding: 0.3rem 0.4rem; font-size: 0.55rem; cursor: pointer;
            border-radius: 4px; display: flex; justify-content: space-between;
        }
        .top-node-item:hover { background: var(--bg-elevated); }
        .top-node-item .rank { color: var(--purple); font-weight: 600; }
        .heatmap-legend {
            position: absolute; bottom: 0.75rem; right: 0.75rem;
            background: var(--bg-secondary); border: 1px solid var(--purple);
            border-radius: 8px; padding: 0.5rem; z-index: 10; display: none;
        }
        .heatmap-legend.visible { display: block; }
        .heatmap-gradient {
            width: 120px; height: 12px;
            background: linear-gradient(90deg, var(--green), var(--yellow), var(--orange), var(--red));
            border-radius: 3px; margin-bottom: 0.25rem;
        }
        .heatmap-labels { display: flex; justify-content: space-between; font-size: 0.5rem; color: var(--fg-muted); }
    </style>
</head>
<body>
    <header>
        <div class="logo">
            <div class="logo-icon">bv</div>
            <h1><span>%s</span> Graph</h1>
        </div>
        <div class="toolbar">
            <div class="search-box"><input type="text" id="search-input" placeholder="Search..."></div>
            <div class="toolbar-group">
                <select id="view-mode">
                    <option value="force">Force</option>
                    <option value="td">DAG ↓</option>
                    <option value="lr">DAG →</option>
                    <option value="radialout">Radial</option>
                </select>
            </div>
            <div class="toolbar-group">
                <select id="filter-status">
                    <option value="">All Status</option>
                    <option value="open">Open</option>
                    <option value="in_progress">In Progress</option>
                    <option value="blocked">Blocked</option>
                    <option value="closed">Closed</option>
                </select>
            </div>
            <div class="toolbar-group">
                <select id="size-by">
                    <option value="pagerank">Size: PageRank</option>
                    <option value="betweenness">Size: Betweenness</option>
                    <option value="critical">Size: Critical Path</option>
                    <option value="indegree">Size: In-Degree</option>
                </select>
            </div>
            <div class="toolbar-group">
                <button id="btn-heatmap" title="Toggle Heatmap">🔥</button>
                <button id="btn-top" title="Top Nodes">⭐</button>
                <button id="btn-fit" title="Fit (F)">Fit</button>
                <button id="btn-reset" title="Reset (R)">Reset</button>
            </div>
        </div>
    </header>
    <main>
        <div id="graph-container">
            <div class="overlay-stats">
                <div class="stat"><span class="stat-value">%d</span> nodes</div>
                <div class="stat"><span class="stat-value">%d</span> edges</div>
                <div class="stat" id="stat-visible"><span class="stat-value">%d</span> visible</div>
            </div>
            <button class="fullscreen-btn" id="btn-fullscreen" title="Fullscreen (Space)">⛶</button>
            <div class="top-nodes-panel" id="top-nodes-panel"></div>
            <div class="heatmap-legend" id="heatmap-legend">
                <div class="heatmap-gradient"></div>
                <div class="heatmap-labels"><span>Low</span><span id="heatmap-metric">PageRank</span><span>High</span></div>
            </div>
        </div>
        <div id="sidebar">
            <div class="stats-grid">
                <div class="stat-card"><div class="stat-value" id="stat-nodes">%d</div><div class="stat-label">Nodes</div></div>
                <div class="stat-card"><div class="stat-value" id="stat-edges">%d</div><div class="stat-label">Edges</div></div>
                <div class="stat-card"><div class="stat-value" id="stat-actionable">-</div><div class="stat-label">Actionable</div></div>
                <div class="stat-card"><div class="stat-value warning" id="stat-blocked">-</div><div class="stat-label">Blocked</div></div>
                <div class="stat-card"><div class="stat-value" id="stat-critical">-</div><div class="stat-label">Critical</div></div>
                <div class="stat-card"><div class="stat-value warning" id="stat-articulation">-</div><div class="stat-label">Cut Pts</div></div>
            </div>
            <div class="panel">
                <div class="panel-title">Status</div>
                <div class="legend">
                    <div class="legend-item"><div class="legend-dot" style="background:#50fa7b;color:#50fa7b"></div>Open</div>
                    <div class="legend-item"><div class="legend-dot" style="background:#ffb86c;color:#ffb86c"></div>In Progress</div>
                    <div class="legend-item"><div class="legend-dot" style="background:#ff5555;color:#ff5555"></div>Blocked</div>
                    <div class="legend-item"><div class="legend-dot" style="background:#6272a4;color:#6272a4"></div>Closed</div>
                </div>
            </div>
            <div class="panel">
                <div class="panel-title">Priority</div>
                <div class="legend">
                    <div class="legend-item"><div class="legend-dot" style="background:#ff0000;color:#ff0000"></div>P0</div>
                    <div class="legend-item"><div class="legend-dot" style="background:#ff5555;color:#ff5555"></div>P1</div>
                    <div class="legend-item"><div class="legend-dot" style="background:#ffb86c;color:#ffb86c"></div>P2</div>
                    <div class="legend-item"><div class="legend-dot" style="background:#f1fa8c;color:#f1fa8c"></div>P3</div>
                    <div class="legend-item"><div class="legend-dot" style="background:#6272a4;color:#6272a4"></div>P4</div>
                </div>
            </div>
            <div class="panel">
                <div class="panel-title">Selected Node</div>
                <div id="node-detail">
                    <div class="detail-header">
                        <div class="detail-id" id="detail-id">-</div>
                        <div class="detail-priority" id="detail-priority">P2</div>
                    </div>
                    <div class="detail-name" id="detail-name">-</div>
                    <div class="detail-badges" id="detail-badges"></div>
                    <div class="detail-metrics">
                        <div class="metric-item"><span class="metric-label">PageRank</span><span class="metric-value" id="m-pagerank">-</span></div>
                        <div class="metric-item"><span class="metric-label">PR Rank</span><span class="metric-value" id="m-prrank">-</span></div>
                        <div class="metric-item"><span class="metric-label">Betweenness</span><span class="metric-value" id="m-between">-</span></div>
                        <div class="metric-item"><span class="metric-label">BW Rank</span><span class="metric-value" id="m-bwrank">-</span></div>
                        <div class="metric-item"><span class="metric-label">Critical</span><span class="metric-value" id="m-critical">-</span></div>
                        <div class="metric-item"><span class="metric-label">Slack</span><span class="metric-value" id="m-slack">-</span></div>
                        <div class="metric-item"><span class="metric-label">In-Degree</span><span class="metric-value" id="m-indeg">-</span></div>
                        <div class="metric-item"><span class="metric-label">Out-Degree</span><span class="metric-value" id="m-outdeg">-</span></div>
                        <div class="metric-item"><span class="metric-label">Hub</span><span class="metric-value" id="m-hub">-</span></div>
                        <div class="metric-item"><span class="metric-label">Authority</span><span class="metric-value" id="m-auth">-</span></div>
                        <div class="metric-item"><span class="metric-label">Core #</span><span class="metric-value" id="m-core">-</span></div>
                        <div class="metric-item"><span class="metric-label">Eigenvector</span><span class="metric-value" id="m-eigen">-</span></div>
                    </div>
                </div>
                <div class="no-selection" id="no-selection">
                    <div class="no-selection-icon">🔍</div>
                    Click a node to see metrics
                </div>
            </div>
            <div class="panel">
                <div class="panel-title">Shortcuts</div>
                <div class="keyboard-hints">
                    <kbd>F</kbd> Fit · <kbd>R</kbd> Reset · <kbd>Space</kbd> Fullscreen<br>
                    <kbd>Esc</kbd> Clear · <kbd>1-4</kbd> View modes<br>
                    <kbd>H</kbd> Heatmap · <kbd>T</kbd> Top nodes
                </div>
            </div>
        </div>
    </main>
    <footer>
        <div>Generated %s | Hash: %s</div>
        <div>Project: %s | <a href="https://github.com/Dicklesworthstone/beads_viewer">bv</a></div>
    </footer>
    <div class="toast" id="toast"></div>
    <div class="context-menu" id="context-menu">
        <div class="context-menu-item" id="ctx-focus">🎯 Focus on this node</div>
        <div class="context-menu-item" id="ctx-deps">📥 Show dependencies</div>
        <div class="context-menu-item" id="ctx-dependents">📤 Show dependents</div>
        <div class="context-menu-divider"></div>
        <div class="context-menu-item" id="ctx-path">🛤️ Find path to...</div>
        <div class="context-menu-item" id="ctx-copy">📋 Copy ID</div>
    </div>
    <script src="https://unpkg.com/force-graph@1.43.5/dist/force-graph.min.js"></script>
    <script>
const DATA = %s;
const STATUS_COLORS = { open: '#50fa7b', in_progress: '#ffb86c', blocked: '#ff5555', closed: '#6272a4' };
const PRIORITY_COLORS = ['#ff0000', '#ff5555', '#ffb86c', '#f1fa8c', '#6272a4'];

// Stats calculation
let actionable = 0, blocked = 0, onCriticalPath = 0, articulationCount = 0;
const blockerCount = {};
DATA.links.forEach(l => blockerCount[l.source] = (blockerCount[l.source] || 0) + 1);
DATA.nodes.forEach(n => {
    n.blockerCount = blockerCount[n.id] || 0;
    if ((n.status === 'open' || n.status === 'in_progress') && n.blockerCount === 0) actionable++;
    if (n.status === 'blocked') blocked++;
    if (n.slack === 0) onCriticalPath++;
    if (n.is_articulation) articulationCount++;
});
document.getElementById('stat-actionable').textContent = actionable;
document.getElementById('stat-blocked').textContent = blocked;
document.getElementById('stat-critical').textContent = onCriticalPath;
document.getElementById('stat-articulation').textContent = articulationCount;

// Max values for sizing
const maxPR = Math.max(...DATA.nodes.map(n => n.pagerank || 0), 0.001);
const maxBW = Math.max(...DATA.nodes.map(n => n.betweenness || 0), 0.001);
const maxCP = Math.max(...DATA.nodes.map(n => n.critical_path || 0), 1);
const maxInDeg = Math.max(...DATA.nodes.map(n => n.in_degree || 0), 1);

let sizeMetric = 'pagerank', heatmapMode = false;
function getNodeSize(n) {
    const base = 4, scale = 14;
    switch(sizeMetric) {
        case 'pagerank': return base + ((n.pagerank || 0) / maxPR) * scale;
        case 'betweenness': return base + ((n.betweenness || 0) / maxBW) * scale;
        case 'critical': return base + ((n.critical_path || 0) / maxCP) * scale;
        case 'indegree': return base + ((n.in_degree || 0) / maxInDeg) * scale;
        default: return base + ((n.pagerank || 0) / maxPR) * scale;
    }
}

function getHeatmapColor(n) {
    let val = 0, max = 1;
    switch(sizeMetric) {
        case 'pagerank': val = n.pagerank || 0; max = maxPR; break;
        case 'betweenness': val = n.betweenness || 0; max = maxBW; break;
        case 'critical': val = n.critical_path || 0; max = maxCP; break;
        case 'indegree': val = n.in_degree || 0; max = maxInDeg; break;
    }
    const ratio = val / max;
    if (ratio < 0.33) return 'hsl(' + (120 - ratio * 180) + ', 70%%, 50%%)';
    if (ratio < 0.66) return 'hsl(' + (60 - (ratio - 0.33) * 180) + ', 80%%, 50%%)';
    return 'hsl(' + (0 - (ratio - 0.66) * 60) + ', 90%%, 50%%)';
}

const container = document.getElementById('graph-container');
const Graph = ForceGraph()(container)
    .graphData(JSON.parse(JSON.stringify(DATA)))
    .backgroundColor('transparent')
    .nodeId('id')
    .nodeLabel(null)
    .nodeColor(n => heatmapMode ? getHeatmapColor(n) : STATUS_COLORS[n.status] || '#6272a4')
    .nodeVal(n => getNodeSize(n))
    .linkColor(l => l.critical ? '#ff79c680' : '#44475a40')
    .linkWidth(l => l.critical ? 2 : 0.8)
    .linkDirectionalArrowLength(4)
    .linkDirectionalArrowColor(l => l.critical ? '#ff79c6' : '#44475a')
    .linkDirectionalArrowRelPos(1)
    .linkCurvature(0.12)
    .linkDirectionalParticles(l => l.critical ? 2 : 0)
    .linkDirectionalParticleSpeed(0.003)
    .linkDirectionalParticleWidth(2)
    .linkDirectionalParticleColor(() => '#ff79c6')
    .d3AlphaDecay(0.02)
    .d3VelocityDecay(0.25)
    .nodeCanvasObject((node, ctx, globalScale) => {
        const size = getNodeSize(node);
        const color = heatmapMode ? getHeatmapColor(node) : STATUS_COLORS[node.status] || '#6272a4';
        const x = node.x, y = node.y;
        if (node.is_articulation) {
            ctx.beginPath(); ctx.arc(x, y, size + 5, 0, 2 * Math.PI);
            const g = ctx.createRadialGradient(x, y, size, x, y, size + 7);
            g.addColorStop(0, '#ff79c650'); g.addColorStop(1, 'transparent');
            ctx.fillStyle = g; ctx.fill();
        }
        if (node.slack === 0) {
            ctx.beginPath(); ctx.arc(x, y, size + 2.5, 0, 2 * Math.PI);
            ctx.fillStyle = color + '25'; ctx.fill();
        }
        const pColor = PRIORITY_COLORS[node.priority] || PRIORITY_COLORS[2];
        ctx.beginPath(); ctx.arc(x, y, size + 1.2, 0, 2 * Math.PI);
        ctx.strokeStyle = pColor; ctx.lineWidth = 1.5; ctx.stroke();
        ctx.beginPath(); ctx.arc(x, y, size, 0, 2 * Math.PI);
        ctx.fillStyle = color; ctx.fill();
        const hl = ctx.createRadialGradient(x - size/3, y - size/3, 0, x, y, size);
        hl.addColorStop(0, 'rgba(255,255,255,0.15)'); hl.addColorStop(1, 'transparent');
        ctx.fillStyle = hl; ctx.fill();
        if (globalScale > 1.3) {
            ctx.font = (globalScale > 2.5 ? '3.5px' : '2.5px') + ' JetBrains Mono';
            ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
            ctx.fillStyle = '#f8f8f2'; ctx.fillText(node.id, x, y + size + 5);
            if (globalScale > 3) { ctx.fillStyle = pColor; ctx.fillText('P' + node.priority, x, y); }
        }
    })
    .nodePointerAreaPaint((n, c, ctx) => {
        const size = getNodeSize(n) + 3;
        ctx.fillStyle = c; ctx.beginPath(); ctx.arc(n.x, n.y, size, 0, 2 * Math.PI); ctx.fill();
    })
    .onNodeClick(selectNode)
    .onNodeRightClick((node, event) => { event.preventDefault(); showContextMenu(node, event); })
    .onNodeHover(n => container.style.cursor = n ? 'pointer' : 'grab')
    .onBackgroundClick(() => { clearSelection(); hideContextMenu(); })
    .onBackgroundRightClick(() => hideContextMenu());

let selectedNode = null;
function selectNode(node) {
    selectedNode = node;
    document.getElementById('detail-id').textContent = node.id;
    document.getElementById('detail-name').textContent = node.title;
    const prioEl = document.getElementById('detail-priority');
    prioEl.textContent = 'P' + node.priority;
    prioEl.style.background = PRIORITY_COLORS[node.priority];
    prioEl.style.color = node.priority <= 1 ? '#f8f8f2' : '#21222c';
    const badgesEl = document.getElementById('detail-badges');
    badgesEl.innerHTML = '';
    const sb = document.createElement('span'); sb.className = 'badge badge-' + node.status; sb.textContent = node.status.replace('_', ' '); badgesEl.appendChild(sb);
    const tb = document.createElement('span'); tb.className = 'badge badge-type'; tb.textContent = node.type; badgesEl.appendChild(tb);
    if (node.is_articulation) { const ab = document.createElement('span'); ab.className = 'badge badge-articulation'; ab.textContent = 'Cut Vertex'; badgesEl.appendChild(ab); }
    if (node.slack === 0) { const cb = document.createElement('span'); cb.className = 'badge badge-critical'; cb.textContent = 'Critical Path'; badgesEl.appendChild(cb); }
    document.getElementById('m-pagerank').textContent = (node.pagerank * 100).toFixed(3) + '%%';
    document.getElementById('m-prrank').textContent = '#' + (node.pagerank_rank || '-');
    document.getElementById('m-between').textContent = node.betweenness.toFixed(4);
    document.getElementById('m-bwrank').textContent = '#' + (node.betweenness_rank || '-');
    document.getElementById('m-critical').textContent = node.critical_path.toFixed(1);
    const slackEl = document.getElementById('m-slack');
    slackEl.textContent = node.slack.toFixed(1);
    slackEl.className = 'metric-value' + (node.slack === 0 ? ' highlight' : '');
    document.getElementById('m-indeg').textContent = node.in_degree;
    document.getElementById('m-outdeg').textContent = node.out_degree;
    document.getElementById('m-hub').textContent = node.hub.toFixed(4);
    document.getElementById('m-auth').textContent = node.authority.toFixed(4);
    document.getElementById('m-core').textContent = node.core_number;
    document.getElementById('m-eigen').textContent = node.eigenvector.toFixed(4);
    document.getElementById('node-detail').classList.add('visible');
    document.getElementById('no-selection').style.display = 'none';
    Graph.nodeColor(n => {
        if (n.id === node.id) return '#f8f8f2';
        const isConn = DATA.links.some(l => (l.source === node.id && l.target === n.id) || (l.target === node.id && l.source === n.id) || (l.source.id === node.id && l.target.id === n.id) || (l.target.id === node.id && l.source.id === n.id));
        if (isConn) return STATUS_COLORS[n.status] || '#6272a4';
        return (STATUS_COLORS[n.status] || '#6272a4') + '30';
    });
}
function clearSelection() {
    selectedNode = null;
    document.getElementById('node-detail').classList.remove('visible');
    document.getElementById('no-selection').style.display = 'block';
    Graph.nodeColor(n => heatmapMode ? getHeatmapColor(n) : STATUS_COLORS[n.status] || '#6272a4');
}

// Context menu
let contextNode = null;
function showContextMenu(node, event) {
    contextNode = node;
    const menu = document.getElementById('context-menu');
    menu.style.left = event.clientX + 'px';
    menu.style.top = event.clientY + 'px';
    menu.classList.add('visible');
}
function hideContextMenu() { document.getElementById('context-menu').classList.remove('visible'); contextNode = null; }
document.getElementById('ctx-focus').onclick = () => { if (contextNode) { Graph.centerAt(contextNode.x, contextNode.y, 500); Graph.zoom(3, 500); } hideContextMenu(); };
document.getElementById('ctx-deps').onclick = () => { if (contextNode) highlightDependencies(contextNode, 'deps'); hideContextMenu(); };
document.getElementById('ctx-dependents').onclick = () => { if (contextNode) highlightDependencies(contextNode, 'dependents'); hideContextMenu(); };
document.getElementById('ctx-copy').onclick = () => { if (contextNode) { navigator.clipboard.writeText(contextNode.id); showToast('Copied: ' + contextNode.id); } hideContextMenu(); };
document.getElementById('ctx-path').onclick = () => { showToast('Click another node to find path'); pathStartNode = contextNode; hideContextMenu(); };

let pathStartNode = null;
function highlightDependencies(node, type) {
    const connected = new Set([node.id]);
    DATA.links.forEach(l => {
        const src = typeof l.source === 'object' ? l.source.id : l.source;
        const tgt = typeof l.target === 'object' ? l.target.id : l.target;
        if (type === 'deps' && src === node.id) connected.add(tgt);
        if (type === 'dependents' && tgt === node.id) connected.add(src);
    });
    Graph.nodeVisibility(n => connected.has(n.id));
    updateVisibleCount();
    showToast(connected.size + ' nodes shown');
}

// Search and filter
let searchTerm = '', statusFilter = '';
document.getElementById('search-input').oninput = e => { searchTerm = e.target.value.toLowerCase(); applyFilters(); };
document.getElementById('filter-status').onchange = e => { statusFilter = e.target.value; applyFilters(); };
function applyFilters() {
    Graph.nodeVisibility(n => {
        const matchSearch = !searchTerm || n.id.toLowerCase().includes(searchTerm) || n.title.toLowerCase().includes(searchTerm) || (n.labels || []).some(l => l.toLowerCase().includes(searchTerm));
        const matchStatus = !statusFilter || n.status === statusFilter;
        return matchSearch && matchStatus;
    });
    updateVisibleCount();
}
function updateVisibleCount() {
    const count = DATA.nodes.filter(n => Graph.nodeVisibility()(n)).length;
    document.getElementById('stat-visible').innerHTML = '<span class="stat-value">' + count + '</span> visible';
}

// View mode
document.getElementById('view-mode').onchange = e => {
    const mode = e.target.value;
    Graph.dagMode(mode === 'force' ? null : mode);
    setTimeout(() => Graph.zoomToFit(400), 100);
};

// Size metric
document.getElementById('size-by').onchange = e => {
    sizeMetric = e.target.value;
    document.getElementById('heatmap-metric').textContent = { pagerank: 'PageRank', betweenness: 'Betweenness', critical: 'Critical Path', indegree: 'In-Degree' }[sizeMetric];
    Graph.nodeVal(n => getNodeSize(n));
    if (heatmapMode) Graph.nodeColor(n => getHeatmapColor(n));
};

// Controls
document.getElementById('btn-fit').onclick = () => Graph.zoomToFit(400, 40);
document.getElementById('btn-reset').onclick = () => {
    document.getElementById('filter-status').value = '';
    document.getElementById('search-input').value = '';
    document.getElementById('view-mode').value = 'force';
    document.getElementById('size-by').value = 'pagerank';
    searchTerm = ''; statusFilter = ''; sizeMetric = 'pagerank'; heatmapMode = false;
    Graph.dagMode(null); Graph.nodeVisibility(() => true); Graph.nodeVal(n => getNodeSize(n));
    Graph.nodeColor(n => STATUS_COLORS[n.status] || '#6272a4');
    clearSelection(); Graph.zoomToFit(400, 40); updateVisibleCount();
    document.getElementById('heatmap-legend').classList.remove('visible');
    document.getElementById('top-nodes-panel').classList.remove('visible');
};

// Heatmap toggle
document.getElementById('btn-heatmap').onclick = () => {
    heatmapMode = !heatmapMode;
    document.getElementById('btn-heatmap').classList.toggle('active', heatmapMode);
    document.getElementById('heatmap-legend').classList.toggle('visible', heatmapMode);
    Graph.nodeColor(n => heatmapMode ? getHeatmapColor(n) : STATUS_COLORS[n.status] || '#6272a4');
};

// Top nodes panel
document.getElementById('btn-top').onclick = () => {
    const panel = document.getElementById('top-nodes-panel');
    const visible = panel.classList.toggle('visible');
    document.getElementById('btn-top').classList.toggle('active', visible);
    if (visible) {
        const sorted = [...DATA.nodes].sort((a, b) => (b.pagerank || 0) - (a.pagerank || 0)).slice(0, 10);
        panel.innerHTML = sorted.map((n, i) => '<div class="top-node-item" data-id="' + n.id + '"><span class="rank">#' + (i+1) + '</span><span>' + n.id + '</span></div>').join('');
        panel.querySelectorAll('.top-node-item').forEach(el => {
            el.onclick = () => {
                const node = DATA.nodes.find(n => n.id === el.dataset.id);
                if (node) { selectNode(node); Graph.centerAt(node.x, node.y, 500); Graph.zoom(2.5, 500); }
            };
        });
    }
};

// Fullscreen
document.getElementById('btn-fullscreen').onclick = () => {
    if (!document.fullscreenElement) container.requestFullscreen();
    else document.exitFullscreen();
};

// Toast
function showToast(msg) {
    const toast = document.getElementById('toast');
    toast.textContent = msg; toast.classList.add('visible');
    setTimeout(() => toast.classList.remove('visible'), 2000);
}

// Keyboard shortcuts
document.onkeydown = e => {
    if (e.target.tagName === 'INPUT') return;
    switch(e.key.toLowerCase()) {
        case 'f': Graph.zoomToFit(400, 40); break;
        case 'r': document.getElementById('btn-reset').click(); break;
        case 'escape': clearSelection(); break;
        case ' ': e.preventDefault(); document.getElementById('btn-fullscreen').click(); break;
        case 'h': document.getElementById('btn-heatmap').click(); break;
        case 't': document.getElementById('btn-top').click(); break;
        case '1': document.getElementById('view-mode').value = 'force'; Graph.dagMode(null); break;
        case '2': document.getElementById('view-mode').value = 'td'; Graph.dagMode('td'); break;
        case '3': document.getElementById('view-mode').value = 'lr'; Graph.dagMode('lr'); break;
        case '4': document.getElementById('view-mode').value = 'radialout'; Graph.dagMode('radialout'); break;
    }
};

// Initial fit
setTimeout(() => { Graph.zoomToFit(400, 40); updateVisibleCount(); }, 800);
    </script>
</body>
</html>`, title, title, nodeCount, edgeCount, nodeCount, nodeCount, edgeCount, timestamp, dataHash, projectName, graphDataJSON)
}
