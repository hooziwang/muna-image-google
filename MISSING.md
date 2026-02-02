# Gemini API 适配缺失清单

以下清单基于官方 `models.generateContent` 与 `models.streamGenerateContent` 全量能力，逐项对照当前 CLI（仅图像生成）得出。

## 1. 输入与多模态
- 缺：图像/音频/视频/PDF 输入（本地文件、URI、base64）
- 缺：多文件混合输入与多 `parts` 组合
- 缺：File API 上传（files.upload）与 URI 引用

## 2. 生成模式
- 缺：文本生成（纯文本 response）
- 缺：多模态输出控制（responseModalities）
- 缺：流式生成（streamGenerateContent）

## 3. 生成配置（GenerationConfig）
- 缺：candidateCount
- 缺：stopSequences
- 缺：maxOutputTokens
- 缺：temperature / topP / topK
- 缺：seed
- 缺：presencePenalty / frequencyPenalty
- 缺：responseMimeType / responseSchema / responseJsonSchema
- 缺：responseLogprobs / logprobs
- 缺：mediaResolution

## 4. 系统指令
- 缺：systemInstruction 支持

## 5. 工具调用
- 缺：Function Calling（工具声明、toolConfig、自动回填结果）
- 缺：Code Execution

## 6. 安全设置
- 缺：safetySettings 参数化
- 缺：safetyRatings / finishReason / usageMetadata 输出控制

## 7. 缓存
- 缺：cachedContent 支持
- 缺：caches.create / get / list / delete

## 8. 响应处理
- 缺：非图像响应的标准化输出（text/json/metadata）
- 缺：多候选输出处理（candidates）

## 9. CLI 结构
- 缺：子命令划分（gen/chat/image/files/cache/tools）
- 缺：统一输入输出规范与参数整合
