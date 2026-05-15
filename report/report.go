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
		fmt.Println("未发现问题。")
		return
	}

	if len(cycles) > 0 {
		fmt.Printf("\n=== 循环依赖（共 %d 处）===\n\n", len(cycles))
		for i, cycle := range cycles {
			fmt.Printf("循环 %d:\n", i+1)
			fmt.Printf("  %s\n", buildChain(cycle.Edges))
			fmt.Println()
		}
	}

	if len(refIssues) > 0 {
		fmt.Printf("=== @Reference 使用不当（共 %d 处）===\n\n", len(refIssues))
		for i, issue := range refIssues {
			fmt.Printf("问题 %d:\n", i+1)
			fmt.Printf("  位置:         %s\n", relativePath(issue.FilePath, issue.Line))
			fmt.Printf("  类:           %s\n", issue.ClassName)
			fmt.Printf("  引用的 Dubbo 接口: %s\n", issue.ReferencedIface)
			fmt.Printf("  该接口的本地实现:   %s\n", issue.LocalImpl)
			fmt.Printf("  说明:  @Reference 引用了本应用自身提供的 Dubbo 服务。\n")
			fmt.Printf("         这会引入不必要的 RPC 开销，应改为本地注入。\n")
			fmt.Println()
		}
	}
}

// PrintBeanSummary outputs parsed bean information for debugging.
func PrintBeanSummary(beans map[string]*parser.BeanInfo) {
	fmt.Printf("解析到 %d 个 Bean:\n\n", len(beans))
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
			fmt.Printf("    实现接口: %v\n", bean.Interfaces)
		}
		if len(bean.Dependencies) > 0 {
			for _, dep := range bean.Dependencies {
				annotation := dep.Annotation
				if dep.IsDubboRef {
					annotation = "@Reference(Dubbo)"
				}
				fmt.Printf("    %s -> %s", annotation, dep.TypeName)
				if dep.Line > 0 {
					fmt.Printf(" (第 %d 行)", dep.Line)
				}
				fmt.Println()
			}
		}
		fmt.Println()
	}
}

// buildChain joins edges into a single chain: A --[Resource]--> B --[Autowired]--> C --[注入]--> A
func buildChain(edges []string) string {
	if len(edges) == 0 {
		return ""
	}
	// First edge "A --[Resource]--> B" → parts = ["A", "Resource]--> B"]
	parts := strings.SplitN(edges[0], " --[", 2)
	chain := parts[0]
	for _, edge := range edges {
		parts := strings.SplitN(edge, " --[", 2)
		if len(parts) == 2 {
			chain += " --[" + parts[1]
		}
	}
	return chain
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
