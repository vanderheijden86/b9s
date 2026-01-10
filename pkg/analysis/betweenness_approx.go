package analysis

import (
	"math/rand"
	"runtime"
	"sort"
	"sync"
	"time"

	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/simple"
)

// brandesBuffers holds reusable data structures for Brandes' algorithm.
// These buffers are pooled via sync.Pool to avoid per-call allocations.
//
// Memory characteristics:
//   - sigma: stores shortest path counts, O(V) entries
//   - dist: stores BFS distances (-1 = unvisited), O(V) entries
//   - delta: stores dependency accumulation, O(V) entries
//   - pred: stores predecessor lists, O(V) entries + O(E) total slice capacity
//   - queue: BFS frontier, up to O(V) capacity
//   - stack: reverse order for accumulation, up to O(V) capacity
//   - neighbors: temporary slice for iterator results, typically small
type brandesBuffers struct {
	sigma     map[int64]float64 // σ_s(v) = number of shortest paths from s through v
	dist      map[int64]int     // d_s(v) = distance from source s to v (-1 = infinity)
	delta     map[int64]float64 // δ_s(v) = dependency of s on v
	pred      map[int64][]int64 // P_s(v) = predecessors of v on shortest paths from s
	queue     []int64           // BFS queue (FIFO)
	stack     []int64           // Visited nodes in BFS order (LIFO for backprop)
	neighbors []int64           // Temp slice to collect neighbor IDs from iterator
}

// brandesPool provides reusable buffer sets for singleSourceBetweenness.
// Pre-allocation with capacity 256 handles most real-world graphs efficiently;
// maps will grow if needed but retain capacity for subsequent reuse.
//
// Concurrency: sync.Pool is safe for concurrent Get/Put. Each goroutine
// gets its own buffer; no synchronization needed during algorithm execution.
//
// GC behavior: Pool may discard buffers during GC. This is acceptable since
// New() will create fresh buffers as needed; we trade occasional allocations
// for reduced peak memory during steady-state operation.
var brandesPool = sync.Pool{
	New: func() interface{} {
		return &brandesBuffers{
			sigma:     make(map[int64]float64, 256),
			dist:      make(map[int64]int, 256),
			delta:     make(map[int64]float64, 256),
			pred:      make(map[int64][]int64, 256),
			queue:     make([]int64, 0, 256),
			stack:     make([]int64, 0, 256),
			neighbors: make([]int64, 0, 32),
		}
	},
}

// reset clears buffer contents while retaining allocated capacity.
// Must be called before each new source node BFS traversal.
//
// Memory strategy:
//   - If maps grew >2x node count, use clear() to free excess entries
//     while retaining underlying capacity (prevents unbounded growth)
//   - For normal-sized maps, iterate and reset values in-place
//   - Slices reset via [:0] to retain backing array
//
// Initialization values match fresh-allocation semantics:
//   - sigma[nid] = 0 (no paths counted yet)
//   - dist[nid] = -1 (infinity/unvisited sentinel)
//   - delta[nid] = 0 (no dependency accumulated)
//   - pred[nid] = pred[nid][:0] (empty predecessor list, retain slice capacity)
func (b *brandesBuffers) reset(nodes []graph.Node) {
	nodeCount := len(nodes)

	// Clear maps if they've grown excessively (prevents unbounded memory)
	// Threshold: 2x node count indicates significant graph size change
	if len(b.sigma) > nodeCount*2 {
		clear(b.sigma)
		clear(b.dist)
		clear(b.delta)
		clear(b.pred)
	}

	// Initialize all node entries
	for _, n := range nodes {
		nid := n.ID()
		b.sigma[nid] = 0
		b.dist[nid] = -1
		b.delta[nid] = 0
		// Reuse predecessor slice backing array, reset length to 0
		if existing, ok := b.pred[nid]; ok {
			b.pred[nid] = existing[:0]
		} else {
			// First time seeing this node - allocate small slice
			b.pred[nid] = make([]int64, 0, 4)
		}
	}

	// Reset auxiliary slices (retain capacity)
	b.queue = b.queue[:0]
	b.stack = b.stack[:0]
	b.neighbors = b.neighbors[:0]
}

