# Backend Improvement Plan

## Immediate (this sprint)

### P1
- [ ] WebSocket 断线重连消息补发 — 支持 `since_id` 参数拉取断线期间消息
- [ ] 消息已读回执 — mark_as_read 后推送 `message.read` 事件给发送者

### P2
- [ ] 跨会话全局搜索 — `GET /api/v1/messages/search?q=keyword`
- [ ] 文件过期清理 — 定时任务清理 6 个月以上的 file_records（可配置）
- [ ] 按 API Key 限流 — 替代按 IP 限流
- [ ] 结构化 JSON 日志 — 统一 GIN + 审计日志格式
- [ ] 健康检查端点 — `GET /api/v1/health`
- [ ] 文件下载认证测试补充（CI）
- [ ] WebSocket 集成测试

### P3
- [ ] Webhook 重试优化 — 指数退避 + 死信队列
- [ ] 消息编辑历史
- [ ] JWT Token 过期时间可配置
- [ ] API Key 权限分级
