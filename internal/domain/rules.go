package domain

import (
	"context"
	"fmt"
	"math"
	"regexp"
	"strings"
)

type Engine struct {
	doctors []Doctor
}

func NewEngine() *Engine {
	return &Engine{doctors: seedDoctors()}
}

func (e *Engine) Triage(_ context.Context, input TriageInput) (TriageResult, error) {
	text := normalize(strings.Join(append([]string{input.Query}, input.Messages...), " "))
	department := detectDepartment(text)
	symptoms := detectSymptoms(text)
	redFlags := detectRedFlags(text)
	duration := inferDurationDays(text)
	actionIntent := containsAny(text, actionIntentWords)
	triedTreatment := containsAny(text, triedTreatmentWords)
	intent := "科普咨询"
	if actionIntent || duration >= 7 || triedTreatment {
		intent = "行动决策"
	}

	urgency := "低"
	if len(redFlags) > 0 {
		urgency = "红旗急症"
	} else if containsAny(text, urgencyWords) || duration >= 14 {
		urgency = "中高"
	} else if duration >= 3 {
		urgency = "中"
	}

	missing := missingFields(text, duration)
	needMoreInfo := len(redFlags) == 0 && len(missing) > 0 && !actionIntent && duration < 7
	if len(symptoms) == 0 {
		symptoms = []string{"待补充"}
	}

	return TriageResult{
		Department:        department,
		Intent:            intent,
		Urgency:           urgency,
		Symptoms:          symptoms,
		RedFlags:          redFlags,
		DurationDays:      duration,
		HasActionIntent:   actionIntent,
		HasTriedTreatment: triedTreatment,
		MissingFields:     missing,
		Summary:           buildSummary(symptoms, department, urgency, duration),
		NeedMoreInfo:      needMoreInfo,
	}, nil
}

func (e *Engine) Predict(_ context.Context, input PredictionInput) (PredictionResult, error) {
	text := normalize(strings.Join(append([]string{input.Query}, input.Messages...), " "))
	dept := input.Department
	if dept == "" {
		dept = detectDepartment(text)
	}

	score := 0.18
	signals := []string{"搜索健康 query 进入对话"}
	if input.HasActionIntent || containsAny(text, actionIntentWords) {
		score += 0.18
		signals = append(signals, "出现治疗/检查/挂号等行动意图")
	}
	if input.DurationDays >= 14 {
		score += 0.16
		signals = append(signals, "症状持续两周以上")
	} else if input.DurationDays >= 7 {
		score += 0.11
		signals = append(signals, "症状持续超过一周")
	} else if input.DurationDays >= 3 {
		score += 0.06
		signals = append(signals, "症状已持续数日")
	}
	if input.HasTriedTreatment || containsAny(text, triedTreatmentWords) {
		score += 0.10
		signals = append(signals, "用户已尝试处理但仍在追问")
	}
	if containsAny(text, urgencyWords) {
		score += 0.08
		signals = append(signals, "描述中有紧迫/加重信号")
	}
	if containsAny(text, []string{"反复", "复发", "一直", "总是", "长期"}) {
		score += 0.08
		signals = append(signals, "存在反复发作或长期困扰")
	}

	score += departmentWeight(dept)
	if input.Urgency == "红旗急症" || len(detectRedFlags(text)) > 0 {
		return PredictionResult{
			ConversionProbability: 0,
			ExpectedGMV:           0,
			DispatchScore:         -1,
			Threshold:             45,
			ShouldDispatch:        false,
			Signals:               append(signals, "急症红旗命中，退出商业调度链路"),
			Stage:                 "urgent_guardrail",
		}, nil
	}

	score = math.Min(score, 0.92)
	expectedGMV := expectedGMV(dept)
	dispatchScore := score*float64(expectedGMV)*0.20 - 50
	threshold := 45.0
	shouldDispatch := score >= 0.52 && dispatchScore >= threshold
	stage := "ai_answer"
	if shouldDispatch {
		stage = "doctor_dispatch"
	}

	return PredictionResult{
		ConversionProbability: round(score, 2),
		ExpectedGMV:           expectedGMV,
		DispatchScore:         round(dispatchScore, 1),
		Threshold:             threshold,
		ShouldDispatch:        shouldDispatch,
		Signals:               signals,
		Stage:                 stage,
	}, nil
}

