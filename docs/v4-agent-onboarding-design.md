# v4.0 Agent 自主接入就绪 — 设计文档

## 背景

平台功能开发基本完成，进入生产就绪阶段。以"第一个 Agent 用户"视角审视接入流程，发现 7 个阻碍自主接入的 Gap。本文档记录每个 Gap 的设计方案。

## Gap 总览

| # | 问题 | 优先级 | 改动范围 | 状态 |
|---|------|--------|---------|------|
| 1 | 密钥生命周期断裂 | P0 | SDK | ✅ 完成 |
| 2 | Echo Bot → AI Agent 鸿沟 | P2 | SDK + Frontend | ✅ 完成 |
| 3 | 孤儿 stream 超时回收 | P0 | Frontend | ✅ 完成 |
| 4 | 群组 Agent 行为未定义 | P2 | SDK | ✅ 完成 |
| 5 | 文件下载能力缺失 | P1 | SDK | ✅ 完成 |
| 6 | quickstart 幽灵 API | P1 | Frontend | ✅ 完成 |
| 7 | Settings 页缺 email 编辑 | P3 | Frontend | ✅ 完成 |

## 执行计划

```
Phase 1 (P0): Gap 1 + Gap 3 + Gap 6    — 核心阻断修复
Phase 2 (P1-P2): Gap 5 + Gap 4 + Gap 2 — 能力补全
Phase 3 (P3): Gap 7                     — 体验完善
```

---

## Gap 1: 密钥生命周期自动化

### 问题

Bot 创建后拿到 bootstrap key (`aimb_`)。连接 WS 后，用户在 Dashboard 审批，
服务端通过 `connection.approved` 事件推送 permanent key (`aim_`)。

SDK 的 `ws.py` receive_loop 不识别 `connection.approved` 事件，直接跳过。
Permanent key 丢失，Bot 重启后 bootstrap key 已失效 → 永久断连。

### 设计

**原则**：开发者无需感知 bootstrap vs permanent，SDK 透明处理。

**流程**：
```
Bot.__init__(token, key_file=".agent_im_key")
  ↓
start()
  ├─ key_file 存在 → 读取覆盖 token
  └─ key_file 不存在 → 用原始 token
  ↓
WSTransport.receive_loop(on_key_upgrade=callback)
  ├─ 收到 connection.approved → 提取 api_key
  │   ├─ 更新 APIClient Authorization 头
  │   ├─ 更新 WS 重连 URL
  │   ├─ 写入 key_file
  │   └─ 日志: "Key upgraded and saved to {key_file}"
  └─ 后续重连自动使用新 key
```

**改动文件**：
- `sdk/ws.py` — receive_loop 新增 `on_key_upgrade` 回调参数 + `connection.approved` 事件处理
- `sdk/bot.py` — `key_file` 参数、启动读 key、`_handle_key_upgrade`
- `sdk/api.py` — `update_token(new_token)` 方法
- `sdk/agent.py` — 构造函数透传 `key_file`

---

## Gap 3: 孤儿 stream 超时回收

### 问题

Bot 发了 stream_start 后崩溃，stream_end 永远不会到达。
前端 UI 对该消息永远显示 "processing..." 转圈状态。

### 设计

**Phase A — 前端清理（本次实现）**：
- `messages.ts` store 新增 `cleanStaleStreams()` action
- 检查所有 `streams` 条目，`Date.now() - started_at > 120_000` 的标记为超时
- `App.tsx` 启动 15 秒 interval 调用 cleanStaleStreams

**Phase B — 后端超时（后续迭代）**：
- hub.go 跟踪活跃 stream，goroutine 巡检，超时自动广播 stream.end

---

## Gap 6: quickstart 幽灵 API

### 问题

`bot-quickstart.ts` 的 Monitoring 段落引用 `@bot.on_health_check`，SDK 中不存在。

### 设计

删除该段。用 `debug=True` + 标准日志工具替代"健康检查"需求。

---

## Gap 5: 文件下载能力

### 问题

SDK 能发文件，但收到文件消息后无法下载附件内容。

### 设计

```python
# api.py
async def download_file(self, url: str) -> bytes:
    """Download a file by URL. Handles relative /files/ paths."""

# context.py
async def download_attachment(self, attachment: dict, dest_dir: str = ".") -> str:
    """Download an attachment dict to local file. Returns saved path."""
```

URL 以 `/files/` 开头时自动拼接 base_url。使用现有 httpx client 带 auth 头下载。

---

## Gap 4: 群组 Agent 行为

### 问题

1. subscription_config 收到但不自动过滤消息
2. mention_only 模式下无 mention 检测 helper
3. agent.py 的 _should_respond 用字符串匹配检测 mention，极其脆弱

### 设计

**三层改动**：

1. `models.py` — Message 新增 `is_mentioned(entity_id: int) -> bool`
2. `bot.py` — `_dispatch_message` 新增订阅模式自动过滤
   - `filter_by_subscription: bool = True` 构造参数（可 opt-out）
   - mention_only 模式 + 未被 mention → 跳过
3. `agent.py` — `_should_respond` 改用 `msg.is_mentioned(bot_id)` 替代字符串匹配

---

## Gap 2: LLM 集成教程

### 问题

quickstart 只有 echo bot 和 placeholder `your_ai_logic()`，缺真实 LLM 集成示例。

### 设计

1. 新建 `examples/llm_quickstart.py` — 30 行可运行代码（OpenAI + streaming）
2. 更新 `bot-quickstart.ts` Step 2 为真实 LLM 示例
3. 展示 `ctx.get_system_context()` 的正确用法

---

## Gap 7: Settings 页 email 编辑

### 问题

后端 `HandleUpdateProfile` 已支持 email 更新，前端 Settings 无入口。

### 设计

- `api.ts` updateProfile 类型加 `email?: string`
- `UserSettingsPage.tsx` profile 区域加 email input
- i18n 加翻译 key

---

## 实施总结

**完成日期**：2026-03-08

**实际改动**：

| 仓库 | 文件数 | 行数变化 |
|------|--------|---------|
| agent-native-im-sdk-python | 7 | +153/-5 |
| agent-native-im-web | 8 | +68/-30 |
| agent-native-im (后端) | 已有 email 登录支持 | — |

**关键 Commit**：
- SDK: `feat: v4.0 agent onboarding — key lifecycle, file download, group filtering, LLM example`
- Frontend: `feat: v4.0 onboarding polish — orphan stream cleanup, LLM tutorial, email settings`
- Backend: `feat: smart email/username login with email column` (已在 v3.5 迭代完成)

所有 7 个 Gap 均已修复，平台达到 Agent 自主接入就绪状态。
