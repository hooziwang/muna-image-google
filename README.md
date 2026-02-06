# muna-image-google

运用 Gemini API 生成图像的命令行工具。

## 安装与构建

```bash
# 编译、测试并安装到 GOPATH/bin
make

# 或仅测试
go test ./...

# 或仅安装
go install .
```

## 测试与质量保障

推荐在提交前按下面顺序执行：

```bash
# 1) 全量构建 + 测试 + 安装
make

# 2) 竞态检查（并发相关修改必跑）
go test -race ./cmd

# 3) 覆盖率检查（建议关注 cmd 包）
go test -count=1 -cover ./cmd
```

建议阈值：
- 日常开发：`cmd` 包覆盖率不低于 `65%`
- 合并前：`cmd` 包覆盖率尽量维持在 `70%+`
- 新增功能或修复缺陷：必须附带对应测试（成功路径 + 至少一个失败路径）

当前基线（2026-02-06）：`cmd` 包覆盖率约 `70.7%`。

## 环境变量

- `MUNA_GEMINI_API_KEY`：Gemini API Key，必填。
  - 支持配置多个 key
  - 支持分隔符：逗号、分号、空白、换行
  - 运行时会随机选取 key（`--key` 过滤后在结果集里随机）

示例：

```bash
export MUNA_GEMINI_API_KEY="key_1
key_2
key_3"
```

## 基本用法

```bash
# 无参数且无 stdin 时显示帮助
muna-image-google

# 使用提示词生成
muna-image-google "一个小机器人在画夕阳" --out outputs

# 通过 stdin 传提示词
echo "清晨的未来城市天际线" | muna-image-google --out outputs

# 指定模型
muna-image-google --model gemini-3-pro-image-preview "极简风茶馆 logo" --out outputs

# 设置宽高比与尺寸
muna-image-google "现代咖啡馆室内" --aspect 16:9 --size 2K --out outputs

# 固定种子
muna-image-google "现代咖啡馆室内" --seed 1011567824 --out outputs

# dry-run：仅查看请求配置，不发生真实请求
muna-image-google "现代咖啡馆室内" --seed 1011567824 --dry-run
```

## 最佳实践

### 使用 pueue 排队执行（推荐与下列场景组合）

当需要长时间跑图、批量任务或多轮重试时，建议将命令交给 `pueue` 管理。

```bash
# 1) 添加任务（示例：场景 3 的批量出图）
pueue add -- muna-image-google "一只在海边跑步的狗" --count 3 --out ./outputs/batch

# 2) 查看队列与任务 ID
pueue status

# 3) 查看某个任务日志（示例 ID=25）
pueue log 25

# 4) 失败后重试
pueue restart 25

# 5) 删除任务
pueue remove 25

# 6) 清理已完成任务
pueue clean
```

### 场景 1：先 dry-run，再正式生成

先确认请求参数，再执行真实生成，减少无效调用。

```bash
muna-image-google "品牌 KV 海报" \
  --aspect 16:9 \
  --size 2K \
  --seed 20260206 \
  --dry-run
```

```bash
muna-image-google "品牌 KV 海报" \
  --aspect 16:9 \
  --size 2K \
  --seed 20260206 \
  --out ./outputs/kv
```

### 场景 2：参考图生成（多人物/风格约束）

通过多张参考图控制角色和风格一致性。

```bash
muna-image-google "办公室合影，搞怪表情" \
  -r person1.png \
  -r person2.png \
  -r https://example.com/person3.png \
  --aspect 21:9 \
  --out ./outputs/team
```

### 场景 3：并发批量出图

一次并发生成多张图，提高出图效率。

```bash
muna-image-google "一只在海边跑步的狗" \
  --count 3 \
  --out ./outputs/batch
```

## 常用参数