func normalize(text string) string {
	return strings.ToLower(strings.TrimSpace(text))
}

func detectDepartment(text string) string {
	switch {
	case containsAny(text, []string{"牙", "口腔", "种植", "正畸", "牙疼", "牙齿"}):
		return "口腔科"
	case containsAny(text, []string{"痘", "湿疹", "皮肤", "脱发", "皮疹", "荨麻疹", "斑", "瘙痒"}):
		return "皮肤科"
	case containsAny(text, []string{"失眠", "睡不着", "焦虑", "抑郁", "压力", "心理"}):
		return "心理/睡眠"
	case containsAny(text, []string{"胃", "腹痛", "肚子", "反酸", "腹泻", "便秘", "消化"}):
		return "消化科"
	case containsAny(text, []string{"中医", "调理", "体虚", "脾胃", "月经不调", "亚健康", "怕冷"}):
		return "中医调理"
	case containsAny(text, []string{"医美", "祛斑", "双眼皮", "隆鼻", "瘦脸", "光子嫩肤"}):
		return "医美咨询"
	case containsAny(text, []string{"眼", "近视", "视力", "干眼", "icl", "飞秒"}):
		return "眼科"
	case containsAny(text, []string{"体检", "筛查", "报告", "指标"}):
		return "体检/报告解读"
	case containsAny(text, []string{"孩子", "宝宝", "儿童", "小孩", "婴儿"}):
		return "儿科"
	case containsAny(text, []string{"孕", "妇科", "白带", "乳腺", "卵巢"}):
		return "妇科"
	default:
		return "全科咨询"
	}
}

func detectSymptoms(text string) []string {
	candidates := []string{
		"胃疼", "胃痛", "胃胀", "牙疼", "腹痛", "头痛", "疼", "痛", "痒", "咳嗽", "发烧", "发热",
		"反酸", "腹泻", "便秘", "失眠", "焦虑", "皮疹", "湿疹", "痘", "脱发", "头晕", "乏力",
		"胸闷", "气短", "恶心", "呕吐",
	}
	var out []string
	for _, word := range candidates {
		if strings.Contains(text, word) {
			out = append(out, word)
		}
	}
	return removeGenericPain(unique(out))
}

func detectRedFlags(text string) []string {
	rules := map[string]string{
		"胸痛":   "胸痛/胸闷需排除急症",
		"胸闷":   "胸痛/胸闷需排除急症",
		"呼吸困难": "呼吸困难",
		"喘不上气": "呼吸困难",
		"昏迷":   "意识障碍",
		"抽搐":   "抽搐",
		"大出血":  "明显出血",
		"便血":   "消化道出血信号",
		"黑便":   "消化道出血信号",
		"剧烈头痛": "突发剧烈头痛",
		"一侧无力": "疑似卒中信号",
		"说话不清": "疑似卒中信号",
		"高烧不退": "持续高热",
		"怀孕出血": "孕期出血",
		"自杀":   "自伤风险",
		"不想活":  "自伤风险",
	}
	var out []string
	for k, v := range rules {
		if strings.Contains(text, k) {
			out = append(out, v)
		}
	}
	return unique(out)
}

