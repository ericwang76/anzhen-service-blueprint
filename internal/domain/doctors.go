package domain

import (
	"context"
	"fmt"
	"sort"
)

func (e *Engine) MatchDoctor(_ context.Context, input MatchDoctorInput) (DoctorMatch, error) {
	department := input.Department
	if department == "" {
		department = "全科咨询"
	}

	var matches []DoctorMatch
	for _, doctor := range e.doctors {
		if !doctor.Online || doctor.CurrentLoad >= 4 || doctor.Institution.SubsidyBudget < doctor.ConsultationSubsidy {
			continue
		}
		if doctor.Department != department && doctor.Department != "全科咨询" {
			continue
		}

		speedScore := 1 - min(float64(doctor.ResponseSeconds)/180, 0.8)
		bidScore := min(float64(doctor.Institution.BidAmount)/80, 1)
		loadPenalty := float64(doctor.CurrentLoad) * 0.06
		matchScore := input.ConversionProbability*0.32 +
			doctor.ConversionRate*0.26 +
			doctor.QualityScore*0.20 +
			speedScore*0.12 +
			bidScore*0.10 -
			loadPenalty

		matches = append(matches, DoctorMatch{
			Doctor:              doctor,
			MatchScore:          round(matchScore, 2),
			SubsidyText:         fmt.Sprintf("本次图文咨询由%s补贴 ¥%d，用户无需支付。", doctor.Institution.Name, doctor.ConsultationSubsidy),
			Reason:              fmt.Sprintf("匹配%s，响应约%d秒，机构补贴预算充足，质量分%.0f。", doctor.Department, doctor.ResponseSeconds, doctor.QualityScore*100),
			EstimatedGMVRange:   estimateGMVRange(input.ExpectedGMV),
			RecommendedNextStep: "先由医生判断是否需要检查、到店或继续居家观察。",
		})
	}

	if len(matches) == 0 {
		return DoctorMatch{}, nil
	}
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].MatchScore > matches[j].MatchScore
	})
	return matches[0], nil
}

func seedDoctors() []Doctor {
	return []Doctor{
		{
			ID:                  "doc-gastro-01",
			Name:                "周明",
			Department:          "消化科",
			Title:               "副主任医师",
			Specialties:         []string{"反复胃痛", "反酸烧心", "胃镜前咨询"},
			Online:              true,
			CurrentLoad:         1,
			ConversionRate:      0.42,
			QualityScore:        0.91,
			ResponseSeconds:     45,
			ConsultationSubsidy: 50,
			Institution: Institution{
				Name:            "仁和中西医消化中心",
				Type:            "专科门诊",
				SubsidyBudget:   2600,
				BidAmount:       58,
				ROIWindow:       "7天到店核销",
				DistanceMinutes: 18,
			},
		},
		{
			ID:                  "doc-tcm-01",
			Name:                "林晓岚",
			Department:          "中医调理",
			Title:               "主治医师",
			Specialties:         []string{"脾胃调理", "失眠体虚", "亚健康管理"},
			Online:              true,
			CurrentLoad:         2,
			ConversionRate:      0.38,
			QualityScore:        0.88,
			ResponseSeconds:     60,
			ConsultationSubsidy: 45,
			Institution: Institution{
				Name:            "四时堂中医馆",
				Type:            "中医馆",
				SubsidyBudget:   3200,
				BidAmount:       62,
				ROIWindow:       "30天复诊复购",
				DistanceMinutes: 24,
			},
		},
		{
			ID:                  "doc-skin-01",
			Name:                "许清",
			Department:          "皮肤科",
			Title:               "主治医师",
			Specialties:         []string{"湿疹", "痤疮", "皮肤瘙痒"},
			Online:              true,
			CurrentLoad:         0,
			ConversionRate:      0.36,
			QualityScore:        0.93,
			ResponseSeconds:     38,
			ConsultationSubsidy: 40,
			Institution: Institution{
				Name:            "柏悦皮肤专科",
				Type:            "皮肤门诊",
				SubsidyBudget:   1900,
				BidAmount:       46,
				ROIWindow:       "14天复诊/购药",
				DistanceMinutes: 16,
			},
		},
		{
			ID:                  "doc-dental-01",
			Name:                "陈嘉禾",
			Department:          "口腔科",
			Title:               "口腔修复医师",
			Specialties:         []string{"种植牙", "正畸初筛", "牙痛处理"},
			Online:              true,
			CurrentLoad:         1,
			ConversionRate:      0.48,
			QualityScore:        0.9,
			ResponseSeconds:     52,
			ConsultationSubsidy: 60,
			Institution: Institution{
				Name:            "微笑口腔连锁",
				Type:            "口腔诊所",
				SubsidyBudget:   5200,
				BidAmount:       76,
				ROIWindow:       "15天到店方案",
				DistanceMinutes: 21,
			},
		},
		{
			ID:                  "doc-beauty-01",
			Name:                "陆雯",
			Department:          "医美咨询",
			Title:               "皮肤美容医师",
			Specialties:         []string{"祛斑", "痘坑痘印", "光电项目评估"},
			Online:              true,
			CurrentLoad:         1,
			ConversionRate:      0.52,
			QualityScore:        0.86,
			ResponseSeconds:     42,
			ConsultationSubsidy: 80,
			Institution: Institution{
				Name:            "澄光医美中心",
				Type:            "医美机构",
				SubsidyBudget:   8800,
				BidAmount:       92,
				ROIWindow:       "30天成交回传",
				DistanceMinutes: 32,
			},
		},
		{
			ID:                  "doc-general-01",
			Name:                "王砚",
			Department:          "全科咨询",
			Title:               "全科医师",
			Specialties:         []string{"常见症状初筛", "报告解读", "就医建议"},
			Online:              true,
			CurrentLoad:         0,
			ConversionRate:      0.24,
			QualityScore:        0.89,
			ResponseSeconds:     50,
			ConsultationSubsidy: 35,
			Institution: Institution{
				Name:            "百度健康互联网医院",
				Type:            "互联网医院",
				SubsidyBudget:   4000,
				BidAmount:       38,
				ROIWindow:       "线上复诊",
				DistanceMinutes: 0,
			},
		},
	}
}

func estimateGMVRange(gmv int) string {
	switch {
	case gmv >= 10000:
		return "¥3,000-30,000"
	case gmv >= 7000:
		return "¥2,000-20,000"
	case gmv >= 1000:
		return "¥500-2,000"
	case gmv >= 600:
		return "¥300-1,200"
	default:
		return "¥100-500"
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
