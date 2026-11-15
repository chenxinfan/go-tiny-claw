// internal/config/dotenv.go
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Result 汇总一次 .env 加载的结果，方便调用方把问题打印出来而不是启动失败。
type Result struct {
	// Loaded 是本次真正注入进程环境的变量个数（已存在的不算）。
	Loaded int
	// Skipped 是格式不合法、被跳过的行，形如「第 3 行：不是 key=value 形式」。
	Skipped []string
}

// LoadDotEnv 把 .env 里的 key=value 注入当前进程的环境变量。
//
// 约定（刻意保持简单，不引入第三方库）：
//   - 文件不存在不算错误，返回空结果；
//   - 已存在的环境变量优先，不会被 .env 覆盖（所以临时 export 仍然有用）；
//   - 空行、# 或 ; 开头的注释、export 前缀会被忽略；
//   - 单行写错只跳过这一行并记录到 Skipped，不影响其它变量，也不阻止启动；
//   - 值外层成对的单/双引号会被去掉；值里的 # 不做行内注释处理。
func LoadDotEnv(path string) (Result, error) {
	var result Result

	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return result, nil
		}
		return result, fmt.Errorf("打开 %s 失败: %w", path, err)
	}
	defer file.Close()

	lineNo := 0
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			result.Skipped = append(result.Skipped, fmt.Sprintf("第 %d 行不是 key=value 形式", lineNo))
			continue
		}

		key = strings.TrimSpace(key)
		value = unquote(strings.TrimSpace(value))

		if !validKey(key) {
			result.Skipped = append(result.Skipped, fmt.Sprintf("第 %d 行的变量名 %q 非法", lineNo, key))
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			result.Skipped = append(result.Skipped, fmt.Sprintf("第 %d 行设置 %s 失败: %v", lineNo, key, err))
			continue
		}
		result.Loaded++
	}

	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	return result, nil
}

func unquote(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

func validKey(key string) bool {
	if key == "" {
		return false
	}
	for i, r := range key {
		switch {
		case r == '_', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
			// 合法
		case r >= '0' && r <= '9' && i > 0:
			// 合法
		default:
			return false
		}
	}
	return true
}
