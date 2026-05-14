package parser

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	springStereotypes = map[string]bool{
		"Component":     true,
		"Service":       true, // org.springframework.stereotype.Service
		"Repository":    true,
		"Controller":    true,
		"RestController": true,
	}
	dubboServiceImports = map[string]bool{
		"org.apache.dubbo.config.annotation.Service":     true,
		"com.alibaba.dubbo.config.annotation.Service":    true,
	}
	dubboReferenceImports = map[string]bool{
		"org.apache.dubbo.config.annotation.Reference":   true,
		"com.alibaba.dubbo.config.annotation.Reference":  true,
	}
	springAutowiredImports = map[string]bool{
		"org.springframework.beans.factory.annotation.Autowired": true,
	}
	javaxResourceImports = map[string]bool{
		"javax.annotation.Resource": true,
		"jakarta.annotation.Resource": true,
	}
	injectionAnnotations = map[string]bool{
		"Autowired": true,
		"Resource":  true,
		"Reference": true,
	}
	collectionTypes = map[string]bool{
		"List":       true,
		"Set":        true,
		"Collection": true,
		"Map":        true,
	}
	springPkgPrefixes = []string{
		"org.springframework.stereotype.",
		"org.springframework.beans.factory.annotation.",
	}
	dubboPkgPrefixes = []string{
		"org.apache.dubbo.config.annotation.",
		"com.alibaba.dubbo.config.annotation.",
	}
)

// ParseProject scans a directory recursively for .java files and parses them.
func ParseProject(rootDir string) (map[string]*BeanInfo, error) {
	beans := make(map[string]*BeanInfo)
	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".java") {
			return nil
		}
		bean, err := parseFile(path)
		if err != nil {
			// Skip files that can't be parsed; report but don't fail
			return nil
		}
		if bean != nil {
			beans[bean.Name] = bean
		}
		return nil
	})
	return beans, err
}

// parseFile parses a single Java source file.
func parseFile(path string) (*BeanInfo, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	src := string(content)
	clean := stripComments(src)
	lines := strings.Split(clean, "\n")

	pkg := extractPackage(lines)
	imports := extractImports(lines)
	className, classSimpleName, ifaces, classAnnotations, isAbstract := extractClassInfo(lines, pkg, imports)

	if className == "" {
		return nil, nil // not a class file we care about
	}

	// Determine if it's a Spring bean or Dubbo service
	isSpringBean := false
	isDubboService := false
	dubboIface := ""

	for _, ann := range classAnnotations {
		simple := shortName(ann)
		if springStereotypes[simple] && hasSpringImport(imports, ann) {
			isSpringBean = true
		}
		if simple == "Service" && hasDubboServiceImport(imports) {
			isDubboService = true
		}
		// Also support @DubboService (Apache Dubbo 3.x shorthand)
		if simple == "DubboService" {
			isDubboService = true
		}
	}

	// If the implements clause contains a Dubbo service interface, use it
	if isDubboService && len(ifaces) > 0 {
		dubboIface = ifaces[0] // the first implemented interface is usually the Dubbo service
	}

	// If class has Spring stereotype annotation, it's a Spring bean.
	// Also treat @Service with just dubbo import as a Spring bean (common pattern:
	// @Service is from Spring, @Service from Dubbo imported separately)
	// Actually, a class with Dubbo's @Service is often also a Spring bean.
	// We'll treat Dubbo services as potential beans too.
	isBean := isSpringBean || isDubboService
	if !isBean {
		return nil, nil // skip non-bean classes for dependency analysis
	}

	// Extract injection points
	deps := extractInjections(clean, lines, imports, pkg)

	bean := &BeanInfo{
		Name:           className,
		SimpleName:     classSimpleName,
		Package:        pkg,
		Interfaces:     ifaces,
		IsSpringBean:   isSpringBean,
		IsDubboService: isDubboService,
		DubboInterface: dubboIface,
		Dependencies:   deps,
		FilePath:       path,
		IsAbstract:     isAbstract,
	}

	return bean, nil
}

