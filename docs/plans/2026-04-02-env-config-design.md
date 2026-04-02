# 环境变量配置设计

**目标**

为 `muna-image-google` 增加统一的环境变量配置能力，使接口地址、默认模型、API Key 都可以通过 `MUNA_IMAGE_GOOGLE_*` 环境变量控制，同时保持现有命令行参数优先级和现有 API Key 回退逻辑。

**需求确认**

- 环境变量前缀统一为 `MUNA_IMAGE_GOOGLE_`
- 采用统一配置解析方案
- 优先级为：命令行参数 > 环境变量 > 代码默认值
- `MUNA_IMAGE_GOOGLE_API_KEY` 未设置时，继续沿用当前 API Key 获取逻辑
- 主命令、`model` 子命令、`key` 子命令都使用同一套配置语义

**配置项**

- `MUNA_IMAGE_GOOGLE_BASE_URL`
  - 用于覆盖 Gemini SDK 默认接口地址
  - 用户提供的目标值为 `https://grsai.dakka.com.cn`
- `MUNA_IMAGE_GOOGLE_MODEL`
  - 用于覆盖主命令默认模型
  - 用户提供的目标值为 `nano-banana-pro`
- `MUNA_IMAGE_GOOGLE_API_KEY`
  - 用于直接提供 API Key
  - 用户提供的目标值为 `sk-70ebdcf0e7b4414ebba49854c03b4a3b`
  - 未设置时，继续沿用当前 `MUNA_GEMINI_API_KEY` / `~/.muna-image-google/.env` 的现有回退逻辑

**设计方案**

新增一层统一配置解析，集中处理三类来源：

1. 命令行参数
2. 新环境变量 `MUNA_IMAGE_GOOGLE_*`
3. 现有默认值与旧的 API Key 获取逻辑

主命令在启动时解析：

- 模型：若 `--model` 未显式设置，则读取 `MUNA_IMAGE_GOOGLE_MODEL`，否则使用当前默认值
- Base URL：若设置了 `MUNA_IMAGE_GOOGLE_BASE_URL`，则通过 `genai.HTTPOptions.BaseURL` 注入给 SDK；未设置时继续使用官方默认地址
- API Key：优先读取 `MUNA_IMAGE_GOOGLE_API_KEY`；未设置时走现有 `requireMunaGeminiAPIKeys()` 逻辑

`model` 子命令复用同一套 Base URL 和 API Key 逻辑，确保“列模型”和“实际出图”命中同一服务端。

`key` 子命令不再写死官方地址，而是根据统一配置生成待探测的 `/models` 地址。这样在接入代理网关时，健康检查结果与真实运行环境一致。

**兼容性**

- 现有 CLI 参数保持不变
- 现有 `MUNA_GEMINI_API_KEY` 与 `~/.muna-image-google/.env` 行为保持不变
- 新增的是上层覆盖能力，不会破坏老用户已有脚本

**文档变更**

- README 增加三项新环境变量说明
- README 中示例默认值改为新模型 `nano-banana-pro`
- CLI 帮助说明补充环境变量来源和优先级

**测试策略**

- 为模型默认值解析增加单测
- 为 Base URL 解析增加单测
- 为 API Key 优先级增加单测
- 为 `key` 子命令的目标地址构造增加单测
- 跑 `make`

**风险点**

- `key` 子命令当前直接拼接固定 URL，改成可配置后要小心路径拼接和尾部斜杠
- SDK 侧 Base URL 需要通过 `HTTPOptions` 传入，不能只靠环境变量侧写
- 文档里的旧默认模型如果不一并更新，会让行为和说明脱节
