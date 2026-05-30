## Goal
为 OC 服务器 `/opt/new-api` 项目添加可重复使用的重新构建部署脚本，并默认把 `entry-nginx` 与 NewAPI 放到同一个 Docker 内部网络，避免 nginx 通过宿主机 IP 反代。

## Active Task List
- [x] 定位 OC 上的 NewAPI 项目目录与当前 compose 部署方式。
- [x] 添加 rebuild/deploy shell 脚本。
- [x] 做脚本语法检查与帮助输出验证。
- [x] 记录脚本路径、默认行为和使用方式。

## Progress
### Done
- [x] 确认 NewAPI 项目目录为 `/opt/new-api`，compose 文件为 `/opt/new-api/docker-compose.yml`，服务名/容器名为 `new-api`。
- [x] 确认 `entry-nginx` 当前单独在 `entry-nginx_default` 网络，NewAPI 在 `new-api_new-api-network` 网络。
- [x] 已创建脚本 `/opt/new-api/scripts/rebuild-deploy.sh`，权限 `0755`。
- [x] 脚本默认构建本地镜像 `local/new-api:<HHMMSS>`，例如 `local/new-api:160009`；备份并更新 compose 的 `new-api.image`，执行 `docker compose up -d --no-deps new-api`，等待健康检查与 `/api/status`。
- [x] 脚本默认会把 `entry-nginx` 连接到 `new-api_new-api-network`，并把 `/opt/entry-nginx/conf.d/ai.muling.store.conf` 的 `proxy_pass` 改为 `http://new-api:3000;`，然后 `nginx -t` 和 reload。
- [x] 支持 `--monitor` 采集构建/部署期间 docker stats CSV 到 `/opt/new-api/logs/rebuild-deploy/`。
- [x] 已执行 `bash -n scripts/rebuild-deploy.sh`，语法检查通过；脚本 sha256 为 `6c3271abf8deaf5063c78fe5eae0998804f0e1261a310bb5896553f7a1239a47`。

### In Progress
- [ ] 等待是否执行一次实际重建部署。
- [x] 已按要求将默认镜像 tag 从日期时间改为纯时分秒 `HHMMSS`。

### Pending
- [ ] 如需上线，运行 `cd /opt/new-api && scripts/rebuild-deploy.sh --monitor`；如需无缓存构建，加 `--no-cache`。

## Key Findings (Current)
- 当前脚本已放好但尚未运行实际部署，因此线上容器仍保持原状态。
- 当前 nginx 配置仍是旧的 `proxy_pass http://172.21.0.1:3000;`，会在真正运行脚本时切到内部 Docker DNS `http://new-api:3000;`。
- 脚本不会打印 `SQL_DSN`、Redis 密码或容器环境变量。

## Next Step
- 用户确认需要部署时，在 OC 服务器执行：`cd /opt/new-api && scripts/rebuild-deploy.sh --monitor`。


## Token Test - gpt-image-2 2026-05-30 16:34 CST
- [x] 使用已启用的 `token` 令牌请求 `POST /v1/images/generations`。
- [x] 请求模型：`gpt-image-2`，尺寸：`1024x1536`，质量：`standard`，数量：`1`。
- [x] HTTP 返回：`200`。
- [x] 返回体包含 `data[0].b64_json`，长度约 `1,178,812` 字符。
- [x] NewAPI 记录消费日志：`token_name=token`，`channel_id=20`，`request_path=/v1/images/generations`，`request_conversion=[openai_image]`，`use_time_seconds=20`。
- [x] 结论：当前启用后，令牌真实图片请求可成功；本次实际命中 channel 20，不是之前 502 的 channel 19。


## Channel 19 Test - gpt-image-2 2026-05-30 16:42 CST
- [x] 测试前 channel 19 状态为 `1`，模型包含 `gpt-image-2,codex-gpt-image-2`，base_url 为 `https://api.junche.shop`。
- [x] 使用管理员 token 指定渠道方式请求 `POST /v1/images/generations`，Authorization 形式为 `Bearer <token>-19`，未输出真实 token。
- [x] 请求模型：`gpt-image-2`，尺寸：`1024x1536`，质量：`standard`，数量：`1`。
- [x] HTTP 返回：`502`。
- [x] 返回错误：`bad_response_status_code` / `openai_error`。
- [x] NewAPI 错误日志：`channel_id=19`，`request_path=/v1/images/generations`，`status_code=502`，`use_time=3`。
- [x] GIN 日志：`POST /v1/images/generations`，HTTP `502`，耗时约 `5.41s`。
- [x] 测试后 channel 19 被自动禁用为 `status=3`，`status_reason=status_code=502, bad response status code 502`。
- [x] 结论：修复后 channel 19 的测试已经正确打到图片端点；失败原因是上游 `https://api.junche.shop` 的图片接口返回 502，不是 NewAPI 再走错 chat endpoint。


