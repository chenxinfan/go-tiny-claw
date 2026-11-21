package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/larksuite/oapi-sdk-go/v3/core/httpserverext"
	"github.com/yourname/go-tiny-claw/internal/config"
	"github.com/yourname/go-tiny-claw/internal/engine"
	"github.com/yourname/go-tiny-claw/internal/feishu"
	"github.com/yourname/go-tiny-claw/internal/provider"
	"github.com/yourname/go-tiny-claw/internal/tools"
)

func main() {
	// 本地开发把 key 写进 .env 即可（.env 已被 .gitignore 忽略），不必每次手动 export。
	// 已存在的环境变量优先，.env 不会覆盖它们；单行写错只会被跳过，不影响启动。
	if res, err := config.LoadDotEnv(".env"); err != nil {
		log.Printf("[Config] 读取 .env 失败: %v", err)
	} else {
		for _, skipped := range res.Skipped {
			log.Printf("[Config] 已跳过 .env %s", skipped)
		}
		if res.Loaded > 0 {
			log.Printf("[Config] 已从 .env 加载 %d 个环境变量", res.Loaded)
		}
	}

	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		log.Fatal("请先导出 DEEPSEEK_API_KEY 环境变量")
	}

	workDir, _ := os.Getwd()

	// 课程原版是 provider.NewZhipuOpenAIProvider("glm-4.5-air")，这里换成 DeepSeek。
	model := provider.DeepSeekModel("")
	log.Printf("[Config] DeepSeek base URL = %s, model = %s", provider.DeepSeekBaseURL(), model)

	llmProvider := provider.NewDeepSeekProvider(model)

	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool(workDir))
	registry.Register(tools.NewWriteFileTool(workDir))
	registry.Register(tools.NewBashTool(workDir))
	registry.Register(tools.NewEditFileTool(workDir))

	// 慢思考默认关闭（DeepSeek 在无工具阶段会吐 <tool_call> 文本）。
	eng := engine.NewAgentEngine(llmProvider, registry, workDir, false)

	// 有飞书凭证就走飞书模式，否则本地命令行跑一个 demo。
	// 飞书模式默认用「长连接」(WebSocket)：本地开发不需要公网回调地址，
	// 也不需要在开放平台做 URL 校验。设 FEISHU_MODE=webhook 则回到课程的 HTTP 服务模式。
	if os.Getenv("FEISHU_APP_ID") != "" {
		if os.Getenv("FEISHU_MODE") == "webhook" {
			runFeishuServer(eng)
		} else {
			runFeishuLongConnection(eng)
		}
		return
	}

	runConsoleDemo(eng)
}

// runFeishuLongConnection 用 WebSocket 长连接收事件，适合本机 / 内网开发。
func runFeishuLongConnection(eng *engine.AgentEngine) {
	bot := feishu.NewFeishuBot(eng)
	log.Println("🚀 go-tiny-claw 已通过飞书长连接启动，请在飞书里给机器人发消息")

	if err := bot.StartLongConnection(context.Background()); err != nil {
		log.Fatalf("长连接启动失败: %v", err)
	}
}

// runFeishuServer 是课程第 09 章的主体：起一个 HTTP 服务接飞书事件回调。
func runFeishuServer(eng *engine.AgentEngine) {
	bot := feishu.NewFeishuBot(eng)
	handler := httpserverext.NewEventHandlerFunc(bot.GetEventDispatcher())

	http.HandleFunc("/webhook/event", handler)

	port := ":48080"
	log.Printf("🚀 go-tiny-claw 飞书服务端已启动，正在监听 %s 端口\n", port)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}

// runConsoleDemo 没有飞书凭证时的本地路径：同一个 engine + 终端 Reporter。
func runConsoleDemo(eng *engine.AgentEngine) {
	prompt := `
	我当前目录下有 a.txt, b.txt, c.txt 三个文件。(如果没有请忽略找不到的报错)
	为了节省时间，请你同时一次性利用工具读取这三个文件，并将它们的内容综合起来告诉我。
	`

	if err := eng.Run(context.Background(), prompt, newConsoleReporter()); err != nil {
		log.Fatalf("引擎运行崩溃: %v", err)
	}
}
