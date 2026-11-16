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

	// 第 05 章：工具注册表从「接口 + mock」变成真的注册表，挂上 read_file。
	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool(workDir))

	// 慢思考模式默认关闭：DeepSeek 在 Phase 1（无工具）会把工具调用意图吐成
	// <tool_call> / DSML 这种原始文本，既费 token 又干扰阅读。想体验就改成 true。
	eng := engine.NewAgentEngine(llmProvider, registry, workDir, false)

	prompt := "请调用工具读取一下当前工作区目录下 hello.txt 文件的内容，并用一句话向我总结它说了什么。"

	err := eng.Run(context.Background(), prompt)
	if err != nil {
		log.Fatalf("引擎运行崩溃: %v", err)
	}
}
