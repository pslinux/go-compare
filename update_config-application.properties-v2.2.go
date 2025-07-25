// root@inco71:~/go# cat update_config-application.properties.go
// ... 其他代码保持不变 ...

// 在文件顶部添加编译指令
//go:build linux

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	backupDir     = "./config_backup"
	lineSeparator = "\n"
	tmpSuffix     = ".tmp"
	bufferSize    = 64 * 1024 // 64KB buffer
	configFile    = "config-matcher.json"
	version       = "1.1.0"
	buildDate     = "2023-11-20"
)

// Config 定义配置文件结构
type Config struct {
	PatternKeys string `json:"patternKeys"`
}

// loadConfig 加载配置文件
func loadConfig() (string, error) {
	// 默认配置
	defaultPattern := `^(spring\.datasource|spring\.redis|web\.back\.upLoadPath|web\.front\.upLoadPath|token\.expireTime|ftp.userName|ftp.passWord|ftp.host|ftp.port|ftp.baseUrl|ftp.LocalDir|inco.system.xxmc|inco.system.maintitle|inco.person.xxdm|inco.security.login.checkcode)`

	// 检查配置文件是否存在
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if verbose {
			logger.Printf("配置文件 %s 不存在，使用默认匹配规则", configFile)
		}
		return defaultPattern, nil
	}

	// 读取配置文件
	file, err := os.ReadFile(configFile)
	if err != nil {
		return "", fmt.Errorf("读取配置文件失败: %w", err)
	}

	var config Config
	if err := json.Unmarshal(file, &config); err != nil {
		return "", fmt.Errorf("解析配置文件失败: %w", err)
	}

	if config.PatternKeys == "" {
		if verbose {
			logger.Printf("配置文件中未定义patternKeys，使用默认匹配规则")
		}
		return defaultPattern, nil
	}

	if verbose {
		logger.Printf("从配置文件 %s 加载匹配规则", configFile)
	}
	return config.PatternKeys, nil
}

var (
	verbose     bool
	showVersion bool
	logger      = log.New(os.Stderr, "", log.LstdFlags)
)

func main() {
	flag.BoolVar(&verbose, "v", false, "启用详细输出模式")
	flag.BoolVar(&showVersion, "version", false, "显示版本信息")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "配置文件更新工具 v%s (构建日期: %s)\n", version, buildDate)
		fmt.Fprintf(flag.CommandLine.Output(), "用法: %s [选项] 旧配置文件路径 新配置文件路径\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "选项:")
		flag.PrintDefaults()
		fmt.Fprintln(flag.CommandLine.Output(), "\n示例:")
		fmt.Fprintf(flag.CommandLine.Output(), "  %s old.properties new.properties\n", os.Args[0])
		fmt.Fprintf(flag.CommandLine.Output(), "  %s -v old.properties new.properties\n", os.Args[0])
	}
	flag.Parse()

	if showVersion {
		fmt.Printf("配置文件更新工具 v%s\n", version)
		fmt.Printf("构建日期: %s\n", buildDate)
		os.Exit(0)
	}

	if flag.NArg() < 2 {
		flag.Usage()
		os.Exit(1)
	}

	oldFile := flag.Arg(0)
	newFile := flag.Arg(1)

	if verbose {
		logger.Printf("开始处理文件: 旧文件=%s, 新文件=%s", oldFile, newFile)
	}

	// 创建备份目录
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		logger.Fatalf("创建备份目录失败: %v", err)
	}

	// 生成备份文件
	ts := time.Now().Format("20060102150405")
	if verbose {
		logger.Printf("创建备份文件...")
	}
	if err := backupFile(oldFile, filepath.Join(backupDir, filepath.Base(oldFile)+".bak."+ts)); err != nil {
		logger.Fatalf("备份旧文件失败: %v", err)
	}
	if err := backupFile(newFile, filepath.Join(backupDir, filepath.Base(newFile)+".new.bak."+ts)); err != nil {
		logger.Fatalf("备份新文件失败: %v", err)
	}

	// 步骤1：提取保留参数
	if verbose {
		logger.Printf("从旧文件中提取保留参数...")
	}
	keepParams, err := extractKeepParams(oldFile)
	if err != nil {
		logger.Fatalf("提取保留参数失败: %v", err)
	}

	// 步骤2：更新新文件
	if verbose {
		logger.Printf("更新新文件...")
	}
	if err := updateNewFile(newFile, keepParams); err != nil {
		logger.Fatalf("更新新文件失败: %v", err)
	}

	fmt.Println("配置更新完成!已完全使用新文件内容,并保留以下参数在原位置:")
	printMatchedParams(newFile)

	if verbose {
		logger.Printf("处理完成")
	}
}

func backupFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("打开源文件失败: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("创建目标文件失败: %w", err)
	}
	defer dstFile.Close()

	buf := make([]byte, bufferSize)
	if _, err := io.CopyBuffer(dstFile, srcFile, buf); err != nil {
		return fmt.Errorf("复制文件内容失败: %w", err)
	}

	if verbose {
		logger.Printf("成功创建备份文件: %s", dst)
	}
	return nil
}

