# go-tiny-claw

跟着极客时间《从 0 开始构建 Agent Harness》课程，用 Go 手写一个迷你 Agent Harness（对标 Claude Code）。

当前进度：**第 08 章 · 并发执行工具调用**。

## 章节进度

| 章节 | 内容 | 状态 |
| --- | --- | --- |
| 01 | 工程骨架：`main()` 里用 TODO 占位四大模块 | ✅ |
| 02 | 四个核心接口 + 两轮 ReAct 循环（Mock 模型/工具） | ✅ |
| 03 | 每轮拆成 Thinking / Action 两阶段 | ✅ |
| 04 | 真实模型接入：Claude / OpenAI 兼容 Provider + 真实工具调用 | ✅（本项目用 DeepSeek） |
| 05 | 真实工具：`BaseTool` 接口 + 可注册的 Registry + `read_file` | ✅ |
| 06 | 工具集补齐：`write_file` + `bash`，Agent 能真改文件、真跑命令 | ✅（Windows 适配） |
| 07 | `edit_file`：按 old_text 做局部替换，四层模糊匹配兜底 | ✅ |
| 08 | 同一轮里的多个工具调用改并发执行（goroutine + WaitGroup） | ✅ |

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

## 第 06 章 · 读 / 写 / 执行三件套

第 05 章只有一只「眼睛」，这一章把「手」也装上：

| 工具 | 文件 | 能力 |
| --- | --- | --- |
| `read_file` | `internal/tools/read_file.go` | 按相对路径读文件，8000 字节截断 |
| `write_file` | `internal/tools/write_file.go` | 创建/覆盖文件，父目录自动创建 |
| `bash` | `internal/tools/bash.go` | 在工作区执行命令，30s 超时，output 8000 字节截断 |

`main.go` 里三行注册：

```go
registry.Register(tools.NewReadFileTool(workDir))
registry.Register(tools.NewWriteFileTool(workDir))
registry.Register(tools.NewBashTool(workDir))
```

课程示例任务就是端到端验证：查 Go 版本 → 写 `helloworld.go` → 编译运行。四轮循环、三次工具调用，
全程无需人工干预。

### Windows 适配（课程原版是 `bash -c`）

课程原文用 `exec.Command("bash", "-c", cmd)`，在 Windows 上不一定有 bash，而且中文输出会乱码。
本项目按平台分流：

```go
if runtime.GOOS == "windows" {
    return exec.CommandContext(ctx, "cmd", "/c", "chcp 65001>nul & "+command)
}
return exec.CommandContext(ctx, "bash", "-c", command)
```

两个细节：

1. **用 `cmd.exe` 而不是 PowerShell**：`cmd` 支持 `&&` 链式命令，和课程描述一致；PowerShell 5.1 不支持；
2. **先 `chcp 65001`**：把子进程控制台切到 UTF-8，否则 `dir` 之类命令输出的中文会变成乱码。

工具的 `Description()` 也会按平台提示模型（Windows 下说「用 dir/type，不要用 ls/cat」），
避免模型写出平台不兼容的命令——这是 harness 在给模型「喂环境信息」的又一个例子。

## 第 07 章 · edit_file：让 Agent 安全地改代码

`write_file` 是整文件覆盖，让 Agent 改代码很危险：模型只见过文件的一部分，重写整份就会丢掉它没看见的内容。
`edit_file` 用「旧串 → 新串」的局部替换解决这个问题：

```json
{"path": "server.go", "old_text": "\tif true {...}", "new_text": "\tif user == nil {...}"}
```

难点在于**模型给出的 `old_text` 往往和文件里的字节不完全一致**（缩进、换行、首尾空白）。
所以 `fuzzyReplace` 做了四层降级匹配：

| 层级 | 策略 | 适用场景 |
| --- | --- | --- |
| L1 | 精确 `strings.Count` | 完全一致，最理想 |
| L2 | CRLF → LF 归一化 | **Windows 检出文件带 `\r\n`，模型传 `\n`** |
| L3 | 去掉首尾空白再匹配 | 模型漏了开头/结尾的空行 |
| L4 | 逐行比较、忽略每行缩进 | 模型把 tab 写成了空格 |

任何一层都要求**恰好命中一处**：命中 0 处报「未找到」，命中多处报「请提供更多上下文」。
这个「唯一性约束」很关键——如果模型给的片段在文件里出现两次，宁可失败也不要乱改。

L2 那层在 Windows 上尤其重要：本仓库 `core.autocrlf=true`，检出的文件是 CRLF，
而模型在 JSON 里写的换行是 `\n`，不归一化就永远匹配不上。

## 第 08 章 · 并发执行工具调用