// BetweennessMode specifies how betweenness centrality should be computed.
type BetweennessMode string

const (
	// BetweennessExact computes exact betweenness centrality using Brandes' algorithm.
	// Complexity: O(V*E) - fast for small graphs, slow for large graphs.
	BetweennessExact BetweennessMode = "exact"

	// BetweennessApproximate uses sampling-based approximation.
	// Complexity: O(k*E) where k is the sample size - much faster for large graphs.
	// Error: O(1/sqrt(k)) - with k=100, ~10% error in ranking.
	BetweennessApproximate BetweennessMode = "approximate"

	// BetweennessSkip disables betweenness computation entirely.
	BetweennessSkip BetweennessMode = "skip"
)

// BetweennessResult contains the result of betweenness computation.
type BetweennessResult struct {
	// Scores maps node IDs to their betweenness centrality scores
	Scores map[int64]float64

	// Mode indicates how the result was computed
	Mode BetweennessMode

	// SampleSize is the number of pivot nodes used (only for approximate mode)
	SampleSize int

	// TotalNodes is the total number of nodes in the graph
	TotalNodes int

	// Elapsed is the time taken to compute
	Elapsed time.Duration

	// TimedOut indicates if computation was interrupted by timeout
	TimedOut bool
}

// ApproxBetweenness computes approximate betweenness centrality using sampling.
//
// Instead of computing shortest paths from ALL nodes (O(V*E)), we sample k pivot
// nodes and extrapolate. This is Brandes' approximation algorithm.
//
// Error bounds: With k samples, approximation error is O(1/sqrt(k)):
//   - k=50: ~14% error
//   - k=100: ~10% error
//   - k=200: ~7% error
//
// For ranking purposes (which node is most central), this is usually sufficient.
//
// References:
//   - "A Faster Algorithm for Betweenness Centrality" (Brandes, 2001)
//   - "Approximating Betweenness Centrality" (Bader et al., 2007)
func ApproxBetweenness(g *simple.DirectedGraph, sampleSize int, seed int64) BetweennessResult {
	start := time.Now()
	nodes := graph.NodesOf(g.Nodes())
	n := len(nodes)
	// Ensure deterministic ordering before sampling; gonum's Nodes may be map-backed.
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID() < nodes[j].ID() })

	// Clamp sampleSize to valid range [1, n] to prevent division by zero and negative slice indices
	if sampleSize < 1 {
		sampleSize = 1
	}

	result := BetweennessResult{
		Scores:     make(map[int64]float64),
		Mode:       BetweennessApproximate,
		SampleSize: sampleSize,
		TotalNodes: n,
	}

	if n == 0 {
		result.Elapsed = time.Since(start)
		return result
	}

	// For small graphs or when sample size >= node count, use exact algorithm
	if sampleSize >= n {
		exact := network.Betweenness(g)
		result.Scores = exact
		result.Mode = BetweennessExact
		result.SampleSize = n
		result.Elapsed = time.Since(start)
		return result
	}

	// Sample k random pivot nodes
	pivots := sampleNodes(nodes, sampleSize, seed)

	// Compute partial betweenness from sampled pivots in parallel
	partialBC := make(map[int64]float64)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Limit concurrency to avoid excessive goroutines
	sem := make(chan struct{}, runtime.NumCPU())

	for _, pivot := range pivots {
		wg.Add(1)
		go func(p graph.Node) {
			defer wg.Done()
			sem <- struct{}{} // Acquire token
			defer func() { <-sem }()

			// Compute local contribution
			localBC := make(map[int64]float64)
			singleSourceBetweenness(g, p, localBC)

			// Merge into global result
			mu.Lock()
			for id, val := range localBC {
				partialBC[id] += val
			}
			mu.Unlock()
		}(pivot)
	}
	wg.Wait()

	// Scale up: BC_approx = BC_partial * (n / k)
	// This extrapolates from the sample to the full graph
	scale := float64(n) / float64(sampleSize)
	for id := range partialBC {
		partialBC[id] *= scale
	}

	result.Scores = partialBC
	result.Elapsed = time.Since(start)
	return result
}

