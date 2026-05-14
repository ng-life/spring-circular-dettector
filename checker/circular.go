package checker

import (
	"spring-circular-detector/graph"
	"spring-circular-detector/parser"
)

// CheckCircular parses the project and finds all circular dependencies.
func CheckCircular(rootDir string) ([]graph.Cycle, error) {
	beans, err := parser.ParseProject(rootDir)
	if err != nil {
		return nil, err
	}

	g := graph.BuildGraph(beans)
	cycles := graph.FindCycles(g)
	return cycles, nil
}
