# NewAPI 渠道连续错误自动禁用与自动检测启用

## Goal
- NewAPI 渠道连续错误 10 次后自动禁用。
- 渠道管理支持开启 5 分钟检测一次渠道，检测成功后自动启用自动禁用渠道。
- 自动化测试优化功能并检查合理性。

## Active Task List
- [x] locate-newapi：定位 NewAPI 仓库与渠道错误统计/测试相关代码
- [x] disable-after-10：实现连续错误10次自动禁用渠道
- [x] auto-test-enable：实现渠道5分钟自动检测成功后启用
- [x] verify-newapi：运行测试/构建验证
- [x] report-newapi：汇报改动与部署方式

## Progress
### Done
- [x] 已定位仓库：`/vol3/root_backup/github-work/new-api-zhumengstar`
- [x] 已定位请求错误禁用入口：`controller/relay.go::processChannelError` 当前遇到可禁用错误会立即 `service.DisableChannel(...)`
- [x] 已定位渠道禁用/启用服务：`service/channel.go::{DisableChannel, EnableChannel, ShouldDisableChannel, ShouldEnableChannel}`
- [x] 已定位渠道状态落库：`model/channel.go::UpdateChannelStatus`，状态原因写入 `OtherInfo` 的 `status_reason/status_time`
- [x] 已定位自动检测：`main.go` 启动 `controller.AutomaticallyTestChannels()`；`controller/channel-test.go::AutomaticallyTestChannels/testAllChannels`；`setting/operation_setting/monitor_setting.go` 已有 `auto_test_channel_enabled/auto_test_channel_minutes` 与 `CHANNEL_TEST_FREQUENCY` 环境变量
- [x] 已新增 `service/channel_health_test.go`，覆盖连续错误 9 次不禁用、第 10 次禁用、成功清零、自动启用只针对 auto-disabled。
- [x] 已实现 `service.RecordChannelFailureAndMaybeDisable` 与 `service.ClearChannelConsecutiveErrors`，错误计数持久化在 `Channel.OtherInfo` 的 `consecutive_error_count/consecutive_error_last`。
- [x] 已将 `controller/relay.go::processChannelError` 改为记录连续错误，达到 10 次后才调用现有 `DisableChannel`。
- [x] 已在请求成功后、自动检测成功后清零连续错误计数。
- [x] 已将后端默认自动检测间隔从 10 分钟改为 5 分钟；同步 default/classic 前端默认值为 5。
- [x] 已运行 targeted tests：`go test ./service ./controller ./setting/operation_setting -count=1` 通过。
- [x] 已运行全量 `go test ./... -count=1`：本次改动相关包通过；仓库既有失败位于 `relay/channel/claude` 与 `relay/helper`，与本次渠道健康逻辑无关。
- [x] 已在 tx 上完成本地构建产物上传与替换运行二进制，保留回滚目录 `rollback/local-binary-20260521_064337`。
- [x] 已验证 tx 上 `new-api` 容器恢复为 healthy，`/api/status` 200，首页静态资源正常，`/v1/models` 在无 token 情况下返回 401。

### In Progress
- [x] 无。

### Pending
- [ ] 已按交叉比对确认 tx 运行镜像中的核心源码补丁与本机 fork 8 个关键文件一致；下一步将只把源码/测试/任务记录提交到 `zhumengstar/new-api`，不提交 `.last_*`、build log、镜像包、audit/rollback/backup 等运行产物。
- [ ] 如需后续更换为新镜像构建方式，可再做一次容器镜像版部署。

## Key Findings (Current)
- 现状已从“单次命中 ShouldDisableChannel 的错误即自动禁用”改为“连续 10 次可禁用错误才自动禁用”。
- 自动检测框架已存在，检测成功启用逻辑已存在且只启用 `ChannelStatusAutoDisabled`，不会误启用手动禁用渠道。
- 自动检测默认间隔已从 10 分钟改为 5 分钟；后台开启开关仍使用现有 `auto_test_channel_enabled` 配置，环境变量 `CHANNEL_TEST_FREQUENCY` 仍可覆盖。
- 合理方案：复用 `Channel.OtherInfo` 避免新增 DB migration；成功请求/成功检测清零，减少偶发抖动误禁用。

## Next Step
- 如需我继续，可再做一次镜像化打包或补充前端文案/后台开关说明。