## Cron - Auto Recover Auto-Disabled Channels 2026-05-30
- [x] 已创建目录：`/opt/new-api/cron/`，包含 `logs/`、`state/` 和锁文件路径。
- [x] 已创建脚本：`/opt/new-api/cron/auto_recover_channels.py`，权限 `0755`，sha256=`651e0e349c899ae9fdf2e5efc6039120df6801020202c39ecbd78f8da918edd4`。
- [x] 已安装系统 cron：`/etc/cron.d/newapi-auto-recover-channels`。
- [x] 执行频率：`*/5 * * * *`，即每 5 分钟一次。
- [x] 日志路径：`/opt/new-api/cron/logs/auto_recover_channels.log`。
- [x] 状态文件：`/opt/new-api/cron/state/auto_recover_last_run.json`。
- [x] 脚本带非阻塞文件锁，避免上一轮没结束时重复运行。
- [x] 脚本只扫描 `channels.status=3` 的自动禁用渠道，不处理手动禁用 `status=2`。
- [x] 测试方式：读取一个已启用 token 但不输出密钥，使用指定渠道 token 后缀 `<token>-<channel_id>` 调用本机 NewAPI；图片模型走 `/v1/images/generations`，非图片模型走 `/v1/chat/completions`。
- [x] 为了能测试已禁用渠道，脚本会短暂把目标 channel 状态设为 `1`，但不启用 abilities；测试通过才正式保留 `status=1` 并启用 abilities，测试失败则恢复 `status=3` 并记录失败原因。
- [x] 已验证 dry-run：识别到 9 个自动禁用渠道；首次实际 cron 运行测试了 10 个渠道，其中 channel 14 测试通过并被启用，其余失败后保持自动禁用。
- [x] 当前 cron 服务为 active。
- [x] 注意：早期首次运行日志里 channel 14 成功响应包含较大的图片 JSON 片段；脚本已修补为成功响应只记录 `ok: data returned`，后续不会再输出 b64 图片内容。


## 2026-05-30 auto-recover cron 使用模型测试令牌

- 更新 `/opt/new-api/cron/auto_recover_channels.py` 的令牌选择逻辑：优先使用启用状态的 `模型测试` 令牌，其次 `测试key`、常见 test 名称，最后才回退到 `token`。
- 新增一个 admin 用户下的 `模型测试` 令牌用于 cron 指定渠道模型测试；未在日志或回复中输出 token key。
- 保持原定时任务不变：`/etc/cron.d/newapi-auto-recover-channels` 每 5 分钟运行一次。
- 语法检查通过：`python3 -m py_compile /opt/new-api/cron/auto_recover_channels.py`。
- dry-run 验证通过：能列出自动禁用渠道且不修改状态。
- 单次 `--limit 1` 实测已用 `token_name=模型测试` 写入日志；因 channel #1 上游返回 500，渠道仍保持 `status=3`，说明失败时不会误启用。
- 当前脚本 sha256：`55d15e7b21148a5a9658c9e5789e89dc6e55c0dc6ced2c08448d73ab3f8940f7`。
- cron 服务状态：`active`。


## 2026-05-30 entry-nginx 流式/长连接配置优化

- 背景：NewAPI 日志出现 `client_gone` / `context canceled`，entry-nginx 日志对应较多 `499`，多发生在长流式 `/v1/responses`、`/v1/chat/completions` 请求。
- 操作原则：生产环境谨慎变更；未重启 NewAPI，未改 compose，未改 upstream，未改域名路由，只对 entry-nginx 做平滑 reload。
- 变更文件：`/opt/entry-nginx/conf.d/ai.muling.store.conf`、`/opt/entry-nginx/conf.d/ai2.muling.store.conf`。
- 备份：`/opt/new-api/backups/nginx-20260530_150023/conf.d/`，并在 `/opt/entry-nginx/conf.d/` 下生成 `*.bak_20260530_150202`、`*.bak_20260530_150310`。
- 保持已有配置：`proxy_pass http://new-api:3000`、`proxy_read_timeout 3600s`、`proxy_send_timeout 3600s`、`proxy_connect_timeout 60s`、`proxy_buffering off`、`proxy_cache off`、`proxy_request_buffering off`。
- 新增/调整：`send_timeout 3600s`、`keepalive_timeout 3600s`、`gzip off`、`add_header X-Accel-Buffering no always`、`proxy_set_header Connection ""`。
- 验证：`docker exec entry-nginx nginx -t` 成功。
- 应用方式：`docker exec entry-nginx nginx -s reload` 平滑重载成功。
- Smoke test：对 `ai.muling.store` 和 `ai2.muling.store` 使用 origin Host header 请求 `http://127.0.0.1/api/status` 均返回 HTTP 200，并能看到响应头 `X-Accel-Buffering: no`。
- 容器状态：`new-api` 仍为 healthy，`entry-nginx` 仍为 up。
- 注意：该优化降低 nginx 对流式响应的缓冲/断连风险，但不能阻止客户端主动断开或 Cloudflare 边缘限制导致的 `499/client_gone`；若仍出现，需要继续推进直连灰云 API 域名、客户端 read timeout、减少长请求首包等待。
