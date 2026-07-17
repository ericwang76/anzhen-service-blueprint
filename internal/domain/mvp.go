package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type CaseStatus string

const (
	CaseCollecting CaseStatus = "collecting"
	CaseChecking   CaseStatus = "checking"
	CaseVerified   CaseStatus = "verified"
	CasePreConsult CaseStatus = "preconsult"
	CaseWaiting    CaseStatus = "waiting_doctor"
	CaseLive       CaseStatus = "live"
	CaseCompleted  CaseStatus = "completed"
	CaseUrgent     CaseStatus = "urgent"
)

type CaseMessage struct {
	ID      string    `json:"id"`
	Role    string    `json:"role"`
	Name    string    `json:"name"`
	Content string    `json:"content"`
	At      time.Time `json:"at"`
}

type TraceEvent struct {
	ID        string    `json:"id"`
	Turn      int       `json:"turn"`
	Actor     string    `json:"actor"`
	EventType string    `json:"event_type"`
	Content   string    `json:"content"`
	At        time.Time `json:"at"`
}

type CopilotDraft struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Content string `json:"content"`
}

// CopilotGenerator is intentionally kept behind the domain boundary. Production
// deployments can use a configured model provider, while local development keeps
// the same flow with deterministic clinical copy.
type CopilotGenerator interface {
	CheckDrafts(context.Context, ConsultationCase) ([]CopilotDraft, error)
	LiveReply(context.Context, ConsultationCase) (string, error)
}

