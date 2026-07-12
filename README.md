# 百度健康助手 MVP

一个面向 C 端的 AI health agent 对话产品原型：用户从百度搜索 health query 进入后，系统先做短对话预采集，再根据急症红旗、行动意图、转化预测和医生池匹配，决定是 AI 科普、继续追问、退出急症链路，还是引导到机构补贴的免费医生咨询。

## 已实现

- Go 后端 + 静态前端，一条命令本地运行。
- CloudWeGo Eino ReAct Agent 封装，支持工具调用。
- 本地规则兜底：无模型 key 时仍可跑通完整产品流程。
- 智能体工具：健康 query 分诊、转化预测、补贴医生匹配。
- C 端对话 UI：搜索 query 入口、聊天、AI 漏斗、转化预测、医生承接卡片。
- 医疗安全边界：红旗症状直接建议急诊/120，不进入商业调度。

## 运行

```bash
go run ./cmd/server
```

打开：

```text
http://localhost:8080/?q=胃疼反复发作怎么办
```

## 接入模型

不配置模型时，系统使用本地规则模式。

要启用 Eino + 模型，把 `.env.example` 中的变量设置到环境里：

```bash
export HEALTH_AGENT_API_KEY="你的key"
export HEALTH_AGENT_MODEL="你的模型名"
export OPENAI_BASE_URL="你的OpenAI兼容endpoint"
go run ./cmd/server
```

如果使用标准 OpenAI，也可以用 `OPENAI_API_KEY` 和 `OPENAI_MODEL`；代码会优先读取 `HEALTH_AGENT_*`，再读取 `OPENAI_*`。

## 关键路径

- `cmd/server/main.go`: HTTP API 和静态资源服务。
- `internal/domain/rules.go`: 分诊、急症红旗和转化预测规则。
- `internal/domain/doctors.go`: mock 医生池、机构补贴和匹配排序。
- `internal/domain/service.go`: 会话、回复状态机和产品漏斗。
- `internal/agent/eino.go`: Eino ReAct Agent、工具注册和模型回复生成。
- `web/`: C 端对话产品界面。

## API

`POST /api/chat`

```json
{
  "session_id": "",
  "query": "胃疼反复发作怎么办",
  "message": "胃疼反复发作怎么办",
  "profile": {
    "city": "北京",
    "search_tag": "baidu_search"
  }
}
```

返回包含 `reply`、`assessment`、`prediction`、`doctor`、`funnel` 和 `quick_replies`。
