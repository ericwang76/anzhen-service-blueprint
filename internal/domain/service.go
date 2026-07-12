package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

type Service struct {
	engine   *Engine
	mu       sync.Mutex
	sessions map[string]*Session
}

func NewService(engine *Engine) *Service {
	return &Service{
		engine:   engine,
		sessions: map[string]*Session{},
	}
}

func (s *Service) Handle(ctx context.Context, req ChatRequest) (ChatResponse, error) {
	now := time.Now()
	session := s.upsertSession(req, now)
	userText := strings.TrimSpace(req.Message)
	if userText == "" {
		userText = strings.TrimSpace(req.Query)
	}
	if userText == "" {
		userText = "我想咨询一下健康问题"
	}

	s.mu.Lock()
	session.Messages = append(session.Messages, Message{Role: "user", Content: userText, At: now})
	session.UpdatedAt = now
	userMessages := collectUserMessages(session)
	query := session.Query
	profile := session.Profile
	s.mu.Unlock()

	triage, err := s.engine.Triage(ctx, TriageInput{Query: query, Messages: userMessages, City: profile.City})
	if err != nil {
		return ChatResponse{}, err
	}
	prediction, err := s.engine.Predict(ctx, PredictionInput{
		Query:             query,
		Messages:          userMessages,
		Department:        triage.Department,
		Urgency:           triage.Urgency,
		DurationDays:      triage.DurationDays,
		HasActionIntent:   triage.HasActionIntent,
		HasTriedTreatment: triage.HasTriedTreatment,
	})
	if err != nil {
		return ChatResponse{}, err
	}

	var doctor *DoctorMatch
	if prediction.ShouldDispatch {
		match, err := s.engine.MatchDoctor(ctx, MatchDoctorInput{
			Department:            triage.Department,
			ConversionProbability: prediction.ConversionProbability,
			ExpectedGMV:           prediction.ExpectedGMV,
			City:                  profile.City,
		})
		if err != nil {
			return ChatResponse{}, err
		}
		if match.Doctor.ID != "" {
			doctor = &match
		}
	}

	state, reply, quickReplies := composeReply(userText, len(userMessages), triage, prediction, doctor)
	resp := ChatResponse{
		SessionID:    session.ID,
		Reply:        reply,
		State:        state,
		Mode:         "rules",
		Assessment:   triage,
		Prediction:   prediction,
		Doctor:       doctor,
		QuickReplies: quickReplies,
		Notices: []string{
			"AI仅做健康咨询和就医建议，不替代医生诊断。",
			"命中急症红旗时会退出商业调度链路，优先建议线下急诊或120。",
		},
		Funnel:       buildFunnel(query, state, triage, prediction, doctor),
		UpdatedAt:    now,
		agentContext: buildAgentContext(query, userMessages, triage, prediction, doctor, state),
	}

	return resp, nil
}

func (s *Service) RecordAssistant(sessionID, reply string, state ConversationState) {
	if strings.TrimSpace(sessionID) == "" || strings.TrimSpace(reply) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	session.Messages = append(session.Messages, Message{Role: "assistant", Content: reply, At: time.Now()})
	session.UpdatedAt = time.Now()
	_ = state
}

func (s *Service) Reset(sessionID string) {
	if strings.TrimSpace(sessionID) == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

func (s *Service) Engine() *Engine {
	return s.engine
}

func (s *Service) upsertSession(req ChatRequest, now time.Time) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" {
		sessionID = newSessionID()
	}
	session, ok := s.sessions[sessionID]
	if !ok {
		session = &Session{ID: sessionID, CreatedAt: now}
		s.sessions[sessionID] = session
	}
	if req.Query != "" && session.Query == "" {
		session.Query = req.Query
	}
	if session.Query == "" {
		session.Query = req.Message
	}
	if req.Profile.City != "" || req.Profile.AgeBand != "" || req.Profile.SearchTag != "" {
		session.Profile = req.Profile
	}
	session.UpdatedAt = now
	return session
}

func collectUserMessages(session *Session) []string {
	var out []string
	for _, msg := range session.Messages {
		if msg.Role == "user" {
			out = append(out, msg.Content)
		}
	}
	return out
}

