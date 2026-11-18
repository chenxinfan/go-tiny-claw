package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/yourname/go-tiny-claw/internal/schema"
)

type BashTool struct {
	workDir string
}

func NewBashTool(workDir string) *BashTool {
	return &BashTool{workDir: workDir}
}

func (t *BashTool) Name() string {
	return "bash"
}

func (t *BashTool) Definition() schema.ToolDefinition {
	return schema.ToolDefinition{
		Name: t.Name(),
		Description: "在当前工作区执行命令，返回标准输出(stdout)和标准错误(stderr)。" +
			"支持链式命令(如 &&)。" + shellHint(),
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "要执行的命令",
				},
			},
			"required": []string{"command"},
		},
	}
}

// shellHint 告诉模型当前机器上实际用哪个 shell，避免它写出平台不兼容的命令。
func shellHint() string {
	if runtime.GOOS == "windows" {
		return "当前运行环境是 Windows，命令由 cmd.exe 执行：请使用 Windows 命令（dir、type 等），不要用 ls、cat 这类 Unix 命令。"
	}
	return "当前运行环境是类 Unix，命令由 bash -c 执行。"
}

type bashArgs struct {
	Command string `json:"command"`
}

// newShellCmd 按平台选择执行方式：
//   - Windows: cmd /c，并先把控制台代码页切到 UTF-8(65001)，否则中文输出会变成乱码；
//   - 其它: bash -c。
func (t *BashTool) newShellCmd(ctx context.Context, command string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", "/c", "chcp 65001>nul & "+command)
	}
	return exec.CommandContext(ctx, "bash", "-c", command)
}

func (t *BashTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	var input bashArgs
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := t.newShellCmd(timeoutCtx, input.Command)
	cmd.Dir = t.workDir

	out, err := cmd.CombinedOutput()
	outputStr := string(out)

	if timeoutCtx.Err() == context.DeadlineExceeded {
		return outputStr + "\n[警告: 命令执行超时(30s)，已被系统强制终止。]", nil
	}

	if err != nil {
		return fmt.Sprintf("执行报错: %v\n输出:\n%s", err, outputStr), nil
	}

	if outputStr == "" {
		return "命令执行成功，无终端输出。", nil
	}

	const maxLen = 8000
	if len(outputStr) > maxLen {
		return fmt.Sprintf("%s\n\n...[终端输出过长，已截断至前 %d 字节]...", outputStr[:maxLen], maxLen), nil
	}

	return outputStr, nil
}
