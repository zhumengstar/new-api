# NewAPI 自动禁用渠道恢复检测

## Goal
- 被自动禁用的渠道要每分钟发起测试请求。
- 测试请求成功后自动解除自动禁用，恢复渠道能力。
- 自动禁用渠道恢复测试不受响应时间阈值限制；只要请求成功即可恢复。
- 全量渠道测试周期为 1 小时。
- 渠道连续自动禁用错误阈值改为 3 次。
- 先处理 tx 生产 channel 8，并确认视频文件重新交付。

## Active Task List
- [x] 定位 NewAPI 代码库与生产目标，确认 tx 生产 channel 8 当前状态/禁用原因
- [x] 完成自动禁用渠道每分钟恢复测试逻辑，成功后自动解除自动禁用
- [x] 构建并上传 `dist/new-api-tx-auto-disabled-recovery` 到生产机
- [x] 热替换部署并验证 channel 8 自动恢复成功
- [x] 将全量渠道测试周期改为 60 分钟并重启验证健康
- [x] 修改自动禁用渠道恢复测试逻辑：不再使用响应时间阈值阻止恢复
- [x] 修改连续错误自动禁用阈值：10 次改为 3 次
- [ ] 构建、部署并验证“恢复测试无响应时间阈值 + 连续 3 次禁用”补丁
- [ ] 回报视频文件状态

## Verified Production Facts
- tx production host: `43.173.104.221`
- NewAPI container health: `/api/status = 200` after prior restart
- Channel 8 previously: `status=3`, `auto_ban=1`, reason was upstream 502
- Channel 8 after first hot replace: `status=1`, `auto_ban=1`, `status_reason=""`
- Full channel test setting after change: `monitor_setting.auto_test_channel_minutes = 60`
- New recovery marker in code: `AUTO_DISABLED_CHANNEL_RECOVERY_V1`
- No-threshold recovery marker in code: `AUTO_DISABLED_CHANNEL_RECOVERY_NO_THRESHOLD_V1`

## Notes
- Manual disabled channels (`status=2`) are not auto-recovered.
- Auto-disabled channels (`status=3`) are tested every minute while automatic test/enable is on.
- Enabled channels (`status=1`) may still use response-time threshold during full channel testing.
- Auto-disabled channel recovery should not fail only because response time exceeds `ChannelDisableThreshold`.
