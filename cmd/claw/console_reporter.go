package main

import (
	"context"
	"fmt"
	"log"
)

// consoleReporter 把 Agent 的执行过程打到终端。
// 第 09 章把「展示」从 engine 里抽成了 Reporter 接口，飞书机器人是一种实现，
// 这里补一个终端实现，这样没有飞书凭证时也能本地跑。
type consoleReporter struct{}

func newConsoleReporter() *consoleReporter { return &consoleReporter{} }

func (r *consoleReporter) OnThinking(ctx context.Context) {
	log.Println("[Reporter] 🤔 模型正在慢思考 (Thinking)...")
}

func (r *consoleReporter) OnToolCall(ctx context.Context, toolName string, args string) {
	log.Printf("[Reporter] 🛠️ 调用工具: %s, 参数: %s\n", toolName, args)
}

func (r *consoleReporter) OnToolResult(ctx context.Context, toolName string, result string, isError bool) {
	status := "✅ 成功"
	if isError {
		status = "⚠️ 报错"
	}
	fmt.Printf("[Reporter] %s (%s): %s\n", status, toolName, result)
}

func (r *consoleReporter) OnMessage(ctx context.Context, content string) {
	fmt.Printf("🤖 [对外回复]: %s\n", content)
}
