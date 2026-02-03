运用 Gemini API 生成图像的命令行工具。

## 环境要求

- Go 1.22+
- 环境变量 `MUNA_GEMINI_API_KEY`（可设置多个 key，调用时随机选择一个）

## 使用方式

```bash
# 先设置环境变量（只需设置一次，后续命令无需重复）
export MUNA_GEMINI_API_KEY="你的 key"

# 使用默认提示词生成
muna-image-google

# 多个 key（可用逗号、分号、空白或换行分隔）
export MUNA_GEMINI_API_KEY="key_1
key_2
key_3"
muna-image-google "一个小机器人在画夕阳" --out outputs

# 自定义提示词
muna-image-google "一个小机器人在画夕阳" --out outputs

# 通过 stdin 提供提示词
echo "清晨的未来城市天际线" | muna-image-google --out outputs

# 通过文件提供提示词（管道）
cat prompt.txt | muna-image-google --out outputs

# 指定模型
muna-image-google --model gemini-3-pro-image-preview "极简风茶馆 logo" --out outputs

# 设置宽高比与尺寸（适用于 gemini-3-pro-image-preview）
muna-image-google "现代咖啡馆室内" --aspect 16:9 --size 2K --out outputs

# 增加超时
muna-image-google "现代咖啡馆室内" --timeout 5m --out outputs

# 详细 HTTP 日志（API Key 自动脱敏）
muna-image-google "现代咖啡馆室内" -v --out outputs

# 使用参考图片（可重复，最多 14 张，支持本地路径或 URL）
muna-image-google "办公室合影，搞怪表情" -r person1.png -r person2.png -r https://example.com/person3.png --out outputs

# 并行生成多张（每次并发生成一张）
muna-image-google "一只在海边跑步的狗" -n 3 --out outputs

# 统计管道整体耗时
time cat prompt.txt | muna-image-google --out outputs

# 仅统计生成命令耗时
cat prompt.txt | time muna-image-google --out outputs

# 检查所有 API Key 是否有效（并发）
muna-image-google key
```

## 参数说明

```text
--model   模型 ID（默认：gemini-3-pro-image-preview）
--out     输出目录（默认：.）
--aspect  宽高比（如 1:1、16:9）
--size    图像尺寸（1K、2K、4K，默认：4K）
--timeout 总超时（如 30s、5m，默认：5m）
--verbose 详细日志（API Key 脱敏、长字段裁剪）
--ref     参考图片路径或 URL（可重复，最多 14 张）
--count   生成数量（默认：1）
```

## 行为说明

- 输出目录：`--out` 是目录，不是文件名。
- 输出文件名：`YYYYMMDD` + 12 位大写字母数字 + 扩展名（按 MIME 推断，未知为 `.jpg`）。
- 输出内容：
  - 成功时：只输出生成文件的绝对路径。
  - 失败且无图时（非 `-v`）：只输出 `finishMessage` 内容并以非 0 退出码结束。
- 并发生成：`-n/--count` 会并发发起请求；**每次请求前随机选择一个 key**。
- 参考图片：`-r/--ref` 最多 14 张；URL 会被下载并以 `inlineData` 发送，URL 不可访问会直接失败。
- 多 key 分隔：逗号、分号、空白、换行均可分隔。
- 提示词输入：有位置参数时优先生效；无位置参数时才读取 stdin。
- 详细日志：`-v` 会打印请求/响应日志，API Key 自动脱敏、长字段裁剪。

## key 子命令

- 用法：`muna-image-google key`
- 作用：并发检查 `MUNA_GEMINI_API_KEY` 中的所有 key 是否有效。
- 输出格式：
  - key 打码为前 4 + `...` + 后 8
  - 成功：亮绿色 `OK`
  - 失败：亮红色 `FAIL`，并输出 `code reason message`
- 退出码：只要有一个失败则退出码非 0。
- 超时：`muna-image-google key --timeout 5s`

## 备注

- 默认模型为 `gemini-3-pro-image-preview`。
