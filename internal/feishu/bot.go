package feishu

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
	"github.com/yourname/go-tiny-claw/internal/engine"

	lark "github.com/larksuite/oapi-sdk-go/v3"
)

type FeishuBot struct {
	client    *lark.Client
	appID     string
	appSecret string
	engine    *engine.AgentEngine
}

func NewFeishuBot(eng *engine.AgentEngine) *FeishuBot {
	appID := os.Getenv("FEISHU_APP_ID")
	appSecret := os.Getenv("FEISHU_APP_SECRET")

	if appID == "" || appSecret == "" {
		log.Fatal("请设置 FEISHU_APP_ID 和 FEISHU_APP_SECRET")
	}

	client := lark.NewClient(appID, appSecret)

	return &FeishuBot{
		client:    client,
		appID:     appID,
		appSecret: appSecret,
		engine:    eng,
	}
}

// GetEventDispatcher 用于 Webhook 模式：按开放平台的加密策略决定是否解密事件信封。
func (b *FeishuBot) GetEventDispatcher() *dispatcher.EventDispatcher {
	return b.buildDispatcher(os.Getenv("FEISHU_ENCRYPT_KEY"))
}

// GetLongConnDispatcher 用于长连接模式：事件走 WebSocket 帧，不再有加密信封，
// 所以 encryptKey 必须传空，否则 dispatcher 会去找 body 里的 "encrypt" 字段并报错。
func (b *FeishuBot) GetLongConnDispatcher() *dispatcher.EventDispatcher {
	return b.buildDispatcher("")
}

// StartLongConnection 用 WebSocket 长连接接收事件，不需要公网回调地址，也不需要 URL 校验。
func (b *FeishuBot) StartLongConnection(ctx context.Context) error {
	cli := larkws.NewClient(b.appID, b.appSecret, larkws.WithEventHandler(b.GetLongConnDispatcher()))
	return cli.Start(ctx)
}

func (b *FeishuBot) buildDispatcher(encryptKey string) *dispatcher.EventDispatcher {
	verifyToken := os.Getenv("FEISHU_VERIFY_TOKEN")

	handler := dispatcher.NewEventDispatcher(verifyToken, encryptKey).
		OnP2MessageReceiveV1(func(ctx context.Context, event *larkim.P2MessageReceiveV1) error {
			contentStr := *event.Event.Message.Content
			contentStr = strings.TrimPrefix(contentStr, `{"text":"`)
			contentStr = strings.TrimSuffix(contentStr, `"}`)

			chatId := *event.Event.Message.ChatId
			log.Printf("[Feishu] 收到会话 %s 消息: %s\n", chatId, contentStr)

			go b.handleAgentRun(chatId, contentStr)

			return nil
		}).
		OnP2MessageReadV1(func(ctx context.Context, event *larkim.P2MessageReadV1) error {
			// 消息已读事件，静默忽略
			return nil
		})

	return handler
}

func (b *FeishuBot) handleAgentRun(chatId string, prompt string) {
	reporter := &FeishuReporter{
		client: b.client,
		chatId: chatId,
	}

	err := b.engine.Run(context.Background(), prompt, reporter)
	if err != nil {
		reporter.sendMsg(fmt.Sprintf("❌ Agent 运行崩溃: %v", err))
	}
}

type FeishuReporter struct {
	client *lark.Client
	chatId string
}

func (r *FeishuReporter) sendMsg(text string) {
	// Build text message content
	textContent := map[string]string{
		"text": text,
	}
	contentBytes, _ := json.Marshal(textContent)
	contentStr := string(contentBytes)

	msgReq := larkim.NewCreateMessageReqBuilder().
		ReceiveIdType(larkim.ReceiveIdTypeChatId).
		Body(larkim.NewCreateMessageReqBodyBuilder().
			ReceiveId(r.chatId).
			MsgType(larkim.MsgTypeText).
			Content(contentStr).
			Build()).
		Build()

	_, _ = r.client.Im.Message.Create(context.Background(), msgReq)
}

func (r *FeishuReporter) OnThinking(ctx context.Context) {
	r.sendMsg("🤔 模型正在慢思考 (Thinking)...")
}

func (r *FeishuReporter) OnToolCall(ctx context.Context, toolName string, args string) {
	r.sendMsg(fmt.Sprintf("🛠️ **正在执行工具**：`%s`\n参数：`%s`", toolName, args))
}

func (r *FeishuReporter) OnToolResult(ctx context.Context, toolName string, result string, isError bool) {
	if isError {
		r.sendMsg(fmt.Sprintf("⚠️ **执行报错** (%s)：\n%s", toolName, result))
	} else {
		r.sendMsg(fmt.Sprintf("✅ **执行成功** (%s)", toolName))
	}
}

func (r *FeishuReporter) OnMessage(ctx context.Context, content string) {
	r.sendMsg(content)
}

var _ engine.Reporter = (*FeishuReporter)(nil)
