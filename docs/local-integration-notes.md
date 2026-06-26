# 本地整合提交说明

当前分支基于 `QuantumNous/new-api` 上游主线，并在提交 `f2522b2c 整合本地功能并同步上游主线` 中保留了本地功能改动。

## 主要功能

- 会话亲和与上游会话头转发：支持把用户会话标识传递到上游渠道，用于请求亲和和路由稳定性。
- 渠道故障转移：请求失败时支持跨渠道优先级重试，并避免用户侧请求错误触发无效重试。
- 渠道自动禁用：支持 5xx 状态码自动禁用渠道，并修复多 Key 渠道禁用后的缓存状态。
- 用户管理：用户列表支持服务端时间排序。
- 兑换码管理：支持批量删除选中的兑换码。
- 渠道测试：允许没有模型价格配置时继续执行渠道测试。
- OpenAI usage 保护：当上游返回的 `prompt_tokens` 明显低于本地估算时，使用本地估算值防止少计费。
- Gemini 图片协议适配：支持 Gemini 图片生成/编辑模型走 OpenAI 图片接口，包括 `/v1/images/generations` 和 `/v1/images/edits`。
- 图片参数适配：支持 `size`、`quality`、`1k/2k/4k`、具体像素尺寸、宽高比和 `strict_aspect_ratio`。
- 图片响应转换：支持 Gemini 原生图片响应、OpenAI 图片响应和 Gemini chat 图片响应转换为 OpenAI 图片响应格式。
- 绘图日志增强：生成图片后记录绘图日志，保存图片宽高、原图大小和 MIME 类型。
- 图片展示优化：绘图日志列表展示缩略图，查看时使用预览图，下载时拉取原图。

## 移除内容

已移除与当前 New API 主业务无直接关系的 Paseo 辅助脚本：

- `bin/paseo_failover.sh`
- `bin/paseo_failover.env.example`
- `cmd/paseo-proxy/`

这些脚本只是独立的 OpenAI-compatible 接口故障转发/调试工具，不属于 New API 服务启动链路，也没有被构建流程或业务代码引用。
