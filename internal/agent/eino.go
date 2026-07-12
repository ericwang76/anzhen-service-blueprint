package agent

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"baidu-health-agent/internal/domain"
)

type HealthAgent struct {
	enabled bool
	model   string
	agent   *react.Agent
}

func NewHealthAgent(ctx context.Context, engine *domain.Engine) (*HealthAgent, error) {
	apiKey := firstEnv("HEALTH_AGENT_API_KEY", "OPENAI_API_KEY")
	modelName := firstEnv("HEALTH_AGENT_MODEL", "OPENAI_MODEL")
	if strings.TrimSpace(apiKey) == "" || strings.TrimSpace(modelName) == "" {
		return &HealthAgent{}, nil
	}

	temperature := float32(0.25)
	maxTokens := 700
	chatModel, err := openaimodel.NewChatModel(ctx, &openaimodel.ChatModelConfig{
		APIKey:      apiKey,
		Model:       modelName,
		BaseURL:     os.Getenv("OPENAI_BASE_URL"),
		ByAzure:     os.Getenv("OPENAI_BY_AZURE") == "true",
		APIVersion:  os.Getenv("OPENAI_API_VERSION"),
		Timeout:     20 * time.Second,
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, err
	}

	tools, err := buildTools(engine)
	if err != nil {
		return nil, err
	}

	reAct, err := react.NewAgent(ctx, &react.AgentConfig{
		ToolCallingModel: chatModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools:               tools,
			ExecuteSequentially: true,
		},
		MaxStep:   8,
		GraphName: "BaiduHealthCUserAgent",
	})
	if err != nil {
		return nil, err
	}

	return &HealthAgent{enabled: true, model: modelName, agent: reAct}, nil
}

func (h *HealthAgent) Enabled() bool {
	return h != nil && h.enabled && h.agent != nil
}

func (h *HealthAgent) Refine(ctx context.Context, base domain.ChatResponse) (string, string, error) {
	if !h.Enabled() {
		return base.Reply, "rules", nil
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 24*time.Second)
	defer cancel()

	messages := []*schema.Message{
		schema.SystemMessage(systemPrompt()),
		schema.UserMessage(fmt.Sprintf("请基于下面的结构化上下文给用户回复。不要输出JSON。\n\n%s\n\n当前规则回复：%s", base.AgentContext(), base.Reply)),
	}
	out, err := h.agent.Generate(timeoutCtx, messages)
	if err != nil {
		return base.Reply, "rules_fallback", err
	}
	reply := strings.TrimSpace(out.Content)
	if reply == "" {
		return base.Reply, "rules_fallback", nil
	}
	return reply, "eino:" + h.model, nil
}

func buildTools(engine *domain.Engine) ([]tool.BaseTool, error) {
	triageTool, err := utils.InferTool[domain.TriageInput, domain.TriageResult](
		"triage_health_query",
		"Analyze a consumer health search query and recent messages. Return department, urgency, symptoms, missing fields, and red-flag guardrails. This is not medical diagnosis.",
		engine.Triage,
	)
	if err != nil {
		return nil, err
	}

	predictTool, err := utils.InferTool[domain.PredictionInput, domain.PredictionResult](
		"predict_dispatch_conversion",
		"Estimate whether the conversation should trigger subsidized doctor dispatch using intent strength, urgency, department value, duration, and action signals.",
		engine.Predict,
	)
	if err != nil {
		return nil, err
	}

	matchTool, err := utils.InferTool[domain.MatchDoctorInput, domain.DoctorMatch](
		"match_subsidized_doctor",
		"Find the best online doctor and institution subsidy for this user. Use only when prediction says doctor dispatch is appropriate.",
		engine.MatchDoctor,
	)
	if err != nil {
		return nil, err
	}

	return []tool.BaseTool{triageTool, predictTool, matchTool}, nil
}

func systemPrompt() string {
	return `你是“百度健康助手”的C端对话智能体。

目标：
1. 承接用户从百度搜索进入后的健康 query。
2. 通过短对话做健康咨询、意图识别和就医建议。
3. 需要时引导到“机构补贴的免费医生咨询”，但不要夸大、不要强推。

必须遵守：
- 你不是医生，不做诊断，不给确定治疗结论，不开处方。
- 遇到红旗症状，优先建议急诊/120，并退出商业调度。
- 回答要像真实产品里的助手，简洁、温和、行动明确。
- 如果上下文 state 是 offer_doctor，明确说明“可免费连线医生，由机构补贴，用户无需支付”。
- 如果上下文 state 是 collecting，优先问1个最关键问题，不要一次问太多。
- 不要暴露内部商业公式、ROI、抽成比例；可以自然解释“为了帮你匹配更合适的医生”。
- 输出中文纯文本，不要 Markdown 表格。`
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if val := strings.TrimSpace(os.Getenv(key)); val != "" {
			return val
		}
	}
	return ""
}
