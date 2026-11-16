# go-tiny-claw

跟着极客时间《从 0 开始构建 Agent Harness》课程，用 Go 手写一个迷你 Agent Harness（对标 Claude Code）。

当前进度：**第 05 章 · 真实工具与工具注册表**。

## 章节进度

| 章节 | 内容 | 状态 |
| --- | --- | --- |
| 01 | 工程骨架：`main()` 里用 TODO 占位四大模块 | ✅ |
| 02 | 四个核心接口 + 两轮 ReAct 循环（Mock 模型/工具） | ✅ |
| 03 | 每轮拆成 Thinking / Action 两阶段 | ✅ |
| 04 | 真实模型接入：Claude / OpenAI 兼容 Provider + 真实工具调用 | ✅（本项目用 DeepSeek） |
| 05 | 真实工具：`BaseTool` 接口 + 可注册的 Registry + `read_file` | ✅ |

## 第 01 章 · 工程骨架

不写逻辑，先把「大脑 / 手脚 / 记忆 / 心脏」四个模块在 `main()` 里按装配顺序留成 TODO。

## 第 02 章 · 接口抽象与主循环

| 模块 | 文件 | 职责 |
| --- | --- | --- |
| 消息 schema | `internal/schema/message.go` | `Message` / `ToolCall` / `ToolResult` / `ToolDefinition` |
| 大脑 | `internal/provider/interface.go` | `LLMProvider.Generate(ctx, messages, tools)` |
| 手脚 | `internal/tools/registry.go` | `Registry`：工具清单 + 执行 |
| 心脏 | `internal/engine/loop.go` | `AgentEngine.Run()`：调模型 → 有工具调用就执行回填 → 没有就退出 |

## 第 03 章 · 慢思考模式

每轮拆两阶段，靠 `EnableThinking` 控制：Phase 1 用 `Generate(ctx, history, nil)` 剥夺工具强制规划，
Phase 2 挂回工具再行动。

## 第 04 章 · 接入真实模型

课程给了两个真实 Provider，都实现了同一个 `LLMProvider` 接口：

| 文件 | 说明 |
| --- | --- |
| `internal/provider/claude.go` | Anthropic 官方 SDK（`anthropic-sdk-go`） |
| `internal/provider/openai.go` | OpenAI 官方 SDK（`openai-go/v3`），构造时用 `WithBaseURL` 指向智谱的兼容接口 |
| `internal/provider/deepseek.go` | **本项目新增**：复用 `openai.go` 的转换逻辑，只换 base URL / key / 模型名 |

### 为什么换 DeepSeek 只需要加一个 20 行的构造函数

DeepSeek 提供 OpenAI 完全兼容的 Chat Completions 接口，所以 `schema.Message` ↔ OpenAI 消息的转换、
`ToolDefinition` ↔ `tools` 数组的转换、`ToolCall` 的解析，全都不用重写——`NewDeepSeekProvider`
只是把 client 的 `baseURL`、`apiKey`、`model` 换成 DeepSeek 的，返回的还是同一个
`*OpenAIProvider`，`Generate` 方法一行没动：

```go
llmProvider := provider.NewDeepSeekProvider("deepseek-chat")
```

| 配置项 | 环境变量 | 默认值 |
| --- | --- | --- |
| API Key（必填） | `DEEPSEEK_API_KEY` | 无，缺失时 `log.Fatal` 直接退出 |
| 接口地址 | `DEEPSEEK_BASE_URL` | `https://api.deepseek.com/v1/` |
| 模型 | `DEEPSEEK_MODEL` | `deepseek-chat`（可换 `deepseek-reasoner`） |

`main.go` 里的 `mockRegistry` 换成真实工具是第 05 章的事，这一章工具仍是 mock 的
`get_weather`，用来验证「模型真的会按 JSON Schema 请求工具、拿到结果再作答」这条链路。

> 课程原文用智谱 GLM，所以 `openai.go` 里 `NewZhipuOpenAIProvider` 和错误信息中的
> 「OpenAI/Zhipu」字样保持原样未改；本项目实际调用 DeepSeek 时，报错前缀会显示成
> 「OpenAI/Zhipu API 请求失败」，这是共用代码路径导致的，不是接错了。

## 第 05 章 · 真实工具与工具注册表

第 04 章的工具还是 `main.go` 里手写的 `mockRegistry`；这一章把它换成真正的工具系统：

| 文件 | 内容 |
| --- | --- |
| `internal/tools/registry.go` | `BaseTool` 接口（`Name()` / `Definition()` / `Execute()`）+ `Registry` 接口 + `registryImpl`（map 存工具，按名字派发） |
| `internal/tools/read_file.go` | 第一个真实工具：按相对路径读文件，超过 8000 字节自动截断 |
| `hello.txt` | 给 `read_file` 用的测试文件 |

关键设计是**工具自己声明自己**：每个工具提供 `Definition()` 返回 JSON Schema，Registry 收集起来喂给模型，
模型按 schema 填参数，Registry 再按 `call.Name` 找回对应工具执行。这样加新工具只需要写一个 struct + 一行
`registry.Register(...)`，不用改 engine 和 provider。

`main.go` 里也从 `mockRegistry` 换成了：

```go
registry := tools.NewRegistry()
registry.Register(tools.NewReadFileTool(workDir))
```

## 目录结构

