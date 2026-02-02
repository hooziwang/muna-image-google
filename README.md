运用 Gemini API 生成图像的 Go 命令行工具。

## 环境要求

- Go 1.22+
- 环境变量 `MUNA_GEMINI_API_KEY`

## 安装依赖

```bash
go mod tidy
```

## 使用方式

```bash
```bash
# 使用默认提示词生成
MUNA_GEMINI_API_KEY=... go run .

# 自定义提示词
MUNA_GEMINI_API_KEY=... go run . "一个小机器人在画夕阳" --out outputs

# 通过 stdin 提供提示词
echo "清晨的未来城市天际线" | MUNA_GEMINI_API_KEY=... go run . --out outputs

# 指定模型
MUNA_GEMINI_API_KEY=... go run . --model gemini-3-pro-image-preview "极简风茶馆 logo" --out outputs

# 设置宽高比与尺寸（适用于 gemini-3-pro-image-preview）
MUNA_GEMINI_API_KEY=... go run . "现代咖啡馆室内" --aspect 16:9 --size 2K --out outputs

# 增加超时
MUNA_GEMINI_API_KEY=... go run . "现代咖啡馆室内" --timeout 5m --out outputs

# 详细 HTTP 日志（API Key 自动脱敏）
MUNA_GEMINI_API_KEY=... go run . "现代咖啡馆室内" -v --out outputs

# 使用参考图片（可重复，最多 14 张）
MUNA_GEMINI_API_KEY=... go run . "办公室合影，搞怪表情" -r person1.png -r person2.png -r person3.png --out outputs
```
```

## 参数说明

```text
--model   模型 ID（默认：gemini-3-pro-image-preview）
--out     输出目录（默认：.）
--aspect  宽高比（如 1:1、16:9）
--size    图像尺寸（1K、2K、4K，默认：4K）
--timeout 总超时（如 30s、5m，默认：5m）
--verbose 详细日志（API Key 脱敏、长字段裁剪）
--ref     参考图片路径（可重复，最多 14 张）
```

## 备注

- 默认模型为 `gemini-3-pro-image-preview`。
