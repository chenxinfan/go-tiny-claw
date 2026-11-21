package main

import (
	"context"
	"log"
	"os"

	"github.com/yourname/go-tiny-claw/internal/config"
	"github.com/yourname/go-tiny-claw/internal/engine"
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

	// 课程原版是 provider.NewZhipuOpenAIProvider("glm-4.5-air")，这里换成 DeepSeek；
	// 两者都是 OpenAI 兼容接口，所以只是换了 base URL / key / 模型名。
	// 模型可用 DEEPSEEK_MODEL 覆盖（默认 deepseek-chat，换成 deepseek-reasoner 即思考模式）。
	model := provider.DeepSeekModel("")
	log.Printf("[Config] DeepSeek base URL = %s, model = %s", provider.DeepSeekBaseURL(), model)

	llmProvider := provider.NewDeepSeekProvider(model)

	// 第 08 章：引擎支持并发执行同一轮里的多个工具调用。
	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool(workDir))
	registry.Register(tools.NewWriteFileTool(workDir))
	registry.Register(tools.NewBashTool(workDir))
	registry.Register(tools.NewEditFileTool(workDir))

	// 课程这里开慢思考来促使模型一次规划出多个并行工具调用；
	// 但 DeepSeek 在 Phase 1（无工具）会把调用意图吐成 <tool_call>/DSML 文本，
	// 所以本项目默认关闭，靠 prompt 里「一次性读取」的措辞驱动并发。
	eng := engine.NewAgentEngine(llmProvider, registry, workDir, false)

	prompt := `
	我当前目录下有 a.txt, b.txt, c.txt 三个文件。(如果没有请忽略找不到的报错)
	为了节省时间，请你同时一次性利用工具读取这三个文件，并将它们的内容综合起来告诉我。
	`

	err := eng.Run(context.Background(), prompt)
	if err != nil {
		log.Fatalf("引擎运行崩溃: %v", err)
	}
}
