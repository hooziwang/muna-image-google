# Meta Commands Official URL Restriction Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 让 `key` 和 `model` 子命令仅在 Google 官方默认地址下运行，并在非官方地址时输出说明后退出。

**Architecture:** 在 `cmd` 包中新增一个专门给元信息子命令使用的 Base URL 守卫函数。`key` 与 `model` 在任何网络操作前先调用该守卫；若地址不合法则立即返回说明，不影响主出图命令的自定义网关能力。

**Tech Stack:** Go 1.24, Cobra, Go testing

---

### Task 1: 守卫逻辑测试

**Files:**
- Modify: `cmd/root_more_test.go`
- Test: `cmd/root_more_test.go`

**Step 1: Write the failing test**

增加这些测试：

- `TestValidateOfficialBaseURLForMetaCommands_AllowsUnset`
- `TestValidateOfficialBaseURLForMetaCommands_AllowsOfficialURL`
- `TestValidateOfficialBaseURLForMetaCommands_RejectsCustomURL`

**Step 2: Run test to verify it fails**

Run: `go test ./cmd -run 'TestValidateOfficialBaseURLForMetaCommands_'`

Expected: FAIL，提示缺少守卫函数或返回结果不符合预期

**Step 3: Write minimal implementation**

在 `cmd/root.go` 增加守卫函数，最小实现：

- 未设置时返回 `nil`
- 官方地址返回 `nil`
- 非官方地址返回错误

**Step 4: Run test to verify it passes**

Run: `go test ./cmd -run 'TestValidateOfficialBaseURLForMetaCommands_'`

Expected: PASS

### Task 2: 接入 key/model 子命令

**Files:**
- Modify: `cmd/key.go`
- Modify: `cmd/model.go`
- Test: `cmd/root_more_test.go`

**Step 1: Write the failing test**

增加命令级测试：

- `TestModelCommand_RejectsCustomBaseURL`
- `TestKeyCommand_RejectsCustomBaseURL`

要求断言：

- 退出前输出说明
- 不继续执行后续逻辑

**Step 2: Run test to verify it fails**

Run: `go test ./cmd -run 'TestModelCommand_RejectsCustomBaseURL|TestKeyCommand_RejectsCustomBaseURL'`

Expected: FAIL

**Step 3: Write minimal implementation**

将 `key` 和 `model` 子命令改为可返回错误的执行形式，在入口最前面接入守卫。

**Step 4: Run test to verify it passes**

Run: `go test ./cmd -run 'TestModelCommand_RejectsCustomBaseURL|TestKeyCommand_RejectsCustomBaseURL'`

Expected: PASS

### Task 3: 文档更新

**Files:**
- Modify: `README.md`
- Modify: `cmd/key.go`
- Modify: `cmd/model.go`
- Test: `cmd/root_more_test.go`

**Step 1: Write the failing test**

扩充帮助文案断言，要求 `key` 与 `model` 的长说明包含“仅支持 Google 官方默认地址”。

**Step 2: Run test to verify it fails**

Run: `go test ./cmd -run 'TestCommandLongHelpIncludesKeySource'`

Expected: FAIL

**Step 3: Write minimal implementation**

更新 README 与命令帮助文案。

**Step 4: Run test to verify it passes**

Run: `go test ./cmd -run 'TestCommandLongHelpIncludesKeySource'`

Expected: PASS

### Task 4: 全量验证

**Files:**
- Modify: `cmd/root.go`
- Modify: `cmd/model.go`
- Modify: `cmd/key.go`
- Modify: `cmd/root_more_test.go`
- Modify: `README.md`

**Step 1: Run package tests**

Run: `go test ./...`

Expected: PASS

**Step 2: Run repository build pipeline**

Run: `make`

Expected: PASS
