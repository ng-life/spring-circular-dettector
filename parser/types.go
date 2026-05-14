package parser

// BeanInfo represents a parsed Java class that may be a Spring bean.
type BeanInfo struct {
	Name            string       // fully qualified class name
	SimpleName      string       // simple class name
	Package         string       // package name
	Interfaces      []string     // fully qualified implemented interface names
	IsSpringBean    bool         // has @Component/@Service/@Repository/@Controller/@RestController
	IsDubboService  bool         // has org.apache.dubbo.config.annotation.Service or com.alibaba.dubbo.config.annotation.Service
	DubboInterface  string       // the interface this Dubbo service exposes (fully qualified)
	Dependencies    []Dependency // injection points
	FilePath        string       // source file path
	IsAbstract      bool         // abstract class or interface
}

// Dependency represents one injection point in a bean.
type Dependency struct {
	TypeName        string // fully qualified type name of the injected dependency
	Annotation      string // @Autowired, @Resource, or @Reference
	IsDubboRef      bool   // true if injected via Dubbo @Reference
	ReferenceIface  string // if Dubbo @Reference, the referenced interface FQN
	Line            int    // source line number (1-based)
}

// CycleInfo describes a detected circular dependency.
type CycleInfo struct {
	Path  []string // ordered list of bean names forming the cycle
	Edges []string // description of each edge in the cycle (annotation + line info)
}

// ReferenceIssue describes an improper @Reference usage.
type ReferenceIssue struct {
	ClassName        string // class containing the @Reference
	FieldName        string // field name (if field injection)
	ReferencedIface  string // the Dubbo interface being referenced
	LocalImpl        string // local implementation class that provides this service
	FilePath         string
	Line             int
}
