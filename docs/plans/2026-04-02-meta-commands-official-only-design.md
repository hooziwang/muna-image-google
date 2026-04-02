# key/model 子命令官方地址限制设计

**目标**

限制 `key` 与 `model` 两个子命令仅在 Google 官方默认地址下可用；当 `MUNA_IMAGE_GOOGLE_BASE_URL` 被设置为非官方地址时，输出说明并退出。

**需求确认**

- 仅检查 `MUNA_IMAGE_GOOGLE_BASE_URL`
- 未设置时，`key` / `model` 正常工作
- 设置为 `https://generativelanguage.googleapis.com` 时，`key` / `model` 正常工作
- 设置为非 Google 官方地址时，`key` / `model` 输出说明并退出
- 图像生成主命令不受此限制

**设计方案**

新增一个轻量守卫函数，专门给元信息类子命令使用：

1. 读取 `MUNA_IMAGE_GOOGLE_BASE_URL`
2. 去掉首尾空白和末尾 `/`
3. 若为空，放行
4. 若等于 `https://generativelanguage.googleapis.com`，放行
5. 否则返回错误，由 `key` / `model` 在入口处打印说明并退出

这个守卫仅放在 `key` 与 `model` 子命令入口最前面，保证：

- 不会在非官方地址下继续发请求
- 不会误用代理网关去跑模型列表和 key 检查
- 不影响主出图命令继续使用自定义地址

**输出文案**

建议输出包含两部分：

1. 当前检测到的自定义地址
2. 子命令仅支持的官方地址

例如：

```text
检测到自定义 MUNA_IMAGE_GOOGLE_BASE_URL=https://grsai.dakka.com.cn
key 和 model 子命令仅支持 Google 官方默认地址 https://generativelanguage.googleapis.com
```

**测试策略**

- 未设置 `MUNA_IMAGE_GOOGLE_BASE_URL` 时守卫放行
- 设置为官方地址时守卫放行
- 设置为官方地址且带末尾 `/` 时守卫放行
- 设置为非官方地址时守卫返回错误
- `key` / `model` 帮助说明同步更新

**文档变更**

- README 的子命令章节补充限制说明
- `key` 和 `model` 的 Long help 补充限制说明
