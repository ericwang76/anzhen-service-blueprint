package domain

import "time"

type ConversationState string

const (
	StateCollecting       ConversationState = "collecting"
	StateEducation        ConversationState = "education"
	StateOfferDoctor      ConversationState = "offer_doctor"
	StateDoctorConnected  ConversationState = "doctor_connected"
	StateUrgentCare       ConversationState = "urgent_care"
	StateFollowupGuidance ConversationState = "followup_guidance"
)

type ChatRequest struct {
	SessionID string      `json:"session_id"`
	Query     string      `json:"query"`
	Message   string      `json:"message"`
	Profile   UserProfile `json:"profile,omitempty"`
}

type UserProfile struct {
	City      string `json:"city,omitempty"`
	AgeBand   string `json:"age_band,omitempty"`
	SearchTag string `json:"search_tag,omitempty"`
}

type ChatResponse struct {
	SessionID    string            `json:"session_id"`
	Reply        string            `json:"reply"`
	State        ConversationState `json:"state"`
	Mode         string            `json:"mode"`
	Assessment   TriageResult      `json:"assessment"`
	Prediction   PredictionResult  `json:"prediction"`
	Doctor       *DoctorMatch      `json:"doctor,omitempty"`
	QuickReplies []string          `json:"quick_replies"`
	Notices      []string          `json:"notices"`
	Funnel       []FunnelStep      `json:"funnel"`
	UpdatedAt    time.Time         `json:"updated_at"`

	agentContext string
}

func (r ChatResponse) AgentContext() string {
	return r.agentContext
}

type FunnelStep struct {
	Key    string `json:"key"`
	Label  string `json:"label"`
	Status string `json:"status"`
	Value  string `json:"value,omitempty"`
}

type Message struct {
	Role    string    `json:"role"`
	Content string    `json:"content"`
	At      time.Time `json:"at"`
}

type Session struct {
	ID        string
	Query     string
	Profile   UserProfile
	Messages  []Message
	CreatedAt time.Time
	UpdatedAt time.Time
}

type TriageInput struct {
	Query    string   `json:"query" jsonschema:"description=Original search query from Baidu or current user intent"`
	Messages []string `json:"messages" jsonschema:"description=Recent user messages in the health consultation"`
	City     string   `json:"city,omitempty" jsonschema:"description=User city if available"`
}

type TriageResult struct {
	Department        string   `json:"department"`
	Intent            string   `json:"intent"`
	Urgency           string   `json:"urgency"`
	Symptoms          []string `json:"symptoms"`
	RedFlags          []string `json:"red_flags"`
	DurationDays      int      `json:"duration_days"`
	HasActionIntent   bool     `json:"has_action_intent"`
	HasTriedTreatment bool     `json:"has_tried_treatment"`
	MissingFields     []string `json:"missing_fields"`
	Summary           string   `json:"summary"`
	NeedMoreInfo      bool     `json:"need_more_info"`
}

type PredictionInput struct {
	Query             string   `json:"query"`
	Messages          []string `json:"messages"`
	Department        string   `json:"department"`
	Urgency           string   `json:"urgency"`
	DurationDays      int      `json:"duration_days"`
	HasActionIntent   bool     `json:"has_action_intent"`
	HasTriedTreatment bool     `json:"has_tried_treatment"`
}

type PredictionResult struct {
	ConversionProbability float64  `json:"conversion_probability"`
	ExpectedGMV           int      `json:"expected_gmv"`
	DispatchScore         float64  `json:"dispatch_score"`
	Threshold             float64  `json:"threshold"`
	ShouldDispatch        bool     `json:"should_dispatch"`
	Signals               []string `json:"signals"`
	Stage                 string   `json:"stage"`
}

type MatchDoctorInput struct {
	Department            string  `json:"department"`
	ConversionProbability float64 `json:"conversion_probability"`
	ExpectedGMV           int     `json:"expected_gmv"`
	City                  string  `json:"city,omitempty"`
}

type Institution struct {
	Name            string `json:"name"`
	Type            string `json:"type"`
	SubsidyBudget   int    `json:"subsidy_budget"`
	BidAmount       int    `json:"bid_amount"`
	ROIWindow       string `json:"roi_window"`
	DistanceMinutes int    `json:"distance_minutes"`
}

type Doctor struct {
	ID                  string      `json:"id"`
	Name                string      `json:"name"`
	Department          string      `json:"department"`
	Title               string      `json:"title"`
	Specialties         []string    `json:"specialties"`
	Online              bool        `json:"online"`
	CurrentLoad         int         `json:"current_load"`
	ConversionRate      float64     `json:"conversion_rate"`
	QualityScore        float64     `json:"quality_score"`
	ResponseSeconds     int         `json:"response_seconds"`
	ConsultationSubsidy int         `json:"consultation_subsidy"`
	Institution         Institution `json:"institution"`
}

type DoctorMatch struct {
	Doctor              Doctor  `json:"doctor"`
	MatchScore          float64 `json:"match_score"`
	SubsidyText         string  `json:"subsidy_text"`
	Reason              string  `json:"reason"`
	EstimatedGMVRange   string  `json:"estimated_gmv_range"`
	RecommendedNextStep string  `json:"recommended_next_step"`
}