// stripComments removes Java comments from source code.
func stripComments(src string) string {
	var out strings.Builder
	out.Grow(len(src))
	i := 0
	n := len(src)
	for i < n {
		// Check for string literals - skip them
		if src[i] == '"' {
			out.WriteByte(src[i])
			i++
			for i < n {
				if src[i] == '\\' && i+1 < n {
					out.WriteByte(src[i])
					i++
					out.WriteByte(src[i])
					i++
				} else if src[i] == '"' {
					out.WriteByte(src[i])
					i++
					break
				} else {
					out.WriteByte(src[i])
					i++
				}
			}
			continue
		}
		// Check for char literals
		if src[i] == '\'' {
			out.WriteByte(src[i])
			i++
			if i < n && src[i] == '\\' && i+1 < n {
				out.WriteByte(src[i])
				i++
				out.WriteByte(src[i])
				i++
			} else if i < n {
				out.WriteByte(src[i])
				i++
			}
			if i < n && src[i] == '\'' {
				out.WriteByte(src[i])
				i++
			}
			continue
		}
		// Line comment
		if i+1 < n && src[i] == '/' && src[i+1] == '/' {
			i += 2
			for i < n && src[i] != '\n' {
				i++
			}
			out.WriteByte('\n') // preserve line count
			if i < n {
				i++ // skip newline
			}
			continue
		}
		// Block comment
		if i+1 < n && src[i] == '/' && src[i+1] == '*' {
			i += 2
			for i+1 < n && !(src[i] == '*' && src[i+1] == '/') {
				if src[i] == '\n' {
					out.WriteByte('\n') // preserve line count
				}
				i++
			}
			if i+1 < n {
				i += 2 // skip */
			}
			continue
		}
		out.WriteByte(src[i])
		i++
	}
	return out.String()
}

// extractPackage finds the package declaration.
func extractPackage(lines []string) string {
	re := regexp.MustCompile(`^\s*package\s+([\w.]+)\s*;`)
	for _, line := range lines {
		if m := re.FindStringSubmatch(line); m != nil {
			return m[1]
		}
	}
	return ""
}

// extractImports builds a map from simple class name to fully qualified name.
func extractImports(lines []string) map[string]string {
	re := regexp.MustCompile(`^\s*import\s+(static\s+)?([\w.]+)(?:\.\*)?\s*;`)
	imports := make(map[string]string)
	for _, line := range lines {
		if m := re.FindStringSubmatch(line); m != nil {
			fqn := m[2]
			if m[1] == "" { // non-static import
				parts := strings.Split(fqn, ".")
				simple := parts[len(parts)-1]
				imports[simple] = fqn
			}
		}
	}
	return imports
}

// extractClassInfo extracts class name, interfaces, class-level annotations.
func extractClassInfo(lines []string, pkg string, imports map[string]string) (fqn string, simpleName string, interfaces []string, annotations []string, isAbstract bool) {
	// Find the class/interface declaration line
	classRe := regexp.MustCompile(`^\s*(?:public\s+)?(?:abstract\s+)?(class|interface)\s+(\w+)`)
	// Annotations are on preceding lines
	annRe := regexp.MustCompile(`^\s*@(\w[\w.]*)\s*(?:\([^)]*\))?\s*$`)
	implementsRe := regexp.MustCompile(`\bimplements\s+([^{]+)`)

	var classIdx int = -1
	var classType string
	for i, line := range lines {
		if m := classRe.FindStringSubmatch(line); m != nil {
			classType = m[1]
			simpleName = m[2]
			classIdx = i
			// Check for extends/implements on the same line
			if implMatch := implementsRe.FindStringSubmatch(line); implMatch != nil {
				for _, iface := range splitTypes(implMatch[1]) {
					iface = strings.TrimSpace(iface)
					if iface != "" {
						interfaces = append(interfaces, resolveType(iface, imports, pkg))
					}
				}
			}
			break
		}
	}

	if classIdx < 0 {
		return "", "", nil, nil, false
	}

	// Check if class extends a parent and parent's interfaces are on next lines
	// "implements" might be on the next line
	for j := classIdx; j < len(lines) && j <= classIdx+1; j++ {
		line := lines[j]
		if implMatch := implementsRe.FindStringSubmatch(line); implMatch != nil {
			for _, iface := range splitTypes(implMatch[1]) {
				iface = strings.TrimSpace(iface)
				if iface != "" {
					ifaceFQN := resolveType(stripGenerics(iface), imports, pkg)
					if !containsStr(interfaces, ifaceFQN) {
						interfaces = append(interfaces, ifaceFQN)
					}
				}
			}
		}
	}

	// Look backward from class declaration for annotations
	for j := classIdx - 1; j >= 0; j-- {
		line := lines[j]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if m := annRe.FindStringSubmatch(trimmed); m != nil {
			annotations = append([]string{m[1]}, annotations...)
			continue
		}
		// Also handle annotations with content on the same line
		if strings.HasPrefix(trimmed, "@") && !strings.Contains(trimmed, "class ") && !strings.Contains(trimmed, "interface ") {
			annName := extractAnnotationName(trimmed)
			if annName != "" {
				annotations = append([]string{annName}, annotations...)
				continue
			}
		}
		break // stop at first non-annotation, non-empty line
	}

	isAbstract = (classType == "interface") || strings.Contains(lines[classIdx], "abstract")

	fqn = pkg + "." + simpleName
	if pkg == "" {
		fqn = simpleName
	}
	return fqn, simpleName, interfaces, annotations, isAbstract
}