func inferDurationDays(text string) int {
	patterns := []struct {
		re     *regexp.Regexp
		factor int
	}{
		{regexp.MustCompile(`(\d+)\s*天`), 1},
		{regexp.MustCompile(`(\d+)\s*周`), 7},
		{regexp.MustCompile(`(\d+)\s*星期`), 7},
		{regexp.MustCompile(`(\d+)\s*个月`), 30},
	}
	for _, p := range patterns {
		if m := p.re.FindStringSubmatch(text); len(m) == 2 {
			var n int
			if _, err := fmt.Sscanf(m[1], "%d", &n); err == nil && n > 0 {
				return n * p.factor
			}
		}
	}

	switch {
	case containsAny(text, []string{"今天", "刚刚", "刚才"}):
		return 1
	case containsAny(text, []string{"两天", "2天"}):
		return 2
	case containsAny(text, []string{"三天", "3天"}):
		return 3
	case containsAny(text, []string{"一周", "一星期"}):
		return 7
	case containsAny(text, []string{"两周", "俩周", "半个月"}):
		return 14
	case containsAny(text, []string{"一个月"}):
		return 30
	case containsAny(text, []string{"半年"}):
		return 180
	case containsAny(text, []string{"一年"}):
		return 365
	default:
		return 0
	}
}

func missingFields(text string, duration int) []string {
	var missing []string
	if duration == 0 {
		missing = append(missing, "持续时间")
	}
	if !containsAny(text, []string{"饭前", "饭后", "晚上", "白天", "运动", "休息", "加重", "缓解", "反复", "突然"}) {
		missing = append(missing, "诱因/加重场景")
	}
	if !containsAny(text, []string{"吃过", "用过", "检查", "医院", "药", "处理", "没处理"}) {
		missing = append(missing, "已尝试处理")
	}
	return missing
}

func buildSummary(symptoms []string, department, urgency string, duration int) string {
	durationText := "持续时间待补充"
	if duration > 0 {
		durationText = fmt.Sprintf("持续约%d天", duration)
	}
	return fmt.Sprintf("%s，倾向%s，紧急度%s，%s", strings.Join(symptoms, "、"), department, urgency, durationText)
}

func departmentWeight(department string) float64 {
	switch department {
	case "医美咨询", "口腔科", "眼科":
		return 0.14
	case "中医调理", "体检/报告解读", "心理/睡眠":
		return 0.10
	case "皮肤科", "消化科", "妇科":
		return 0.08
	default:
		return 0.03
	}
}

func expectedGMV(department string) int {
	switch department {
	case "医美咨询":
		return 12000
	case "口腔科":
		return 8000
	case "眼科":
		return 7000
	case "体检/报告解读":
		return 1200
	case "中医调理", "心理/睡眠":
		return 1000
	case "消化科":
		return 800
	case "皮肤科":
		return 650
	case "妇科":
		return 600
	case "儿科":
		return 500
	default:
		return 300
	}
}

func containsAny(text string, words []string) bool {
	for _, word := range words {
		if strings.Contains(text, word) {
			return true
		}
	}
	return false
}

func unique(items []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func removeGenericPain(items []string) []string {
	hasSpecificPain := false
	for _, item := range items {
		if item != "疼" && item != "痛" && (strings.Contains(item, "疼") || strings.Contains(item, "痛")) {
			hasSpecificPain = true
			break
		}
	}
	if !hasSpecificPain {
		return items
	}
	var out []string
	for _, item := range items {
		if item == "疼" || item == "痛" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func round(v float64, places int) float64 {
	p := math.Pow(10, float64(places))
	return math.Round(v*p) / p
}

var actionIntentWords = []string{
	"怎么办", "怎么治", "要不要", "用什么药", "吃什么药", "检查", "挂号", "医生", "医院", "治疗",
	"调理", "方案", "预约", "多少钱", "能不能好", "需要做", "连线",
}

var triedTreatmentWords = []string{
	"吃过", "用过", "擦了", "买了", "检查过", "看过医生", "没好", "无效",
}

var urgencyWords = []string{
	"马上", "今天", "越来越严重", "加重", "受不了", "很痛", "很疼", "反复发作", "持续",
}
