package domain

import (
	"context"
	"testing"
)

func TestMVPServiceEndToEnd(t *testing.T) {
	svc := NewMVPService()
	c, err := svc.Create("空腹血糖 6.3 正常吗", "百科说可能是空腹血糖受损，需要担心吗？")
	if err != nil {
		t.Fatal(err)
	}
	for _, answer := range []string{"晨起空腹 10 小时", "以前没有", "母亲有糖尿病"} {
		c, err = svc.PatientMessage(c.ID, answer)
		if err != nil {
			t.Fatal(err)
		}
	}
	if c.Status != CaseCollecting || len(c.Answers) != 3 {
		t.Fatalf("want three collected answers, got status=%s answers=%d", c.Status, len(c.Answers))
	}
	if c, err = svc.SubmitCheck(c.ID); err != nil || c.Status != CaseChecking {
		t.Fatalf("submit check: status=%s err=%v", c.Status, err)
	}
	drafts, err := svc.Drafts(context.Background(), c.ID)
	if err != nil || len(drafts) != 2 {
		t.Fatalf("drafts=%d err=%v", len(drafts), err)
	}
	if c, err = svc.SendCheck(c.ID, drafts[0].Content, "direct"); err != nil || c.Status != CaseVerified {
		t.Fatalf("send check: status=%s err=%v", c.Status, err)
	}
	if c, err = svc.Pay(c.ID); err != nil || c.Status != CasePreConsult {
		t.Fatalf("pay: status=%s err=%v", c.Status, err)
	}
	for _, answer := range []string{"想知道饮食怎么调整", "没有明显症状", "目前未用药"} {
		c, err = svc.PatientMessage(c.ID, answer)
		if err != nil {
			t.Fatal(err)
		}
	}
	if c.Status != CaseWaiting {
		t.Fatalf("want waiting status, got %s", c.Status)
	}
	if c, err = svc.Accept(c.ID); err != nil || c.Status != CaseLive {
		t.Fatalf("accept: status=%s err=%v", c.Status, err)
	}
	if _, err = svc.CopilotReply(context.Background(), c.ID); err != nil {
		t.Fatal(err)
	}
	if c, err = svc.DoctorMessage(c.ID, "建议水果放在两餐之间。", "direct"); err != nil {
		t.Fatal(err)
	}
	if c, err = svc.End(c.ID); err != nil || c.Status != CaseCompleted {
		t.Fatalf("end: status=%s err=%v", c.Status, err)
	}
	if len(c.Trace) < 15 {
		t.Fatalf("expected trace to be retained, got %d events", len(c.Trace))
	}
}

func TestMVPServiceUrgentSignalStopsNormalFlow(t *testing.T) {
	svc := NewMVPService()
	c, err := svc.Create("突然胸痛还呼吸困难", "想请医生核验一下")
	if err != nil {
		t.Fatal(err)
	}
	if c.Status != CaseUrgent {
		t.Fatalf("want urgent status, got %s", c.Status)
	}
}

func TestPersistentMVPServiceReloadsCases(t *testing.T) {
	dir := t.TempDir()
	first, err := NewPersistentMVPService(dir)
	if err != nil {
		t.Fatal(err)
	}
	created, err := first.CreateForOwner("patient_test", "血糖咨询", "请核验这段百科内容")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewPersistentMVPService(dir)
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := second.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.OwnerID != "patient_test" || loaded.Query != created.Query || len(loaded.Messages) != 2 {
		t.Fatalf("case was not persisted correctly: %#v", loaded)
	}
}