// extractInjections finds all dependency injection points in a class.
func extractInjections(cleanSrc string, lines []string, imports map[string]string, pkg string) []Dependency {
	var deps []Dependency

	// Check imports to determine how to interpret annotations
	hasDubboRefImport := false
	for imp := range dubboReferenceImports {
		if _, ok := imports[shortName(imp)]; ok {
			if imports[shortName(imp)] == imp {
				hasDubboRefImport = true
				break
			}
		}
	}
	hasSpringAutowired := false
	for imp := range springAutowiredImports {
		if _, ok := imports[shortName(imp)]; ok {
			if imports[shortName(imp)] == imp {
				hasSpringAutowired = true
				break
			}
		}
	}

	// Find all annotation lines and the declaration that follows
	annRe := regexp.MustCompile(`^\s*@(\w[\w.]*)\s*(?:\([^)]*\))?$`)

	// Field injection: annotation followed by field declaration
	fieldRe := regexp.MustCompile(`^\s*(?:private|protected|public)\s+(\w+(?:<[^>]+>)?)\s+(\w+)\s*;`)

	// Constructor: @Autowired followed by constructor
	constructorRe := regexp.MustCompile(`^\s*(?:public|protected|private)?\s*(\w+)\s*\(([^)]*)\)`)

	// Setter: @Autowired followed by void setXxx method
	setterRe := regexp.MustCompile(`^\s*(?:public|protected|private)?\s*(?:static\s+)?(?:synchronized\s+)?(?:<[^>]+>\s+)?(\w+(?:<[^>]+>)?)\s+(\w+)\s*\(([^)]*)\)`)

	lineCount := len(lines)
	for i := 0; i < lineCount; i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Check if this line is an injection annotation
		annMatch := annRe.FindStringSubmatch(trimmed)
		if annMatch == nil {
			continue
		}
		annName := annMatch[1]
		annSimple := shortName(annName)

		if !injectionAnnotations[annSimple] {
			continue
		}

		// For @Reference, only count if Dubbo import exists
		if annSimple == "Reference" && !hasDubboRefImport {
			continue
		}
		// For @Autowired, only count if Spring import exists
		if annSimple == "Autowired" && !hasSpringAutowired {
			// Spring 4.3+ doesn't require explicit @Autowired import in some cases,
			// but typically it's still imported. We'll be lenient here.
			// Check if @Autowired is used without import - could be wildcard import
		}

		// Look ahead for the declaration (skip empty lines, skip other annotations)
		declIdx := -1
		for j := i + 1; j < lineCount && j <= i+5; j++ {
			t := strings.TrimSpace(lines[j])
			if t == "" {
				continue
			}
			if strings.HasPrefix(t, "@") {
				continue // skip chained annotations
			}
			declIdx = j
			break
		}
		if declIdx < 0 {
			continue
		}
		declLine := lines[declIdx]

		// Try field pattern first
		if fm := fieldRe.FindStringSubmatch(declLine); fm != nil {
			fieldType := fm[1]
			depType, refIface := resolveDepType(fieldType, imports, pkg, annSimple)
			isRef := annSimple == "Reference"
			deps = append(deps, Dependency{
				TypeName:       depType,
				Annotation:     annSimple,
				IsDubboRef:     isRef,
				ReferenceIface: refIface,
				Line:           declIdx + 1,
			})
			i = declIdx
			continue
		}

		// Try constructor pattern
		if cm := constructorRe.FindStringSubmatch(declLine); cm != nil {
			// Extract parameter types from constructor
			paramStr := cm[2]
			if paramStr != "" {
				params := splitParams(paramStr)
				for _, param := range params {
					param = strings.TrimSpace(param)
					parts := strings.Fields(param)
					if len(parts) >= 1 {
						paramType := stripGenerics(parts[0])
						depType, refIface := resolveDepType(paramType, imports, pkg, annSimple)
						isRef := annSimple == "Reference"
						deps = append(deps, Dependency{
							TypeName:       depType,
							Annotation:     annSimple,
							IsDubboRef:     isRef,
							ReferenceIface: refIface,
							Line:           declIdx + 1,
						})
					}
				}
			}
			i = declIdx
			continue
		}

		// Try setter method pattern
		if sm := setterRe.FindStringSubmatch(declLine); sm != nil {
			paramStr := sm[3]
			if paramStr != "" {
				params := splitParams(paramStr)
				for _, param := range params {
					param = strings.TrimSpace(param)
					parts := strings.Fields(param)
					if len(parts) >= 1 {
						paramType := stripGenerics(parts[0])
						depType, refIface := resolveDepType(paramType, imports, pkg, annSimple)
						isRef := annSimple == "Reference"
						deps = append(deps, Dependency{
							TypeName:       depType,
							Annotation:     annSimple,
							IsDubboRef:     isRef,
							ReferenceIface: refIface,
							Line:           declIdx + 1,
						})
					}
				}
			}
			i = declIdx
			continue
		}
	}

	// Also find single-constructor implicit injection (Spring 4.3+)
	// A class with exactly one non-private constructor gets implicit @Autowired
	deps = append(deps, findImplicitConstructorInjection(cleanSrc, lines, imports, pkg)...)

	return deps
}