// sampleNodes returns a random sample of k nodes from the input slice.
// Uses Fisher-Yates shuffle for unbiased sampling.
func sampleNodes(nodes []graph.Node, k int, seed int64) []graph.Node {
	if k >= len(nodes) {
		return nodes
	}

	// Create a copy to avoid modifying the original
	shuffled := make([]graph.Node, len(nodes))
	copy(shuffled, nodes)

	// Fisher-Yates shuffle for first k elements
	rng := rand.New(rand.NewSource(seed))
	for i := 0; i < k; i++ {
		j := i + rng.Intn(len(shuffled)-i)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled[:k]
}

// singleSourceBetweenness computes the betweenness contribution from a single source node.
// This is the core of Brandes' algorithm, run once per pivot.
//
// The algorithm performs BFS from the source and accumulates dependency scores
// in a reverse topological order traversal.
func singleSourceBetweenness(g *simple.DirectedGraph, source graph.Node, bc map[int64]float64) {
	sourceID := source.ID()
	nodes := graph.NodesOf(g.Nodes())

	// Get buffer from pool - will be returned after this BFS completes
	buf := brandesPool.Get().(*brandesBuffers)
	defer brandesPool.Put(buf)

	// Initialize buffer for this source (clears previous state while retaining capacity)
	buf.reset(nodes)

	// Use pooled data structures (aliases for readability)
	sigma := buf.sigma
	dist := buf.dist
	delta := buf.delta
	pred := buf.pred

	sigma[sourceID] = 1
	dist[sourceID] = 0

	// Queue for BFS (reuse pooled slice)
	buf.queue = append(buf.queue, sourceID)

	// BFS phase
	for len(buf.queue) > 0 {
		v := buf.queue[0]
		buf.queue = buf.queue[1:]
		buf.stack = append(buf.stack, v)

		// Collect neighbors into pooled slice to avoid iterator allocation
		buf.neighbors = buf.neighbors[:0]
		to := g.From(v) // Outgoing edges
		for to.Next() {
			buf.neighbors = append(buf.neighbors, to.Node().ID())
		}
		// Sort neighbors to ensure deterministic BFS and predecessor order
		sort.Slice(buf.neighbors, func(i, j int) bool { return buf.neighbors[i] < buf.neighbors[j] })

		for _, w := range buf.neighbors {
			// Path discovery
			if dist[w] < 0 {
				dist[w] = dist[v] + 1
				buf.queue = append(buf.queue, w)
			}

			// Path counting
			if dist[w] == dist[v]+1 {
				sigma[w] += sigma[v]
				pred[w] = append(pred[w], v)
			}
		}
	}

	// Accumulation phase
	for i := len(buf.stack) - 1; i >= 0; i-- {
		w := buf.stack[i]
		if w == sourceID {
			continue
		}

		for _, v := range pred[w] {
			if sigma[w] > 0 {
				delta[v] += (sigma[v] / sigma[w]) * (1 + delta[w])
			}
		}

		// Add dependency to betweenness (w != sourceID already checked above)
		bc[w] += delta[w]
	}
}

// RecommendSampleSize returns a recommended sample size based on graph characteristics.
// The goal is to balance accuracy vs. speed.
//
// Note: edgeCount is accepted for future density-aware heuristics but currently unused.
func RecommendSampleSize(nodeCount, edgeCount int) int {
	_ = edgeCount // Reserved for future density-aware sampling heuristics
	switch {
	case nodeCount < 100:
		// Small graph: use exact algorithm
		return nodeCount
	case nodeCount < 500:
		// Medium graph: 20% sample for ~22% error
		minSample := 50
		sample := nodeCount / 5
		if sample > minSample {
			return sample
		}
		return minSample
	case nodeCount < 2000:
		// Large graph: fixed sample for ~10% error
		return 100
	default:
		// XL graph: larger fixed sample
		return 200
	}
}
