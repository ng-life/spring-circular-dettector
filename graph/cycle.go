package graph

const (
	white = 0
	gray  = 1
	black = 2
)

// Cycle represents a detected cycle in the dependency graph.
type Cycle struct {
	Path  []string // ordered list of nodes in the cycle
	Edges []string // edge descriptions for each hop
}

// FindCycles detects all simple cycles in the dependency graph using DFS.
func FindCycles(g *DependencyGraph) []Cycle {
	color := make(map[string]int)
	parent := make(map[string]string)
	var cycles []Cycle
	// Track unique cycles (by canonical path) to deduplicate
	seen := make(map[string]bool)

	// Process nodes in deterministic order
	nodes := sortedNodes(g)

	for _, node := range nodes {
		if color[node] == white {
			dfs(node, g, color, parent, &cycles, &seen)
		}
	}

	return cycles
}

func dfs(node string, g *DependencyGraph, color map[string]int, parent map[string]string,
	cycles *[]Cycle, seen *map[string]bool) {

	color[node] = gray

	for neighbor := range g.edges[node] {
		if color[neighbor] == white {
			parent[neighbor] = node
			dfs(neighbor, g, color, parent, cycles, seen)
		} else if color[neighbor] == gray {
			// Found a back edge: node -> neighbor forms a cycle
			// Reconstruct the cycle path
			cyclePath := reconstructPath(node, neighbor, parent)
			canonical := canonicalPath(cyclePath)
			if !(*seen)[canonical] {
				(*seen)[canonical] = true
				edges := buildEdgeDescriptions(cyclePath, g)
				*cycles = append(*cycles, Cycle{
					Path:  cyclePath,
					Edges: edges,
				})
			}
		}
		// neighbor is black: cross edge, ignore
	}

	color[node] = black
}

// reconstructPath builds the cycle path from the back edge.
func reconstructPath(from, to string, parent map[string]string) []string {
	path := []string{to}
	// Walk from "from" up to "to" via parent pointers
	for cur := from; cur != to; cur = parent[cur] {
		path = append([]string{cur}, path...)
	}
	// Complete the cycle by adding "to" at the end
	path = append(path, to)
	return path
}

// canonicalPath creates a canonical representation of a cycle for deduplication.
func canonicalPath(path []string) string {
	if len(path) <= 1 {
		return ""
	}
	// The cycle nodes (without the repeated last node)
	nodes := path[:len(path)-1]

	// Find the smallest element (lexicographically) to start at
	minIdx := 0
	for i := 1; i < len(nodes); i++ {
		if nodes[i] < nodes[minIdx] {
			minIdx = i
		}
	}

	// Build canonical string
	result := ""
	for i := 0; i < len(nodes); i++ {
		idx := (minIdx + i) % len(nodes)
		result += nodes[idx]
		if i < len(nodes)-1 {
			result += "->"
		}
	}
	return result
}

// buildEdgeDescriptions creates human-readable descriptions for each edge in the cycle.
func buildEdgeDescriptions(path []string, g *DependencyGraph) []string {
	var edges []string
	for i := 0; i < len(path)-1; i++ {
		from := path[i]
		to := path[i+1]
		key := from + "->" + to
		info := g.edgeInfo[key]
		if info == "" {
			info = "injects"
		}
		edges = append(edges, from+" --["+info+"]--> "+to)
	}
	return edges
}

// sortedNodes returns all nodes in sorted order for deterministic processing.
func sortedNodes(g *DependencyGraph) []string {
	var nodes []string
	for node := range g.allNodes {
		nodes = append(nodes, node)
	}
	// Simple insertion sort for deterministic output
	for i := 1; i < len(nodes); i++ {
		for j := i; j > 0 && nodes[j] < nodes[j-1]; j-- {
			nodes[j], nodes[j-1] = nodes[j-1], nodes[j]
		}
	}
	return nodes
}