```
go-tiny-claw/
├── .env.example                  # 环境变量模板，复制成 .env 使用（.env 已被忽略）
├── hello.txt                     # read_file 工具的测试文本
├── go.mod / go.sum               # 带 anthropic-sdk-go、openai-go/v3 依赖
├── cmd/claw/main.go              # 用 DeepSeek Provider + 真实工具注册表驱动循环
└── internal/
    ├── config/dotenv.go          # 极简 .env 加载器（本项目新增，无第三方依赖）
    ├── engine/loop.go            # 两阶段 ReAct 循环
    ├── provider/
    │   ├── interface.go          # LLMProvider 接口
    │   ├── claude.go             # Anthropic 实现（课程原版，未被调用）
    │   ├── openai.go             # OpenAI 兼容实现（课程原版 + 智谱构造器）
    │   └── deepseek.go           # DeepSeek 构造器（本项目新增）
    ├── schema/message.go
    └── tools/
        ├── registry.go           # BaseTool / Registry / registryImpl
        └── read_file.go          # 读取文件工具
```

## 运行

### 1. 配置环境变量

推荐用 `.env`，一次性配好，之后 `go run` / IDE 里点运行都能读到：

```powershell
Copy-Item .env.example .env      # 或手动新建 .env
notepad .env                     # 填上 DEEPSEEK_API_KEY=sk-...
```

`.env` 已被 `.gitignore` 忽略，不会进仓库；`.env.example` 会进仓库，当模板用。

加载规则（`internal/config/dotenv.go`）：

- 已存在的环境变量优先，`.env` 不会覆盖它们（临时 `$env:` 仍然管用）；
- 文件不存在不算错误；单行写错只会跳过这一行并打印 `[Config] 已跳过 .env 第 N 行…`，其它变量照常生效；
- 支持空行、`#` 或 `;` 注释、`export` 前缀、值外层成对的引号。

不想用文件也行，直接给进程设环境变量：

```powershell
$env:DEEPSEEK_API_KEY = "sk-..."        # 当前窗口有效
setx DEEPSEEK_API_KEY "sk-..."          # 长期生效，需重开窗口
```

可选配置（`.env` 或环境变量都可以）：

```
DEEPSEEK_MODEL=deepseek-reasoner                  # 换思考模式模型
DEEPSEEK_BASE_URL=https://api.deepseek.com/v1/    # 自建代理/网关时用
```

启动时会打印实际生效的配置（不含 key）：

```
[Config] 已从 .env 加载 2 个环境变量
[Config] DeepSeek base URL = https://api.deepseek.com/v1/, model = deepseek-chat
```

### 2. 构建与运行

```bash
go build ./...
go vet ./...
go run ./cmd/claw
```

预期：模型先请求 `read_file(path=hello.txt)`，Registry 派发给真实文件工具读取 `hello.txt`，
结果回填后模型用一句话总结，然后因为没有新的工具调用而退出：

```
[Registry] 成功挂载工具: read_file
========== [Turn 1] 开始 ==========
[Engine] 模型请求调用 1 个工具...
  -> 🛠️ 执行工具: read_file, 参数: {"path": "hello.txt"}
  -> ✅ 工具执行成功 (返回 120 字节)
========== [Turn 2] 开始 ==========
🤖 [对外回复]: 文件内容是一句问候：它自称是来自物理文件系统的一段"神秘文本"……
[Engine] 模型未请求调用工具，任务宣告完成。
```

### Go 版本

`go.mod` 保持了课程写的 `go 1.25.0`。本机装的是 Go 1.22.4，`GOTOOLCHAIN=auto` 会自动下载并使用
1.25.0 工具链（首次需要联网，之后有缓存）。如果处在离线环境，把 `go.mod` 改回 `go 1.22` 也能编译
（依赖本身要求的最低版本低于 1.22）。

### 如果编译报 `go-build ... Access is denied`

当前环境不允许写全局构建缓存（例如在受限沙箱里运行）。把缓存指到项目内：

```powershell
$env:GOCACHE="$PWD\.gocache"; go run ./cmd/claw
```

`.gocache/` 已加入 `.gitignore`。

## 排错：模型有时不调用工具

**现象**：同一句「帮我查查北京天气」，有时会调 `get_weather`，有时直接凭记忆写一大段建议，
还说「如果你能装个联网查天气的工具…」，明明工具就在清单里。

**实测数据**（同一 prompt 反复跑）：

| 配置 | 调到工具的轮次 |
| --- | --- |
| `EnableThinking = true`（慢思考两阶段） | 1 / 4 |
| `EnableThinking = false`（课程默认单阶段） | 9 / 9 |

**根因主要是慢思考模式**，不是模型坏了：

Phase 1 用 `Generate(ctx, history, nil)` 把工具清空，模型只能输出一段「我打算调用 get_weather…」的规划；
这段规划又被追加进历史。到了 Phase 2，模型看着自己刚写完的计划，经常直接给结论、不再真的发起调用。
把 `EnableThinking` 关掉（`main.go` 里第四个参数传 `false`），调用立刻稳定。

次要旋钮（慢思考打开时才明显）：

| 旋钮 | 位置 | 做法 |
| --- | --- | --- |
| 温度 | `internal/provider/deepseek.go` | 给 client 加 `option.WithJSONSet("temperature", 0.2)`，DeepSeek 默认 1.0 太随机 |
| 提示词 | `internal/engine/loop.go` 的 system message | 加一条 Tool policy：实时/外部事实必须先调工具，禁止凭记忆回答 |

两者都上之后，即使 `EnableThinking = true` 也能回到 6/6。更硬的一档是 `tool_choice: "required"`
（首轮强制调工具），代价是纯聊天也被迫调一次，所以真实 harness 通常是三者组合使用。

## 下一章

第 06 章补齐工具集：新增 `internal/tools/bash.go`、`internal/tools/write_file.go`，
把「读 / 写 / 执行命令」三件套都注册进去，Agent 才算真的能动手。