// findImplicitConstructorInjection finds constructors that don't have @Autowired
// but are the only constructor in the class (Spring 4.3+ implicit injection).
func findImplicitConstructorInjection(_ string, lines []string, imports map[string]string, pkg string) []Dependency {
	var deps []Dependency
	constructorRe := regexp.MustCompile(`^\s*(?:public|protected)\s+(\w+)\s*\(([^)]*)\)`)

	// Find class name first
	classRe := regexp.MustCompile(`^\s*(?:public\s+)?(?:abstract\s+)?(class|interface)\s+(\w+)`)
	var className string
	for _, line := range lines {
		if m := classRe.FindStringSubmatch(line); m != nil {
			className = m[2]
			break
		}
	}
	if className == "" {
		return nil
	}

	// Find non-private constructors
	type ctorInfo struct {
		line       string
		params     string
		hasAutowired bool
	}
	var constructors []ctorInfo

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Check if previous meaningful line has @Autowired
		hasAnn := false
		for j := i - 1; j >= 0 && j >= i-3; j-- {
			t := strings.TrimSpace(lines[j])
			if t == "" {
				continue
			}
			if strings.Contains(t, "@Autowired") {
				hasAnn = true
			}
			break
		}

		if m := constructorRe.FindStringSubmatch(trimmed); m != nil {
			if m[1] == className {
				constructors = append(constructors, ctorInfo{
					line:   trimmed,
					params: m[2],
					hasAutowired: hasAnn,
				})
			}
		}
	}

	// Single non-private constructor without @Autowired → implicit injection
	nonAutowiredCtors := 0
	for _, c := range constructors {
		if !c.hasAutowired {
			nonAutowiredCtors++
		}
	}
	if nonAutowiredCtors != 1 {
		return nil
	}

	for _, c := range constructors {
		if c.hasAutowired {
			continue
		}
		if c.params != "" {
			params := splitParams(c.params)
			for _, param := range params {
				param = strings.TrimSpace(param)
				parts := strings.Fields(param)
				if len(parts) >= 1 {
					paramType := stripGenerics(parts[0])
					depType, _ := resolveDepType(paramType, imports, pkg, "Autowired")
					deps = append(deps, Dependency{
						TypeName:   depType,
						Annotation: "Autowired",
						Line:       0,
					})
				}
			}
		}
	}
	return deps
}