type ConsultationCase struct {
	ID             string         `json:"id"`
	OwnerID        string         `json:"owner_id"`
	Query          string         `json:"query"`
	CheckContent   string         `json:"check_content"`
	Status         CaseStatus     `json:"status"`
	Department     string         `json:"department"`
	DoctorName     string         `json:"doctor_name"`
	Paid           bool           `json:"paid"`
	QuestionIndex  int            `json:"question_index"`
	PreQuestionIdx int            `json:"pre_question_index"`
	Answers        []string       `json:"answers"`
	PreAnswers     []string       `json:"pre_answers"`
	Messages       []CaseMessage  `json:"messages"`
	Drafts         []CopilotDraft `json:"drafts,omitempty"`
	Trace          []TraceEvent   `json:"trace"`
	LiveEndsAt     *time.Time     `json:"live_ends_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

type MVPService struct {
	mu      sync.RWMutex
	cases   map[string]*ConsultationCase
	store   CaseStore
	copilot CopilotGenerator
}

func (s *MVPService) SetCopilot(generator CopilotGenerator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.copilot = generator
}

func NewMVPService() *MVPService {
	return &MVPService{cases: map[string]*ConsultationCase{}}
}

func NewPersistentMVPService(dataDir string) (*MVPService, error) {
	store, err := NewFileCaseStore(dataDir)
	if err != nil {
		return nil, err
	}
	cases, err := store.Load()
	if err != nil {
		return nil, err
	}
	service := &MVPService{cases: map[string]*ConsultationCase{}, store: store}
	for _, c := range cases {
		if c != nil && c.ID != "" {
			service.cases[c.ID] = c
		}
	}
	return service, nil
}

func (s *MVPService) Create(query, checkContent string) (*ConsultationCase, error) {
	return s.CreateForOwner("", query, checkContent)
}

func (s *MVPService) CreateForOwner(ownerID, query, checkContent string) (*ConsultationCase, error) {
	query = strings.TrimSpace(query)
	checkContent = strings.TrimSpace(checkContent)
	if query == "" || checkContent == "" {
		return nil, fmt.Errorf("query and check content are required")
	}

	now := time.Now()
	c := &ConsultationCase{
		ID:           newCaseID(),
		OwnerID:      ownerID,
		Query:        query,
		CheckContent: checkContent,
		Status:       CaseCollecting,
		Department:   "内分泌科",
		DoctorName:   "林夏主任医师",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	c.appendMessage("patient", "你", fmt.Sprintf("我想请医生核一下：%s\n\n核验内容：%s", query, checkContent))
	c.addTrace("user", "user.check_input", query+" | "+checkContent)
	if hasUrgentSignal(query + " " + checkContent) {
		c.Status = CaseUrgent
		c.appendMessage("assistant", "医生智能助手", "你提到的情况可能存在急症风险。请立即联系 120 或前往附近急诊；此时不建议等待线上核验。")
		c.addTrace("assistant", "assistant.safety_escalation", "急症风险提示")
	} else {
		c.appendMessage("assistant", "医生智能助手", "你好，我已经把你的搜索问题和想核验的内容带进咨询室，林医生可以直接看到。为了让判断更贴合你，我再问 3 个小问题。\n\n"+verificationQuestions[0])
		c.addTrace("assistant", "assistant.generate_question", verificationQuestions[0])
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cases[c.ID] = c
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return cloneCase(c), nil
}

func (s *MVPService) CanAccess(id, ownerID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.cases[id]
	return ok && c.OwnerID == ownerID
}

func (s *MVPService) Get(id string) (*ConsultationCase, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.cases[id]
	if !ok {
		return nil, fmt.Errorf("case not found")
	}
	return cloneCase(c), nil
}

func (s *MVPService) PatientMessage(id, content string) (*ConsultationCase, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("message is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.caseLocked(id)
	if err != nil {
		return nil, err
	}
	if c.Status == CaseUrgent || c.Status == CaseCompleted {
		return cloneCase(c), nil
	}
	c.appendMessage("patient", "你", content)
	c.addTrace("user", "user.input", content)

	switch c.Status {
	case CaseCollecting:
		c.Answers = append(c.Answers, content)
		c.QuestionIndex++
		if c.QuestionIndex < len(verificationQuestions) {
			q := verificationQuestions[c.QuestionIndex]
			c.appendMessage("assistant", "医生智能助手", q)
			c.addTrace("assistant", "assistant.generate_question", q)
		} else {
			c.appendMessage("assistant", "医生智能助手", "谢谢，关键信息已经补齐。我已经把百科原文、你的搜索问题和这 3 点情况整理好，你可以提交给林医生核验。")
			c.addTrace("assistant", "assistant.generate_handoff", "核验信息已整理")
		}
	case CasePreConsult:
		c.PreAnswers = append(c.PreAnswers, content)
		c.PreQuestionIdx++
		if c.PreQuestionIdx < len(preConsultQuestions) {
			q := preConsultQuestions[c.PreQuestionIdx]
			c.appendMessage("assistant", "医生智能助手", q)
			c.addTrace("assistant", "assistant.generate_preconsult_question", q)
		} else {
			c.Status = CaseWaiting
			c.appendMessage("assistant", "医生智能助手", "收到，我已把这次最想解决的问题、相关症状和当前用药整理给林医生。医生接诊后会直接进入真人对话。")
			c.addTrace("assistant", "assistant.generate_preconsult_summary", "预问诊摘要已生成")
		}
	case CaseWaiting:
		c.appendMessage("assistant", "医生智能助手", "这条补充我已同步到待接诊订单，林医生接诊后可以直接看到。")
		c.addTrace("assistant", "assistant.acknowledge_waiting_input", "同步补充信息")
	case CaseLive:
		c.appendMessage("assistant", "医生智能助手", "林医生正在真人会话中。这条内容已同步到会话记录。")
		c.addTrace("assistant", "assistant.sync_live_input", "同步真人会话")
	default:
		c.appendMessage("assistant", "医生智能助手", "收到。如果你希望林医生继续结合个人情况判断，可以发起图文问诊，本次信息无需重复填写。")
		c.addTrace("assistant", "assistant.reply", "付费问诊引导")
	}
	c.UpdatedAt = time.Now()
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return cloneCase(c), nil
}

func (s *MVPService) SubmitCheck(id string) (*ConsultationCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.caseLocked(id)
	if err != nil {
		return nil, err
	}
	if c.Status != CaseCollecting || len(c.Answers) < len(verificationQuestions) {
		return nil, fmt.Errorf("please complete the three verification questions first")
	}
	c.Status = CaseChecking
	c.appendMessage("assistant", "医生智能助手", "已提交给林医生核验。医生会结合你的个人情况确认百科内容是否适用，请稍等。")
	c.addTrace("assistant", "assistant.submit_check", "提交真人核验")
	c.UpdatedAt = time.Now()
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return cloneCase(c), nil
}

func (s *MVPService) DoctorOrders() []ConsultationCase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	orders := make([]ConsultationCase, 0, len(s.cases))
	for _, c := range s.cases {
		if c.Status == CaseChecking || c.Status == CaseWaiting || c.Status == CaseLive {
			orders = append(orders, *cloneCase(c))
		}
	}
	sort.Slice(orders, func(i, j int) bool { return orders[i].UpdatedAt.After(orders[j].UpdatedAt) })
	return orders
}

func (s *MVPService) Drafts(ctx context.Context, id string) ([]CopilotDraft, error) {
	s.mu.RLock()
	c, ok := s.cases[id]
	if !ok {
		s.mu.RUnlock()
		return nil, fmt.Errorf("case not found")
	}
	if c.Status != CaseChecking {
		s.mu.RUnlock()
		return nil, fmt.Errorf("case is not awaiting verification")
	}
	if len(c.Drafts) > 0 {
		drafts := append([]CopilotDraft(nil), c.Drafts...)
		s.mu.RUnlock()
		return drafts, nil
	}
	snapshot := *cloneCase(c)
	generator := s.copilot
	s.mu.RUnlock()

	drafts := checkDrafts(&snapshot)
	if generator != nil {
		if generated, err := generator.CheckDrafts(ctx, snapshot); err == nil && len(generated) == 2 {
			drafts = generated
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.caseLocked(id)
	if err != nil {
		return nil, err
	}
	if len(c.Drafts) == 0 {
		c.Drafts = drafts
		c.addTrace("copilot", "copilot.generate_check_candidates", "生成核验候选回答 A/B")
		c.UpdatedAt = time.Now()
		if err := s.persistLocked(); err != nil {
			return nil, err
		}
	}
	return append([]CopilotDraft(nil), c.Drafts...), nil
}

func (s *MVPService) SendCheck(id, content, action string) (*ConsultationCase, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("doctor reply is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.caseLocked(id)
	if err != nil {
		return nil, err
	}
	if c.Status != CaseChecking {
		return nil, fmt.Errorf("case is not awaiting verification")
	}
	c.Status = CaseVerified
	c.appendMessage("doctor", c.DoctorName, content)
	c.addTrace("doctor", "doctor."+normalizeAction(action), content)
	c.appendMessage("assistant", "医生智能助手", "林医生的核验回复已经送达。如果你想继续问饮食、用药或复查报告，可以发起图文问诊，之前的信息会自动带入。")
	c.addTrace("assistant", "assistant.generate_consultation_offer", "引导图文问诊")
	c.UpdatedAt = time.Now()
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return cloneCase(c), nil
}

func (s *MVPService) Pay(id string) (*ConsultationCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.caseLocked(id)
	if err != nil {
		return nil, err
	}
	if c.Status != CaseVerified {
		return nil, fmt.Errorf("verification must be completed before consultation payment")
	}
	c.Paid = true
	c.Status = CasePreConsult
	c.appendMessage("system", "系统", "支付成功 ¥39 · 图文问诊已创建")
	c.addTrace("user", "user.payment_success", "图文问诊 ¥39")
	c.appendMessage("assistant", "医生智能助手", "问诊已经创建，我正在通知林医生接诊。等待期间，我先帮你把最想解决的问题整理好。\n\n"+preConsultQuestions[0])
	c.addTrace("assistant", "assistant.generate_preconsult_question", preConsultQuestions[0])
	c.UpdatedAt = time.Now()
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return cloneCase(c), nil
}

func (s *MVPService) Accept(id string) (*ConsultationCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.caseLocked(id)
	if err != nil {
		return nil, err
	}
	if c.Status != CaseWaiting && c.Status != CasePreConsult {
		return nil, fmt.Errorf("consultation is not ready for acceptance")
	}
	now := time.Now()
	end := now.Add(30 * time.Minute)
	c.Status = CaseLive
	c.LiveEndsAt = &end
	c.appendMessage("system", "系统", "林夏医生已接诊 · 已开启 30 分钟限时图文问诊")
	c.addTrace("doctor", "doctor.accept_consultation", "医生接诊并开启限时会话")
	c.appendMessage("doctor", c.DoctorName, "你好，你前面补充的信息我都看到了。我们从饮食和是否需要用药开始聊。")
	c.addTrace("doctor", "doctor.live_message", "接诊首条消息")
	c.UpdatedAt = now
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return cloneCase(c), nil
}

func (s *MVPService) CopilotReply(ctx context.Context, id string) (CopilotDraft, error) {
	s.mu.RLock()
	c, ok := s.cases[id]
	if !ok {
		s.mu.RUnlock()
		return CopilotDraft{}, fmt.Errorf("case not found")
	}
	if c.Status != CaseLive {
		s.mu.RUnlock()
		return CopilotDraft{}, fmt.Errorf("live consultation is not active")
	}
	snapshot := *cloneCase(c)
	generator := s.copilot
	s.mu.RUnlock()

	content := "水果可以吃，建议放在两餐之间，每次约一个拳头大小；优先苹果、梨或莓果，先避免果汁和高糖水果。"
	for i := len(snapshot.Messages) - 1; i >= 0; i-- {
		if snapshot.Messages[i].Role == "patient" && strings.Contains(snapshot.Messages[i].Content, "复查") {
			content = "复查前请保持正常饮食和作息，不要刻意少吃；建议按医生安排复查空腹血糖和糖化血红蛋白，并带上报告一起评估。"
			break
		}
	}
	if generator != nil {
		if generated, err := generator.LiveReply(ctx, snapshot); err == nil && strings.TrimSpace(generated) != "" {
			content = strings.TrimSpace(generated)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.caseLocked(id)
	if err != nil {
		return CopilotDraft{}, err
	}
	if c.Status != CaseLive {
		return CopilotDraft{}, fmt.Errorf("live consultation is not active")
	}
	draft := CopilotDraft{ID: "copilot-live-" + newCaseID(), Label: "Copilot 推荐回复", Content: content}
	c.addTrace("copilot", "copilot.generate_live_reply", content)
	c.UpdatedAt = time.Now()
	if err := s.persistLocked(); err != nil {
		return CopilotDraft{}, err
	}
	return draft, nil
}

func (s *MVPService) DoctorMessage(id, content, action string) (*ConsultationCase, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, fmt.Errorf("doctor message is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.caseLocked(id)
	if err != nil {
		return nil, err
	}
	if c.Status != CaseLive {
		return nil, fmt.Errorf("live consultation is not active")
	}
	c.appendMessage("doctor", c.DoctorName, content)
	c.addTrace("doctor", "doctor."+normalizeAction(action), content)
	c.UpdatedAt = time.Now()
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return cloneCase(c), nil
}

func (s *MVPService) End(id string) (*ConsultationCase, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, err := s.caseLocked(id)
	if err != nil {
		return nil, err
	}
	if c.Status != CaseLive {
		return nil, fmt.Errorf("live consultation is not active")
	}
	c.Status = CaseCompleted
	c.appendMessage("system", "系统", "本次真人问诊已结束")
	c.addTrace("doctor", "doctor.end_consultation", "结束真人问诊")
	summary := "问诊小结：林医生建议目前先不自行用药，优先从晚餐主食减少约四分之一、增加蔬菜和蛋白质开始；水果放在两餐之间，每次约一个拳头大小。请在 1～2 周后复查空腹血糖和糖化血红蛋白。若出现明显口渴、多尿或体重下降，请提前就医。"
	c.appendMessage("assistant", "医生智能助手", summary)
	c.addTrace("assistant", "assistant.generate_consult_summary", summary)
	c.UpdatedAt = time.Now()
	if err := s.persistLocked(); err != nil {
		return nil, err
	}
	return cloneCase(c), nil
}

func (s *MVPService) persistLocked() error {
	if s.store == nil {
		return nil
	}
	cases := make([]*ConsultationCase, 0, len(s.cases))
	for _, c := range s.cases {
		cases = append(cases, cloneCase(c))
	}
	sort.Slice(cases, func(i, j int) bool { return cases[i].CreatedAt.Before(cases[j].CreatedAt) })
	return s.store.Save(cases)
}

func (s *MVPService) caseLocked(id string) (*ConsultationCase, error) {
	c, ok := s.cases[id]
	if !ok {
		return nil, fmt.Errorf("case not found")
	}
	return c, nil
}

func (c *ConsultationCase) appendMessage(role, name, content string) {
	c.Messages = append(c.Messages, CaseMessage{ID: newCaseID(), Role: role, Name: name, Content: content, At: time.Now()})
}

func (c *ConsultationCase) addTrace(actor, eventType, content string) {
	c.Trace = append(c.Trace, TraceEvent{ID: newCaseID(), Turn: len(c.Trace) + 1, Actor: actor, EventType: eventType, Content: content, At: time.Now()})
}

func cloneCase(c *ConsultationCase) *ConsultationCase {
	copy := *c
	copy.Answers = append([]string(nil), c.Answers...)
	copy.PreAnswers = append([]string(nil), c.PreAnswers...)
	copy.Messages = append([]CaseMessage(nil), c.Messages...)
	copy.Drafts = append([]CopilotDraft(nil), c.Drafts...)
	copy.Trace = append([]TraceEvent(nil), c.Trace...)
	if c.LiveEndsAt != nil {
		end := *c.LiveEndsAt
		copy.LiveEndsAt = &end
	}
	return &copy
}

func checkDrafts(c *ConsultationCase) []CopilotDraft {
	base := "这段百科内容基本准确。单次空腹血糖 6.3 mmol/L 高于一般参考上限，但不能据此诊断糖尿病。结合你是首次发现偏高且有糖尿病家族史，建议正常饮食状态下于 1～2 周内复查空腹血糖，并同时检查糖化血红蛋白（HbA1c）。"
	return []CopilotDraft{
		{ID: "check-a", Label: "回答 A · 简明结论", Content: base},
		{ID: "check-b", Label: "回答 B · 风险沟通", Content: "你看到的百科表述总体没有问题，但“可能属于空腹血糖受损”不等于已经患糖尿病。当前无需自行用药；建议按正常饮食状态复查，并在复查持续偏高时到内分泌科进一步评估。"},
	}
}

func hasUrgentSignal(text string) bool {
	return containsAny(strings.ToLower(text), []string{"呼吸困难", "意识不清", "昏迷", "胸痛", "120", "自杀"})
}

func normalizeAction(action string) string {
	if strings.TrimSpace(action) == "edit" {
		return "edit_and_send"
	}
	return "direct_send"
}

func newCaseID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

var verificationQuestions = []string{
	"这次 6.3 mmol/L 是在什么情况下测的？",
	"以前是否出现过血糖偏高？",
	"你是否有糖尿病家族史、超重或其他高风险因素？",
}

var preConsultQuestions = []string{
	"这次问诊，你最想让林医生帮你解决什么？",
	"最近有明显口渴、多尿或体重下降吗？",
	"现在有在用降糖药或其他长期药物吗？",
}
