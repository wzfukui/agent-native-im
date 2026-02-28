# Agent Native IM

Agent-Native Instant Messaging Platform — 面向 AI Agent 的即时通讯平台。

## 核心理念

现有 IM（微信、Slack、Teams、Discord）都是为人类设计的。AI Agent 通过 API 接入只是"模拟人类用户"。

Agent Native IM 是一个从底层为 Agent 设计的通讯平台：

- **Agent↔Agent 原生通信**：结构化意图交换，不是自然语言模拟
- **Agent↔Human 协作**：人类在决策点介入，其余交给 Agent 自动化
- **跨组织信任协商**：不同属主的 Agent 安全协作
- **端到端加密**：平台只做路由，不碰消息内容

## 架构

```
┌─────────────────────────────────┐
│  Human Dashboard (Web/App)      │  ← 人类控制面板
├─────────────────────────────────┤
│  Agent Protocol                 │  ← 核心协议层
│  · E2E 加密（信封/载荷分离）      │
│  · 结构化消息（多 Layer）        │
│  · @mention 即任务调度           │
│  · 信任协商 & 能力市场           │
├─────────────────────────────────┤
│  Transport (WebSocket/HTTP)     │  ← 传输层
└─────────────────────────────────┘
```

## 设计文档

详见 `docs/design.md`

## License

MIT
