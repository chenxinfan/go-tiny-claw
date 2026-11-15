// internal/provider/deepseek.go
package provider

import (
	"os"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

// DeepSeek 提供与 OpenAI 完全兼容的 Chat Completions 接口，
// 所以这里不需要新写一套转换逻辑：直接复用 openai.go 里的 OpenAIProvider。
const (
	// DefaultDeepSeekBaseURL 用 /v1/ 结尾的兼容路径（OpenAI SDK 会在此后拼接 chat/completions）。
	DefaultDeepSeekBaseURL = "https://api.deepseek.com/v1/"
	// DefaultDeepSeekModel deepseek-chat 对应 V3 系列；要用推理模型可传 deepseek-reasoner。
	DefaultDeepSeekModel = "deepseek-chat"
)

// DeepSeekBaseURL 返回实际生效的接口地址：优先读 DEEPSEEK_BASE_URL，否则用官方兼容地址。
func DeepSeekBaseURL() string {
	if v := os.Getenv("DEEPSEEK_BASE_URL"); v != "" {
		return v
	}
	return DefaultDeepSeekBaseURL
}

// DeepSeekModel 返回实际生效的模型名：优先用传入值，其次 DEEPSEEK_MODEL，最后默认 deepseek-chat。
func DeepSeekModel(model string) string {
	if model != "" {
		return model
	}
	if v := os.Getenv("DEEPSEEK_MODEL"); v != "" {
		return v
	}
	return DefaultDeepSeekModel
}

// NewDeepSeekProvider 用 DeepSeek 的 OpenAI 兼容 API 构造 Provider。
// 与 NewZhipuOpenAIProvider 的差别只有三处：API Key 环境变量、base URL、默认模型。
func NewDeepSeekProvider(model string) *OpenAIProvider {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		panic("请设置 DEEPSEEK_API_KEY 环境变量")
	}

	return &OpenAIProvider{
		client: openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(DeepSeekBaseURL())),
		model:  DeepSeekModel(model),
	}
}
