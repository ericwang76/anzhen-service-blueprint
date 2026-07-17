package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	healthagent "baidu-health-agent/internal/agent"
	"baidu-health-agent/internal/domain"
	"baidu-health-agent/internal/security"
)

func main() {
	engine := domain.NewEngine()
	service := domain.NewService(engine)
	dataDir := strings.TrimSpace(os.Getenv("DATA_DIR"))
	if dataDir == "" {
		dataDir = "data"
	}
	mvp, err := domain.NewPersistentMVPService(dataDir)
	if err != nil {
		log.Fatalf("initialize consultation store: %v", err)
	}
	production := strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production")
	auth, err := security.New(os.Getenv("APP_SESSION_SECRET"), os.Getenv("DOCTOR_PORTAL_CODE"), production)
	if err != nil {
		log.Fatalf("initialize product auth: %v", err)
	}

	aiAgent, err := healthagent.NewHealthAgent(context.Background(), engine)
	if err != nil {
		log.Printf("Eino agent disabled: %v", err)
		aiAgent = &healthagent.HealthAgent{}
	}
	consultationCopilot, err := healthagent.NewConsultationCopilot(context.Background())
	if err != nil {
		log.Printf("Consultation Copilot disabled: %v", err)
	} else if consultationCopilot != nil {
		mvp.SetCopilot(consultationCopilot)
		log.Printf("Consultation Copilot enabled")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", chatHandler(service, aiAgent))
	mux.HandleFunc("/api/reset", resetHandler(service))
	mux.HandleFunc("/api/meta", metaHandler(aiAgent))
	mux.HandleFunc("/api/auth/guest", guestHandler(auth))
	mux.HandleFunc("/api/auth/doctor/login", doctorLoginHandler(auth))
	mux.HandleFunc("/api/auth/me", meHandler(auth))
	mux.HandleFunc("/api/mvp/cases", mvpCasesHandler(mvp, auth))
	mux.HandleFunc("/api/mvp/cases/", mvpCaseHandler(mvp, auth))
	mux.HandleFunc("/api/mvp/orders", mvpOrdersHandler(mvp, auth))
	mux.HandleFunc("/blueprint", fileHandler(filepath.Join("web", "index.html")))
	mux.Handle("/", spaHandler("web", "product.html"))

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           withSecurityHeaders(withRequestLog(mux)),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("安诊服务 listening on http://localhost:%s (data: %s)", port, dataDir)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func chatHandler(service *domain.Service, aiAgent *healthagent.HealthAgent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req domain.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		resp, err := service.Handle(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if aiAgent.Enabled() {
			reply, mode, err := aiAgent.Refine(r.Context(), resp)
			resp.Reply = reply
			resp.Mode = mode
			if err != nil {
				resp.Notices = append(resp.Notices, "模型调用失败，本轮已使用本地规则兜底。")
				log.Printf("agent fallback: %v", err)
			}
		}

		service.RecordAssistant(resp.SessionID, resp.Reply, resp.State)
		writeJSON(w, http.StatusOK, resp)
	}
}

func resetHandler(service *domain.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			SessionID string `json:"session_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		service.Reset(body.SessionID)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	}
}

func metaHandler(aiAgent *healthagent.HealthAgent) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"agent": map[string]any{
				"framework": "CloudWeGo Eino ReAct",
				"enabled":   aiAgent.Enabled(),
			},
			"product": map[string]any{
				"name":    "百度健康助手",
				"version": "mvp-0.1",
			},
		})
	}
}

func guestHandler(auth *security.Auth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		identity, err := auth.Guest(w)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "无法创建访客会话")
			return
		}
		writeJSON(w, http.StatusCreated, identity)
	}
}

func doctorLoginHandler(auth *security.Auth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var body struct {
			Code string `json:"code"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		identity, err := auth.Doctor(w, body.Code)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, identity)
	}
}

func meHandler(auth *security.Auth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		identity, err := auth.Identity(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, identity)
	}
}

func mvpCasesHandler(service *domain.MVPService, auth *security.Auth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var body struct {
			Query        string `json:"query"`
			CheckContent string `json:"check_content"`
		}
		identity, err := auth.Require(r, "patient")
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		c, err := service.CreateForOwner(identity.Subject, body.Query, body.CheckContent)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, publicCase(c))
	}
}

func mvpOrdersHandler(service *domain.MVPService, auth *security.Auth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if _, err := auth.Require(r, "doctor"); err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		orders := service.DoctorOrders()
		views := make([]caseView, 0, len(orders))
		for i := range orders {
			views = append(views, publicCase(&orders[i]))
		}
		writeJSON(w, http.StatusOK, views)
	}
}