func composeReply(userText string, userTurns int, triage TriageResult, prediction PredictionResult, doctor *DoctorMatch) (ConversationState, string, []string) {
	if len(triage.RedFlags) > 0 {
		reply := fmt.Sprintf("你提到的情况里有%s。为了安全起见，我不建议继续在线等待或做商业转接，请尽快联系120或去最近急诊；如果身边有人，先让对方陪同观察。", strings.Join(triage.RedFlags, "、"))
		return StateUrgentCare, reply, []string{"我已联系急诊", "还有其他症状", "查看就医准备"}
	}

	if isDoctorAcceptance(userText) && doctor != nil {
		reply := fmt.Sprintf("已为你优先匹配%s%s（%s）。%s医生会先看AI整理的病情摘要：%s。你可以直接补充检查报告、照片或最担心的问题。", doctor.Doctor.Name, doctor.Doctor.Title, doctor.Doctor.Institution.Name, doctor.SubsidyText, triage.Summary)
		return StateDoctorConnected, reply, []string{"上传检查报告", "我想问用药", "能否到店检查"}
	}

	if triage.NeedMoreInfo && userTurns <= 2 {
		question := nextQuestion(triage)
		reply := fmt.Sprintf("我先帮你做初步判断。现在看到的是：%s。%s", triage.Summary, question)
		return StateCollecting, reply, []string{"持续一周内", "持续两周以上", "已经用过药"}
	}

	if prediction.ShouldDispatch && doctor != nil {
		reply := fmt.Sprintf("从你的描述看，%s，已经不只是泛泛科普问题。建议让%s%s先免费帮你判断是否需要检查或到店处理。%s", triage.Summary, doctor.Doctor.Department, doctor.Doctor.Name, doctor.SubsidyText)
		return StateOfferDoctor, reply, []string{"同意免费连线", "先给我居家建议", "我想看附近机构"}
	}

	reply := educationReply(triage)
	state := StateEducation
	if triage.HasActionIntent || prediction.ConversionProbability >= 0.45 {
		state = StateFollowupGuidance
	}
	return state, reply, []string{"症状加重怎么办", "需要挂什么科", "帮我判断是否要就医"}
}

func nextQuestion(triage TriageResult) string {
	if contains("持续时间", triage.MissingFields) {
		return "请问这种情况持续多久了，是突然出现还是反复发作？"
	}
	if contains("诱因/加重场景", triage.MissingFields) {
		return "它通常在什么情况下更明显，比如饭后、夜间、运动后，还是没有规律？"
	}
	if contains("已尝试处理", triage.MissingFields) {
		return "你之前用过药、做过检查或采取过什么处理吗，效果怎么样？"
	}
	return "可以补充持续时间、加重场景和已经处理过的方法吗？"
}

func educationReply(triage TriageResult) string {
	base := fmt.Sprintf("根据目前信息，更像是%s方向的健康咨询，紧急度%s。", triage.Department, triage.Urgency)
	return base + "我建议先记录症状出现时间、诱因、是否加重，以及是否伴随发热、出血、呼吸困难等红旗信号。若症状持续不缓解、反复出现或影响生活，建议在线问医生或线下就诊进一步确认。"
}

func buildFunnel(query string, state ConversationState, triage TriageResult, prediction PredictionResult, doctor *DoctorMatch) []FunnelStep {
	dispatchStatus := "pending"
	if state == StateOfferDoctor || state == StateDoctorConnected {
		dispatchStatus = "active"
	} else if state == StateUrgentCare {
		dispatchStatus = "blocked"
	}
	doctorStatus := "pending"
	if doctor != nil {
		doctorStatus = "done"
	}
	if state == StateDoctorConnected {
		doctorStatus = "active"
	}

	return []FunnelStep{
		{Key: "search", Label: "搜索Query", Status: "done", Value: query},
		{Key: "triage", Label: "AI预采集", Status: "done", Value: triage.Department},
		{Key: "predict", Label: "转化预测", Status: "done", Value: fmt.Sprintf("%.0f%%", prediction.ConversionProbability*100)},
		{Key: "dispatch", Label: "医生调度", Status: dispatchStatus, Value: prediction.Stage},
		{Key: "subsidy", Label: "机构补贴", Status: doctorStatus, Value: subsidyValue(doctor)},
	}
}

func subsidyValue(doctor *DoctorMatch) string {
	if doctor == nil {
		return "待触发"
	}
	return fmt.Sprintf("%s ¥%d", doctor.Doctor.Institution.Name, doctor.Doctor.ConsultationSubsidy)
}

func buildAgentContext(query string, messages []string, triage TriageResult, prediction PredictionResult, doctor *DoctorMatch, state ConversationState) string {
	payload := map[string]any{
		"query":      query,
		"messages":   messages,
		"triage":     triage,
		"prediction": prediction,
		"doctor":     doctor,
		"state":      state,
	}
	data, _ := json.Marshal(payload)
	return string(data)
}

func isDoctorAcceptance(text string) bool {
	return containsAny(normalize(text), []string{"同意", "可以", "连线", "需要医生", "免费", "接诊", "好", "要"})
}

func contains(target string, items []string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func newSessionID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