func extractKeepParams(filename string) (map[int]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	pattern, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("加载配置失败: %w", err)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("编译正则表达式失败: %w", err)
	}

	keepParams := make(map[int]string)
	scanner := bufio.NewScanner(file)
	lineNum := 1

	if verbose {
		logger.Printf("开始扫描文件: %s", filename)
		logger.Printf("使用匹配规则: %s", pattern)
	}

	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			keepParams[lineNum] = strings.TrimSuffix(line, "\r")
			if verbose {
				logger.Printf("找到匹配参数[行%d]: %s", lineNum, line)
			}
		}
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("扫描文件失败: %w", err)
	}

	if verbose {
		logger.Printf("共找到%d个需要保留的参数", len(keepParams))
	}
	return keepParams, nil
}

func updateNewFile(filename string, keepParams map[int]string) error {
	// 读取新文件内容
	lines, err := readLines(filename)
	if err != nil {
		return fmt.Errorf("读取新文件失败: %w", err)
	}

	if verbose {
		logger.Printf("开始更新文件: %s (共%d行)", filename, len(lines))
	}

	// 应用保留参数
	for oldLineNum, oldLine := range keepParams {
		key := strings.SplitN(oldLine, "=", 2)[0]
		newLineNum := findKeyInLines(lines, key)

		if newLineNum != -1 {
			if verbose {
				logger.Printf("替换参数[行%d]: %s", newLineNum+1, key)
			}
			lines[newLineNum] = oldLine
		} else {
			if oldLineNum <= len(lines) {
				if verbose {
					logger.Printf("插入参数[行%d]: %s", oldLineNum, key)
				}
				lines = insertLine(lines, oldLineNum-1, oldLine)
			} else {
				if verbose {
					logger.Printf("追加参数[行%d]: %s", len(lines)+1, key)
				}
				lines = append(lines, oldLine)
			}
		}
	}

	// 写入更新后的文件
	if err := writeLines(filename, lines); err != nil {
		return fmt.Errorf("写入更新文件失败: %w", err)
	}

	if verbose {
		logger.Printf("文件更新完成，共处理%d个参数", len(keepParams))
	}
	return nil
}

func readLines(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	buf := make([]byte, 0, bufferSize)
	scanner.Buffer(buf, bufferSize)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	if verbose {
		logger.Printf("读取文件完成: %s (共%d行)", filename, len(lines))
	}
	return lines, nil
}

func writeLines(filename string, lines []string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("创建文件失败: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriterSize(file, bufferSize)
	for _, line := range lines {
		if _, err := writer.WriteString(line + lineSeparator); err != nil {
			return fmt.Errorf("写入文件失败: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("刷新缓冲区失败: %w", err)
	}

	if verbose {
		logger.Printf("写入文件完成: %s (共%d行)", filename, len(lines))
	}
	return nil
}

func findKeyInLines(lines []string, key string) int {
	if len(lines) == 0 {
		return -1
	}

	pattern := `^\s*` + regexp.QuoteMeta(key) + `\s*=`
	re, err := regexp.Compile(pattern)
	if err != nil {
		if verbose {
			logger.Printf("警告: 编译正则表达式失败: %v", err)
		}
		return -1
	}

	for i, line := range lines {
		if re.MatchString(line) {
			if verbose {
				logger.Printf("在行%d找到键: %s", i+1, key)
			}
			return i
		}
	}

	if verbose {
		logger.Printf("未找到键: %s", key)
	}
	return -1
}

func insertLine(lines []string, index int, line string) []string {
	if index < 0 {
		index = 0
	} else if index > len(lines) {
		index = len(lines)
	}

	if verbose {
		logger.Printf("在位置%d插入新行", index+1)
	}

	// 更安全的插入方式，避免潜在的切片问题
	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:index]...)
	result = append(result, line)
	result = append(result, lines[index:]...)
	return result
}

func printMatchedParams(filename string) {
	pattern, err := loadConfig()
	if err != nil {
		logger.Printf("警告: 加载匹配规则失败: %v", err)
		return
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		logger.Printf("警告: 编译正则表达式失败: %v", err)
		return
	}

	file, err := os.Open(filename)
	if err != nil {
		logger.Printf("警告: 无法打开文件显示匹配参数: %v", err)
		return
	}
	defer file.Close()

	if verbose {
		logger.Printf("开始显示匹配参数...")
		logger.Printf("使用匹配规则: %s", pattern)
	}

	scanner := bufio.NewScanner(file)
	lineNum := 1
	matchedCount := 0

	fmt.Println("\n匹配的参数列表:")
	fmt.Println("----------------------------")
	for scanner.Scan() {
		line := scanner.Text()
		if re.MatchString(line) {
			fmt.Printf("%4d: %s\n", lineNum, line)
			matchedCount++
		}
		lineNum++
	}

	if err := scanner.Err(); err != nil {
		logger.Printf("警告: 扫描文件失败: %v", err)
	}

	fmt.Println("----------------------------")
	fmt.Printf("共找到 %d 个匹配参数\n", matchedCount)

	if verbose {
		logger.Printf("显示匹配参数完成")
	}
}
