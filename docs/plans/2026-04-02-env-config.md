# Environment Variable Configuration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让接口地址、默认模型、API Key 支持通过 `MUNA_IMAGE_GOOGLE_*` 环境变量统一配置，并保持命令行参数优先与现有 API Key 回退逻辑。

**Architecture:** 在 `cmd` 包新增统一配置解析辅助函数，分别为 Base URL、模型默认值、API Key 来源提供单一入口。主命令、`model` 子命令、`key` 子命令都复用这套辅助逻辑，避免行为分叉。

**Tech Stack:** Go 1.24, Cobra, google.golang.org/genai, Go testing

---

### Task 1: 配置解析单测

**Files:**
- Modify: `cmd/root_more_test.go`
- Test: `cmd/root_more_test.go`

**Step 1: Write the failing test**

增加这些测试：

- `TestResolveModelValue_UsesEnvWhenFlagUnset`
- `TestResolveModelValue_UsesFlagWhenSet`
- `TestResolveBaseURLValue`
- `TestLoadConfiguredAPIKeys_PreferNewEnv`
- `TestBuildModelsListURL`

重点断言：

- `MUNA_IMAGE_GOOGLE_MODEL` 在未显式传 `--model` 时生效
- 显式传 `--model` 时覆盖环境变量
- `MUNA_IMAGE_GOOGLE_BASE_URL` 能正确规整为基础地址
- `MUNA_IMAGE_GOOGLE_API_KEY` 优先于旧 Key 读取逻辑
- `key` 子命令探测地址由 base URL 派生

**Step 2: Run test to verify it fails**

Run: `go test ./cmd -run 'TestResolveModelValue_UsesEnvWhenFlagUnset|TestResolveModelValue_UsesFlagWhenSet|TestResolveBaseURLValue|TestLoadConfiguredAPIKeys_PreferNewEnv|TestBuildModelsListURL'`

Expected: FAIL，提示缺少新配置解析函数或断言不成立

**Step 3: Write minimal implementation**

在 `cmd/root.go` 和 `cmd/key.go` 增加最小配置辅助函数：

- 解析默认模型
- 解析 Base URL
- 加载 API Key
- 构造 models 探测地址

**Step 4: Run test to verify it passes**

Run: `go test ./cmd -run 'TestResolveModelValue_UsesEnvWhenFlagUnset|TestResolveModelValue_UsesFlagWhenSet|TestResolveBaseURLValue|TestLoadConfiguredAPIKeys_PreferNewEnv|TestBuildModelsListURL'`

Expected: PASS

### Task 2: 接入主命令与子命令

**Files:**
- Modify: `cmd/root.go`
- Modify: `cmd/model.go`
- Modify: `cmd/key.go`
- Test: `cmd/genai_flow_test.go`

**Step 1: Write the failing test**

补一个集成向测试，验证自定义 Base URL 会被注入到 SDK 客户端请求中，或验证 `key` 子命令请求命中自定义地址。

**Step 2: Run test to verify it fails**

Run: `go test ./cmd -run 'TestGenerateOnce_.*|TestCheckKey_.*|TestBuildModelsListURL'`

Expected: FAIL，说明当前实现仍依赖固定地址或未走新配置入口

**Step 3: Write minimal implementation**

最小改动接入：

- 主命令：使用解析后的模型默认值、Base URL、API Key
- `model` 子命令：使用解析后的 Base URL、API Key
- `key` 子命令：使用解析后的 Base URL、API Key，并保留旧回退逻辑

**Step 4: Run test to verify it passes**

Run: `go test ./cmd -run 'TestGenerateOnce_.*|TestCheckKey_.*|TestBuildModelsListURL'`

Expected: PASS

### Task 3: 文档与帮助更新

**Files:**
- Modify: `README.md`
- Modify: `cmd/root.go`
- Modify: `cmd/model.go`
- Modify: `cmd/key.go`

**Step 1: Write the failing test**

补帮助文案相关断言，确保长说明里出现：

- `MUNA_IMAGE_GOOGLE_BASE_URL`
- `MUNA_IMAGE_GOOGLE_MODEL`
- `MUNA_IMAGE_GOOGLE_API_KEY`

**Step 2: Run test to verify it fails**

Run: `go test ./cmd -run 'TestCommandLongHelpIncludesKeySource'`

Expected: FAIL，说明帮助文案还未覆盖新环境变量

**Step 3: Write minimal implementation**

更新 README 与命令帮助文案，明确优先级和默认值：

- `MUNA_IMAGE_GOOGLE_BASE_URL=https://grsai.dakka.com.cn`
- `MUNA_IMAGE_GOOGLE_MODEL=nano-banana-pro`
- `MUNA_IMAGE_GOOGLE_API_KEY=sk-70ebdcf0e7b4414ebba49854c03b4a3b`

并说明新 API Key 环境变量未设置时，继续沿用当前 Key 获取逻辑。

**Step 4: Run test to verify it passes**

Run: `go test ./cmd -run 'TestCommandLongHelpIncludesKeySource'`

Expected: PASS

### Task 4: 全量验证

**Files:**
- Modify: `cmd/root.go`
- Modify: `cmd/model.go`
- Modify: `cmd/key.go`
- Modify: `cmd/root_more_test.go`
- Modify: `cmd/key_test.go`
- Modify: `cmd/genai_flow_test.go`
- Modify: `README.md`

**Step 1: Run package tests**

Run: `go test ./...`

Expected: PASS

**Step 2: Run repository build pipeline**

Run: `make`

Expected: build、test、install 全通过

**Step 3: Check coverage**

Run: `go test -count=1 -cover ./cmd`

Expected: PASS，并保持覆盖率不低于当前基线
