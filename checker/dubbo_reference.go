package checker

import (
	"spring-circular-detector/parser"
)

// CheckDubboReference finds improper @Reference usage — when a @Reference
// references a Dubbo interface that the same application provides locally.
func CheckDubboReference(rootDir string) ([]parser.ReferenceIssue, error) {
	beans, err := parser.ParseProject(rootDir)
	if err != nil {
		return nil, err
	}

	// Collect all Dubbo service interfaces provided by this project
	dubboServices := make(map[string]string) // interface FQN -> implementation class FQN
	for name, bean := range beans {
		if bean.IsDubboService && bean.DubboInterface != "" {
			dubboServices[bean.DubboInterface] = name
		}
	}

	// Check all @Reference injections
	var issues []parser.ReferenceIssue
	for _, bean := range beans {
		for _, dep := range bean.Dependencies {
			if !dep.IsDubboRef {
				continue
			}
			// dep.ReferenceIface is the Dubbo interface being referenced
			if localImpl, ok := dubboServices[dep.ReferenceIface]; ok {
				issues = append(issues, parser.ReferenceIssue{
					ClassName:       bean.Name,
					ReferencedIface: dep.ReferenceIface,
					LocalImpl:       localImpl,
					FilePath:        bean.FilePath,
					Line:            dep.Line,
				})
			}
		}
	}

	return issues, nil
}