func mvpCaseHandler(service *domain.MVPService, auth *security.Auth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/mvp/cases/"), "/")
		parts := strings.Split(path, "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, http.StatusNotFound, "case not found")
			return
		}
		id := parts[0]
		action := ""
		if len(parts) > 1 {
			action = strings.Join(parts[1:], "/")
		}
		identity, err := auth.Identity(r)
		if err != nil {
			writeError(w, http.StatusUnauthorized, err.Error())
			return
		}
		isDoctorAction := strings.HasPrefix(action, "doctor/") || action == "accept" || action == "copilot" || action == "end" || action == "trace"
		if isDoctorAction && identity.Role != "doctor" {
			writeError(w, http.StatusForbidden, "仅医生可执行此操作")
			return
		}
		if !isDoctorAction && identity.Role == "patient" && !service.CanAccess(id, identity.Subject) {
			writeError(w, http.StatusNotFound, "case not found")
			return
		}
		if identity.Role != "doctor" && identity.Role != "patient" {
			writeError(w, http.StatusForbidden, "没有此操作权限")
			return
		}

		if r.Method == http.MethodGet && action == "" {
			c, err := service.Get(id)
			respondCase(w, c, err)
			return
		}
		if r.Method == http.MethodGet && action == "trace" {
			c, err := service.Get(id)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, c.Trace)
			return
		}
		if r.Method == http.MethodGet && action == "doctor/drafts" {
			drafts, err := service.Drafts(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, drafts)
			return
		}
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}

		var body struct {
			Content string `json:"content"`
			Action  string `json:"action"`
		}
		if action == "message" || action == "doctor/send-check" || action == "doctor/message" {
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&body); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON body")
				return
			}
		}

		switch action {
		case "message":
			c, err := service.PatientMessage(id, body.Content)
			respondCase(w, c, err)
		case "submit-check":
			c, err := service.SubmitCheck(id)
			respondCase(w, c, err)
		case "doctor/send-check":
			c, err := service.SendCheck(id, body.Content, body.Action)
			respondCase(w, c, err)
		case "pay":
			c, err := service.Pay(id)
			respondCase(w, c, err)
		case "accept":
			c, err := service.Accept(id)
			respondCase(w, c, err)
		case "copilot":
			draft, err := service.CopilotReply(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, draft)
		case "doctor/message":
			c, err := service.DoctorMessage(id, body.Content, body.Action)
			respondCase(w, c, err)
		case "end":
			c, err := service.End(id)
			respondCase(w, c, err)
		default:
			writeError(w, http.StatusNotFound, "unknown case action")
		}
	}
}

func respondCase(w http.ResponseWriter, c *domain.ConsultationCase, err error) {
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, publicCase(c))
}

type caseView struct {
	ID             string                `json:"id"`
	Query          string                `json:"query"`
	CheckContent   string                `json:"check_content"`
	Status         domain.CaseStatus     `json:"status"`
	Department     string                `json:"department"`
	DoctorName     string                `json:"doctor_name"`
	Paid           bool                  `json:"paid"`
	QuestionIndex  int                   `json:"question_index"`
	PreQuestionIdx int                   `json:"pre_question_index"`
	Answers        []string              `json:"answers"`
	PreAnswers     []string              `json:"pre_answers"`
	Messages       []domain.CaseMessage  `json:"messages"`
	LiveEndsAt     *time.Time            `json:"live_ends_at,omitempty"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

func publicCase(c *domain.ConsultationCase) caseView {
	return caseView{
		ID: c.ID, Query: c.Query, CheckContent: c.CheckContent, Status: c.Status,
		Department: c.Department, DoctorName: c.DoctorName, Paid: c.Paid,
		QuestionIndex: c.QuestionIndex, PreQuestionIdx: c.PreQuestionIdx,
		Answers: nonNilStrings(c.Answers), PreAnswers: nonNilStrings(c.PreAnswers), Messages: c.Messages,
		LiveEndsAt: c.LiveEndsAt, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func fileHandler(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, path)
	}
}

func spaHandler(root, fallback string) http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(root))
	return func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if path == "." || path == "/" {
			http.ServeFile(w, r, filepath.Join(root, fallback))
			return
		}
		fullPath := filepath.Join(root, path)
		if stat, err := os.Stat(fullPath); err == nil && !stat.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(root, fallback))
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
