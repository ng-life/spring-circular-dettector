# spring-circular-detector

用 Go 语言编写的静态分析工具，用于检测 Java Spring 项目中的**循环依赖**和**Dubbo @Reference 使用不当**问题。

## 项目结构

```
spring-circular-detector/
├── main.go                    # 入口
├── checker/
│   ├── circular.go            # 循环依赖检测
│   └── dubbo_reference.go     # Dubbo @Reference 误用检测
├── parser/
│   └── parser.go              # Java 源码解析
├── graph/
│   ├── builder.go             # 依赖图构建
│   └── cycle.go               # DFS 环检测算法
└── report/
    └── report.go              # 结果输出
```

## 各模块功能

### parser/parser.go — Java 源码解析

逐文件解析 Java 源码，识别以下内容：

- **Spring Bean 声明**：通过 `@Component`、`@Service`、`@Repository`、`@Controller`、`@RestController` 注解识别
- **Dubbo 服务声明**：通过 `org.apache.dubbo.config.annotation.Service` 或 `com.alibaba.dubbo.config.annotation.Service` 及 `@DubboService` 识别
- **依赖注入点**：解析三种注入方式——
  - **字段注入**：`@Autowired` / `@Resource` / `@Reference` 标注的字段
  - **构造器注入**：标注 `@Autowired` 的构造器，以及 Spring 4.3+ 的隐式单构造器注入
  - **Setter 注入**：标注相关注解的 setter 方法

注释剥离逻辑 (`stripComments`) 为手工实现，能正确处理字符串/字符字面量中的注释符号，避免误判。

### graph/builder.go — 依赖图构建

将所有解析出的 Bean 构建成有向图：
- 节点 = Bean 的全限定名（FQN）+ Dubbo 接口
- 边 = 依赖注入关系，通过接口实现关系解析具体目标（接口 → 所有实现类）

### graph/cycle.go — DFS 环检测

使用经典的**三色 DFS 算法**（白-灰-黑）检测有向图中的环：
- 遇到灰节点（仍在递归栈中）即发现一条回边，表示存在环
- 通过 `canonicalPath` 去重（按字典序最小起点表达环，避免同一环重复报告）

### checker/circular.go — 循环依赖检测

串联解析 → 建图 → 找环的流程，返回所有检测到的循环依赖。

### checker/dubbo_reference.go — Dubbo @Reference 误用检测

检查是否存在用 `@Reference` 引用了**本项目自身对外暴露的 Dubbo 服务**的情况。这种用法会引入不必要的 RPC 开销，应该改为本地注入。

### report/report.go — 结果输出

格式化输出两种检测结果，并附带文件路径、行号等定位信息。

## 执行流程

```
main.go
  ├── checker.CheckCircular(rootDir)
  │     ├── parser.ParseProject(rootDir)   → 解析所有 .java 文件
  │     ├── graph.BuildGraph(beans)        → 构建依赖图
  │     └── graph.FindCycles(g)            → DFS 找环
  │
  ├── checker.CheckDubboReference(rootDir)
  │     ├── parser.ParseProject(rootDir)   → 解析所有 .java 文件（第二次）
  │     └── 对照本地 Dubbo 服务检查 @Reference
  │
  └── report.PrintResults(cycles, refIssues) → 输出结果
```

工具以 `os.Exit(1)` 退出表示发现问题，适合 CI 集成。

## 使用方式

```bash
# 基本用法
spring-circular-detector -path /path/to/java-project/src

# 带详细输出
spring-circular-detector -path /path/to/java-project/src -v
```

## 亮点

- **独立的 Java 解析器**：不依赖 Java 编译器，纯文本正则解析，适合 CI 快速扫描
- **支持多种注入模式**：字段、构造器、Setter、隐式构造器注入全覆盖
- **Dubbo 感知**：能区分 Spring 的 `@Service` 和 Dubbo 的 `@Service`，正确处理 `@Reference`
- **去重机制**：环检测使用规范路径去重，避免同一环被重复报告
- **适合 CI 集成**：用退出码区分有无问题，输出格式清晰可读
