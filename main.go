package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"spring-circular-detector/checker"
	"spring-circular-detector/report"
)

func main() {
	path := flag.String("path", "", "Path to Java project source directory")
	verbose := flag.Bool("v", false, "Verbose: print parsed bean information")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: %s -path <java项目源码目录> [-v]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nSpring 循环依赖 & Dubbo @Reference 检查工具\n")
		fmt.Fprintf(os.Stderr, "\n检查项:\n")
		fmt.Fprintf(os.Stderr, "  1. 循环依赖（包括 @Reference 链）\n")
		fmt.Fprintf(os.Stderr, "  2. @Reference 使用不当（引用了本地 Dubbo 服务）\n")
		fmt.Fprintf(os.Stderr, "\n参数:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *path == "" {
		flag.Usage()
		os.Exit(1)
	}

	// Verify path exists
	info, err := os.Stat(*path)
	if err != nil {
		log.Fatalf("路径不存在: %s", *path)
	}
	if !info.IsDir() {
		log.Fatalf("路径不是目录: %s", *path)
	}

	fmt.Printf("分析目录: %s\n\n", *path)

	// Check circular dependencies
	fmt.Println("正在检查循环依赖...")
	cycles, err := checker.CheckCircular(*path)
	if err != nil {
		log.Fatalf("检查循环依赖失败: %v", err)
	}

	// Check improper @Reference usage
	fmt.Println("正在检查 @Reference 使用...")
	refIssues, err := checker.CheckDubboReference(*path)
	if err != nil {
		log.Fatalf("检查 @Reference 使用失败: %v", err)
	}

	if *verbose {
		// Re-parse for verbose output (simple approach)
		fmt.Println("\n=== Bean 概览 ===")
	}

	report.PrintResults(cycles, refIssues)

	if len(cycles) == 0 && len(refIssues) == 0 {
		os.Exit(0)
	}
	os.Exit(1)
}
