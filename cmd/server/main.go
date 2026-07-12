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
)

func main() {
	engine := domain.NewEngine()
	service := domain.NewService(engine)

	aiAgent, err := healthagent.NewHealthAgent(context.Background(), engine)
	if err != nil {
		log.Printf("Eino agent disabled: %v", err)
		aiAgent = &healthagent.HealthAgent{}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/chat", chatHandler(service, aiAgent))
	mux.HandleFunc("/api/reset", resetHandler(service))
	mux.HandleFunc("/api/meta", metaHandler(aiAgent))
	mux.Handle("/", spaHandler("web"))

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           withRequestLog(mux),
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("百度健康助手 MVP listening on http://localhost:%s", port)
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

func spaHandler(root string) http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(root))
	return func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if path == "." || path == "/" {
			http.ServeFile(w, r, filepath.Join(root, "index.html"))
			return
		}
		fullPath := filepath.Join(root, path)
		if stat, err := os.Stat(fullPath); err == nil && !stat.IsDir() {
			fileServer.ServeHTTP(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(root, "index.html"))
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
