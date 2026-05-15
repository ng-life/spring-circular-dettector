# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## 项目概述

用 Go 1.21 编写的 CLI 静态分析工具，用于检测 Java Spring 项目中的循环依赖和 Dubbo `@Reference` 使用不当问题。零外部依赖，纯正则解析 `.java` 文件，无需 Java 编译器。

## 构建与运行

```bash
# 构建
go build -o spring-circular-detector

# 运行
./spring-circular-detector -path /path/to/java/src

# 详细模式（打印每个 Bean 的信息）
./spring-circular-detector -path /path/to/java/src -v

# 测试（当前无测试文件）
go test ./...
```

## 架构

```
main.go                     # CLI 入口，flag 解析，编排两个 checker
  ├── checker/circular.go   # 串联：解析 → 建图 → DFS 找环
  ├── checker/dubbo_reference.go  # 解析 → 交叉比对 @Reference 与本地 Dubbo 服务
  ├── parser/parser.go      # 正则驱动的 Java 源码解析器
  │     types.go            # BeanInfo, Dependency, CycleInfo, ReferenceIssue
  ├── graph/builder.go      # 有向图构建（接口 → 实现类展开）
  │     cycle.go            # 三色 DFS 环检测 + 规范路径去重
  └── report/report.go      # 中文格式化输出，退出码 1 表示发现问题
```

## 关键设计细节

### 解析器 (parser/parser.go)
- **注释剥离**：`stripComments` 是手写状态机，正确处理字符串/字符字面量中的注释符号（`//`、`/*`）。用等量换行符替换注释以保留行号。
- **注入点识别**：支持字段注入、构造器注入（含 Spring 4.3+ 隐式单构造器注入）、Setter 注入。
- **注解区分**：`@Service` 需要区分 Spring 的 `org.springframework.stereotype.Service` 和 Dubbo 的 `org.apache.dubbo.config.annotation.Service` / `com.alibaba.dubbo.config.annotation.Service`。
- **类型解析**：通过检查 `import` 语句将短名映射为 FQN。对 `@Reference` 字段，字段类型本身即 Dubbo 接口。

### 图构建 (graph/builder.go)
- Bean 之间的边按接口展开：如果依赖目标是接口，则连线到所有实现该接口的 Bean。
- 项目中不存在的类型作为叶子节点添加到图中。
- 边信息（`edgeInfo`）记录了注入方式（`@Autowired` / `@Reference` 等），用于报告时描述边。

### 环检测 (graph/cycle.go)
- 经典三色 DFS（白/灰/黑），遇到灰节点即发现环。
- `canonicalPath` 通过字典序最小起点对环归一化，确保同一环只报告一次。
- 节点按排序顺序遍历以保证输出确定性。

### Dubbo @Reference 检查 (checker/dubbo_reference.go)
- 如果 `@Reference` 引用的 Dubbo 接口恰好是项目自身对外暴露的服务，则报告为不当使用（应改为本地注入，避免不必要的 RPC 开销）。

### 已知问题
- 项目在 `CheckCircular` 和 `CheckDubboReference` 中各调用一次 `ParseProject`，存在重复解析开销。
- 当前无单元测试。
- `-v` 模式的 Bean 概览输出未完全实现（`main.go:58-61` 有骨架代码）。
