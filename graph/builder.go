package graph

import "spring-circular-detector/parser"

// DependencyGraph represents the dependency graph of all beans.
type DependencyGraph struct {
	// nodes maps bean FQN -> set of bean FQNs it depends on
	edges map[string]map[string]bool
	// edgeInfo maps "from->to" -> descriptive string for reporting
	edgeInfo map[string]string
	// allNodes is the set of all nodes (beans + interface-only nodes)
	allNodes map[string]bool
	// interfaceImpls maps interface FQN -> set of implementing bean FQNs
	interfaceImpls map[string]map[string]bool
}

// NewGraph creates an empty dependency graph.
func NewGraph() *DependencyGraph {
	return &DependencyGraph{
		edges:          make(map[string]map[string]bool),
		edgeInfo:       make(map[string]string),
		allNodes:       make(map[string]bool),
		interfaceImpls: make(map[string]map[string]bool),
	}
}

// BuildGraph constructs the dependency graph from parsed beans.
func BuildGraph(beans map[string]*parser.BeanInfo) *DependencyGraph {
	g := NewGraph()

	// First pass: register all beans and their implemented interfaces
	for name, bean := range beans {
		g.allNodes[name] = true
		if g.edges[name] == nil {
			g.edges[name] = make(map[string]bool)
		}
		// Register interface implementations
		for _, iface := range bean.Interfaces {
			if g.interfaceImpls[iface] == nil {
				g.interfaceImpls[iface] = make(map[string]bool)
			}
			g.interfaceImpls[iface][name] = true
		}
	}

	// Second pass: add dependency edges
	for name, bean := range beans {
		for _, dep := range bean.Dependencies {
			targets := g.resolveTargets(dep.TypeName, beans)

			for _, target := range targets {
				g.addEdge(name, target)
				// Store edge info for reporting
				key := name + "->" + target
				ann := dep.Annotation
				if dep.IsDubboRef {
					ann = "@Reference(Dubbo)"
				}
				g.edgeInfo[key] = ann
			}

			// If this is a @Reference, also register the interface node
			if dep.IsDubboRef && dep.ReferenceIface != "" {
				g.allNodes[dep.ReferenceIface] = true
				if g.edges[dep.ReferenceIface] == nil {
					g.edges[dep.ReferenceIface] = make(map[string]bool)
				}
			}
		}
	}

	return g
}

// resolveTargets resolves a dependency type to concrete bean names.
// If the type is an interface, returns all implementations.
// If the type is a concrete class, returns that class.
func (g *DependencyGraph) resolveTargets(typeName string, beans map[string]*parser.BeanInfo) []string {
	// Check if it's a known interface with implementations
	if impls, ok := g.interfaceImpls[typeName]; ok && len(impls) > 0 {
		var result []string
		for impl := range impls {
			result = append(result, impl)
		}
		return result
	}

	// Check if it's a known concrete class
	if _, ok := beans[typeName]; ok {
		return []string{typeName}
	}

	// It might be a class not in our beans map (e.g., external dependency)
	// Register it as a leaf node (no outgoing edges)
	if !g.allNodes[typeName] {
		g.allNodes[typeName] = true
		if g.edges[typeName] == nil {
			g.edges[typeName] = make(map[string]bool)
		}
		return []string{typeName}
	}

	return nil
}

// addEdge adds a directed edge from -> to.
func (g *DependencyGraph) addEdge(from, to string) {
	g.allNodes[from] = true
	g.allNodes[to] = true
	if g.edges[from] == nil {
		g.edges[from] = make(map[string]bool)
	}
	g.edges[from][to] = true
}

// Edges returns the adjacency list.
func (g *DependencyGraph) Edges() map[string]map[string]bool {
	return g.edges
}

// EdgeInfo returns descriptive info for an edge.
func (g *DependencyGraph) EdgeInfo() map[string]string {
	return g.edgeInfo
}

// Nodes returns all nodes in the graph.
func (g *DependencyGraph) Nodes() map[string]bool {
	return g.allNodes
}
