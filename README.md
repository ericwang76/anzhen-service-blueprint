# 安诊 · 真人医生咨询室 MVP

一个面向搜索场景的真人医生核验与图文问诊原型。用户把搜索 Query 和存疑内容带入咨询室；医生智能助手完成必要追问，真人医生在医生端查看全部有效信息、选择或编辑 Copilot 建议后发送。付费问诊会继续进入预问诊、限时真人 IM 与问诊小结。

## 已实现

- Go 后端 + 可操作的患者端、医生端，一条命令本地运行。
- 真实的本地状态流转：核验追问 → 医生核验 → 模拟支付 → 预问诊 → 接诊 IM → 问诊小结。
- 医生端订单中心、完整病例上下文、左右滑动 Copilot 回答候选、直发/编辑后发。
- 每笔病例沉淀完整 Trace：用户输入、AI 生成、Copilot 生成与医生动作。
- 本地规则兜底；未配置模型 Key 也可完整演示。
- 医疗安全边界：红旗症状直接建议急诊/120，不进入正常服务链路。

## 运行

```bash
go run ./cmd/server
```

打开可操作 MVP：

```text
http://localhost:8080/
```

查看此前沉淀的双端服务设计蓝图：

```text
http://localhost:8080/blueprint
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

## MVP API

| 功能 | 接口 |
| --- | --- |
| 创建核验咨询室 | `POST /api/mvp/cases` |
| 查看病例 | `GET /api/mvp/cases/{id}` |
| 患者发送消息 | `POST /api/mvp/cases/{id}/message` |
| 提交真人核验 | `POST /api/mvp/cases/{id}/submit-check` |
| 医生订单中心 | `GET /api/mvp/orders` |
| 获取核验 Copilot 候选 | `GET /api/mvp/cases/{id}/doctor/drafts` |
| 医生发核验回复 | `POST /api/mvp/cases/{id}/doctor/send-check` |
| 模拟支付 | `POST /api/mvp/cases/{id}/pay` |
| 医生接诊 | `POST /api/mvp/cases/{id}/accept` |
| 生成 IM Copilot 建议 | `POST /api/mvp/cases/{id}/copilot` |
| 医生发送 IM / 结束问诊 | `POST /api/mvp/cases/{id}/doctor/message`、`POST /api/mvp/cases/{id}/end` |
| 查看完整 Trace | `GET /api/mvp/cases/{id}/trace` |

病例、消息、订单与 Trace 会持久化到 `DATA_DIR/consultation-cases.json`；单机部署重启不会丢失。正式生产环境建议把 `CaseStore` 换成受控的 MySQL/PostgreSQL，并接入对象存储、密钥管理、审计与备份。

## 生产配置与部署

复制 `.env.example` 为 `.env`，至少配置：

```bash
APP_ENV=production
APP_SESSION_SECRET=一段足够长的随机字符串
DOCTOR_PORTAL_CODE=给医生端分发的访问口令
```

若接入模型，再填写 `CONSULTATION_AI_API_KEY`、`CONSULTATION_AI_MODEL` 和兼容接口的 `OPENAI_BASE_URL`。Copilot 生成内容只会展示给医生，医生确认后才会发给患者；模型不可用时会自动使用规则兜底。

以容器启动：

```bash
docker compose up -d --build
```

患者端：`/`；医生端：`/doctor`。两端需要分别在各自设备或浏览器会话中访问。当前实现提供匿名患者会话和医生口令入口；接入生产时应替换为短信/微信登录、医生资质认证、真实支付回调和持久化数据库。

## 旧版 Agent API

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