第 07 章之前，一轮里多个工具调用是**串行**的：读完 a.txt 再读 b.txt。模型其实可以在一次响应里给出多个
`tool_calls`，Harness 完全可以并行执行它们 —— 这一章就是把 `for` 循环换成 goroutine + WaitGroup：

```go
observationMsgs := make([]schema.Message, len(actionResp.ToolCalls)) // 预分配，按下标写
var wg sync.WaitGroup
for i, call := range actionResp.ToolCalls {
    wg.Add(1)
    go func(idx int, c schema.ToolCall) {
        defer wg.Done()
        result := e.registry.Execute(ctx, c)          // 并发执行
        observationMsgs[idx] = schema.Message{...}    // 各写各的下标，无数据竞争
    }(i, call)
}
wg.Wait()                                            // 聚合
for _, obs := range observationMsgs {                // 严格按原始顺序回填
    contextHistory = append(contextHistory, obs)
}
```

两个设计要点：

1. **预分配切片 + 按下标写**，避免多 goroutine 抢 `append` 造成数据竞争；
2. **聚合后按原序回填**，保证 `tool_call_id` 与结果的对应关系稳定 —— 模型是靠 id 匹配结果的，顺序乱了它会困惑。

顺带一个观察：课程为了「让模型一次规划出多个并行调用」开了慢思考（`EnableThinking = true`）。
本项目**没开**，只在 prompt 里写「同时一次性读取这三个文件」，DeepSeek 就直接吐出了 3 个并行调用 ——
说明这个行为主要靠提示词驱动，不是非得靠两阶段 hack。

## 目录结构

```
go-tiny-claw/
├── .env.example                  # 环境变量模板，复制成 .env 使用（.env 已被忽略）
├── hello.txt                     # read_file 的测试文本（第 05 章）
├── server.go                     # edit_file 的测试文件（第 07 章）
├── a.txt / b.txt / c.txt         # 并发读的测试文本（第 08 章）
├── go.mod / go.sum               # 带 anthropic-sdk-go、openai-go/v3 依赖
├── cmd/claw/main.go              # 用 DeepSeek Provider + 四件套工具驱动循环
└── internal/
    ├── config/dotenv.go          # 极简 .env 加载器（本项目新增，无第三方依赖）
    ├── engine/loop.go            # 两阶段 ReAct 循环 + 并发工具执行
    ├── provider/
    │   ├── interface.go          # LLMProvider 接口
    │   ├── claude.go             # Anthropic 实现（课程原版，未被调用）
    │   ├── openai.go             # OpenAI 兼容实现（课程原版 + 智谱构造器）
    │   └── deepseek.go           # DeepSeek 构造器（本项目新增）
    ├── schema/message.go
    └── tools/
        ├── registry.go           # BaseTool / Registry / registryImpl
        ├── read_file.go          # 读文件
        ├── write_file.go         # 写文件
        ├── bash.go               # 执行命令（Windows 走 cmd.exe）
        └── edit_file.go          # 局部替换 + 四层模糊匹配
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

预期：模型一次吐出 3 个 `read_file` 调用，Harness 并发执行，再按序把结果回填后综合（两轮）：

```
[Registry] 成功挂载工具: read_file / write_file / bash / edit_file
========== [Turn 1] 开始 ==========
[Engine] 模型请求并发调用 3 个工具...
  -> [Go-2] 🛠️ 触发并行执行: read_file
  -> [Go-0] 🛠️ 触发并行执行: read_file
  -> [Go-1] 🛠️ 触发并行执行: read_file
  -> [Go-1] ✅ 工具执行成功 (返回 62 字节)
  -> [Go-2] ✅ 工具执行成功 (返回 56 字节)
  -> [Go-0] ✅ 工具执行成功 (返回 56 字节)
[Engine] 所有并发工具执行完毕，开始聚合观察结果 (Observation)...
========== [Turn 2] 开始 ==========
🤖 [对外回复]: 三个文件都成功读取了……（a.txt 前端报错日志 / b.txt 后端响应时间 / c.txt 今天是周五）
[Engine] 模型未请求调用工具，任务宣告完成。
```

> 注意 `[Go-N]` 的完成顺序是乱的（谁先跑完谁先打印），但回填进上下文的顺序严格是 0/1/2，
> 因为 `observationMsgs` 是按下标写的、聚合后才 append。这正是并发实现要保证的东西。

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

第 09 章开始接「外部入口」：新增 `internal/feishu/bot.go`（飞书机器人）、`internal/engine/reporter.go`
（把执行过程回报出去），`loop.go` 同时瘦身重构，`go.mod` 也会多出依赖。
