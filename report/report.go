package report

import (
	"fmt"
	"os"
	"strings"

	"spring-circular-detector/graph"
	"spring-circular-detector/parser"
)

// PrintResults outputs the analysis results to stdout.
func PrintResults(cycles []graph.Cycle, refIssues []parser.ReferenceIssue) {
	if len(cycles) == 0 && len(refIssues) == 0 {
		fmt.Println("No issues found.")
		return
	}

	if len(cycles) > 0 {
		fmt.Printf("\n=== Circular Dependencies (%d found) ===\n\n", len(cycles))
		for i, cycle := range cycles {
			fmt.Printf("Cycle %d:\n", i+1)
			for _, edge := range cycle.Edges {
				fmt.Printf("  %s\n", edge)
			}
			fmt.Println()
		}
	}

	if len(refIssues) > 0 {
		fmt.Printf("=== Improper @Reference Usage (%d found) ===\n\n", len(refIssues))
		for i, issue := range refIssues {
			fmt.Printf("Issue %d:\n", i+1)
			fmt.Printf("  Location: %s\n", relativePath(issue.FilePath, issue.Line))
			fmt.Printf("  Class:    %s\n", issue.ClassName)
			fmt.Printf("  References Dubbo interface: %s\n", issue.ReferencedIface)
			fmt.Printf("  Which is locally implemented by: %s\n", issue.LocalImpl)
			fmt.Printf("  Problem:  @Reference references a service provided by this same application.\n")
			fmt.Printf("            This introduces unnecessary RPC overhead. Use local injection instead.\n")
			fmt.Println()
		}
	}
}

// PrintBeanSummary outputs parsed bean information for debugging.
func PrintBeanSummary(beans map[string]*parser.BeanInfo) {
	fmt.Printf("Parsed %d beans:\n\n", len(beans))
	for name, bean := range beans {
		tags := []string{}
		if bean.IsSpringBean {
			tags = append(tags, "Spring")
		}
		if bean.IsDubboService {
			tags = append(tags, "DubboService")
		}

		fmt.Printf("  %s [%s]\n", name, strings.Join(tags, ", "))
		if len(bean.Interfaces) > 0 {
			fmt.Printf("    Implements: %v\n", bean.Interfaces)
		}
		if len(bean.Dependencies) > 0 {
			for _, dep := range bean.Dependencies {
				annotation := dep.Annotation
				if dep.IsDubboRef {
					annotation = "@Reference(Dubbo)"
				}
				fmt.Printf("    %s -> %s", annotation, dep.TypeName)
				if dep.Line > 0 {
					fmt.Printf(" (line %d)", dep.Line)
				}
				fmt.Println()
			}
		}
		fmt.Println()
	}
}

func relativePath(filePath string, line int) string {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Sprintf("%s:%d", filePath, line)
	}
	rel, err := filepathRel(cwd, filePath)
	if err != nil {
		return fmt.Sprintf("%s:%d", filePath, line)
	}
	return fmt.Sprintf("%s:%d", rel, line)
}

func filepathRel(base, target string) (string, error) {
	// Simple relative path computation
	base = strings.TrimSuffix(base, "/")
	target = strings.TrimSuffix(target, "/")

	baseParts := strings.Split(base, "/")
	targetParts := strings.Split(target, "/")

	// Find common prefix
	i := 0
	for i < len(baseParts) && i < len(targetParts) && baseParts[i] == targetParts[i] {
		i++
	}

	var result strings.Builder
	// Go up from base
	for j := i; j < len(baseParts); j++ {
		if result.Len() > 0 {
			result.WriteString("/")
		}
		result.WriteString("..")
	}
	// Go down to target
	for j := i; j < len(targetParts); j++ {
		if result.Len() > 0 {
			result.WriteString("/")
		}
		result.WriteString(targetParts[j])
	}
	return result.String(), nil
}