- `--model`/`-m`：模型 ID（默认：`gemini-3-pro-image-preview`）
- `--out`/`-o`：输出目录
- `--aspect`/`-a`：宽高比（如 `1:1`、`16:9`）
- `--size`：图像尺寸（`1K`、`2K`、`4K`）
- `--seed`/`-s`：指定种子（`0-2147483647`）
- `--count`/`-n`：生成数量（并发）
- `--ref`/`-r`：参考图片路径或 URL（可重复，最多 14 张）
- `--key`/`-k`：指定使用的 API Key（可重复；支持子串模糊匹配）
- `--timeout`：总超时（默认：`5m`）
- `--dry-run`/`-D`：仅打印请求配置，不会发生真实的请求。
- `--verbose`/`-v`：详细日志（API Key 脱敏、长字段裁剪）

## 参数详细说明

### 1) 提示词输入方式

```bash
# 位置参数优先
muna-image-google "一只在海边跑步的狗"

# 无位置参数时读取 stdin
cat prompt.txt | muna-image-google
```

说明：无参数且无 stdin 时，显示帮助并退出。

### 2) 输出目录与文件名

```bash
muna-image-google "现代咖啡馆室内" --out ./outputs
```

说明：
- `--out` 是目录，不是文件名。
- 输出文件名格式：`YYYYMMDD` + 12 位大写字母数字 + `-seed` + 扩展名。

### 3) 宽高比与尺寸

```bash
muna-image-google "海报" --aspect 9:16 --size 2K
```

说明：`--size` 默认 `4K`。

### 4) 种子控制

```bash
# 固定种子（尽量复现）
muna-image-google "海报" --seed 123456

# 不指定种子（每次随机）
muna-image-google "海报"
```

说明：相同提示词/参数/模型 + 相同种子时，模型会尽力给出一致结果，但仍可能有轻微差异。

### 5) 多图并发生成

```bash
muna-image-google "商品图" --count 5 --out ./outputs/products
```

说明：
- `--count` 会并发发起请求。
- 任一请求失败会导致进程最终以非 0 退出码结束。

### 6) 指定 API Key（模糊匹配）

```bash
muna-image-google "女孩自拍照片" -k Bxj91F48 -k AbCd --out outputs
```

说明：
- 模糊匹配支持 key 子串。
- 任意一个模式匹配不到 key 会直接报错。
- 若匹配多个 key，则在匹配结果中随机选择。

### 7) 参考图输入（本地与 URL）

```bash
# 本地图
muna-image-google "办公室合影" -r person1.png -r person2.png

# URL 图
muna-image-google "办公室合影" -r https://example.com/person3.png
```

说明：最多 14 张，URL 不可访问会直接失败。

### 8) dry-run（不发真实请求）

```bash
muna-image-google "快速查看请求配置" --dry-run
```

说明：用于调试最终请求参数（模型、内容、配置、执行模式），不会调用真实生成接口。

### 9) 超时与日志

```bash
# 调整超时
muna-image-google "现代咖啡馆室内" --timeout 5m

# 详细日志（脱敏）
muna-image-google "现代咖啡馆室内" -v
```

## 子命令

### API Key 可用性检查

```bash
# 检查所有 key
muna-image-google key

# 设置检查超时
muna-image-google key --timeout 5s
```

输出说明：
- key 打码为前 4 + `...` + 后 8
- 成功显示亮绿色 `OK`
- 失败显示亮红色 `FAIL` 并附带 `code reason message`

### 模型列表与模糊查询

```bash
# 列出所有模型
muna-image-google model

# 模糊查询（匹配名称/显示名/描述）
muna-image-google model gemini

# JSON 输出
muna-image-google model --json
```

## 发布（Release）

- 当推送 semver 格式的 tag（`v*.*.*`，例如 `v0.1.0`）时，GitHub Actions 会使用 GoReleaser 自动触发发布流程：
  - 交叉编译多个平台：Linux/macOS/Windows（amd64、arm64）
  - 执行 smoke test（在 Linux/macOS/Windows runner 上运行二进制 `--help`）
  - 自动创建或更新对应的 GitHub Release
  - 自动上传各平台压缩包产物（Windows 为 `.zip`，其他平台为 `.tar.gz`）
  - 自动生成并上传 `checksums.txt`（SHA256）

示例：

```bash
git tag -a v0.1.1 -m "release: 版本说明"
git push origin v0.1.1
```

## 备注

- 默认模型为 `gemini-3-pro-image-preview`。