// resolveDepType resolves a dependency type from the field/parameter type.
// Returns the dependency type FQN, and for @Reference, the referenced interface FQN.
func resolveDepType(fieldType string, imports map[string]string, pkg string, annName string) (depType string, refIface string) {
	// Strip generic parameters for resolution
	baseType := stripGenerics(fieldType)
	// Extract generic type argument for collection types
	innerType := extractFirstGenericArg(fieldType)

	resolved := resolveType(baseType, imports, pkg)

	if annName == "Reference" {
		// For @Reference, the field type IS the Dubbo interface
		return resolved, resolved
	}

	// For collection types, the actual dependency is the generic parameter
	if collectionTypes[baseType] && innerType != "" {
		innerResolved := resolveType(innerType, imports, pkg)
		if innerResolved != "" {
			return innerResolved, ""
		}
	}

	return resolved, ""
}

// resolveType resolves a short type name to fully qualified using imports.
func resolveType(shortName string, imports map[string]string, pkg string) string {
	shortName = stripGenerics(strings.TrimSpace(shortName))
	if shortName == "" {
		return ""
	}
	// Already fully qualified
	if strings.Contains(shortName, ".") {
		return shortName
	}
	// Check imports
	if fqn, ok := imports[shortName]; ok {
		return fqn
	}
	// Same package
	if pkg != "" {
		return pkg + "." + shortName
	}
	return shortName
}

// splitTypes splits a comma-separated list of type names, respecting nested generics.
func splitTypes(s string) []string {
	s = strings.TrimSpace(s)
	var result []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			depth--
		case ',':
			if depth == 0 {
				result = append(result, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	result = append(result, strings.TrimSpace(s[start:]))
	return result
}

// splitParams splits constructor/method parameters, respecting nested generics.
func splitParams(s string) []string {
	return splitTypes(s)
}

// stripGenerics removes generic type parameters from a type name.
func stripGenerics(s string) string {
	idx := strings.Index(s, "<")
	if idx < 0 {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(s[:idx])
}

// extractFirstGenericArg extracts the first generic type argument.
func extractFirstGenericArg(s string) string {
	start := strings.Index(s, "<")
	if start < 0 {
		return ""
	}
	end := strings.LastIndex(s, ">")
	if end < 0 || end <= start {
		return ""
	}
	inner := s[start+1 : end]
	// Take first argument before comma (respecting nested generics)
	parts := splitTypes(inner)
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(inner)
}

// shortName extracts the simple class name from a fully qualified name.
func shortName(fqn string) string {
	parts := strings.Split(fqn, ".")
	return parts[len(parts)-1]
}

// extractAnnotationName extracts the annotation name from an annotation line.
func extractAnnotationName(line string) string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "@") {
		return ""
	}
	// Remove @ prefix
	name := line[1:]
	// Remove parameters
	if idx := strings.Index(name, "("); idx >= 0 {
		name = name[:idx]
	}
	return name
}

// hasSpringImport checks if the annotation is from a Spring package.
func hasSpringImport(imports map[string]string, annName string) bool {
	simple := shortName(annName)
	if fqn, ok := imports[simple]; ok {
		for _, prefix := range springPkgPrefixes {
			if strings.HasPrefix(fqn, prefix) {
				return true
			}
		}
	}
	// If no import is found, it might be a wildcard import or same-package.
	// For common Spring annotations, assume Spring if simple name matches.
	if springStereotypes[simple] {
		return true // heuristic: common Spring annotations
	}
	return false
}

// hasDubboServiceImport checks if Dubbo @Service is imported.
func hasDubboServiceImport(imports map[string]string) bool {
	for imp := range dubboServiceImports {
		if _, ok := imports[shortName(imp)]; ok {
			return true
		}
	}
	return false
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
