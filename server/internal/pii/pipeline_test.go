package pii

import (
	"bytes"
	"io"
	"log/slog"
	"testing"
	"time"

	"metro-buddy/server/internal/audit"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/secure"
	"metro-buddy/server/internal/testutil"
)

func newPipeline(t *testing.T) *Pipeline {
	gdb := testutil.DB(t)
	box, _ := secure.NewBox([]byte("0123456789abcdef0123456789abcdef"))
	return &Pipeline{DB: gdb, Box: box, Audit: audit.New(gdb, slog.New(slog.NewTextHandler(io.Discard, nil)), []byte("p"))}
}

func TestProcessVaultsEncryptedRawAndRecordsDetections(t *testing.T) {
	p := newPipeline(t)
	id := model.NewID()
	r, err := p.Process(p.DB, model.CTPost, id, "call 9876543210", Options{})
	if err != nil || r.Text != "call [phone hidden]" {
		t.Fatalf("%q %v", r.Text, err)
	}
	var v model.PIIVault
	p.DB.First(&v, "content_id = ?", id)
	if bytes.Contains(v.RawEnc, []byte("9876543210")) {
		t.Fatal("vault stores plaintext")
	}
	var det []model.PIIDetection
	p.DB.Find(&det, "content_id = ?", id)
	if len(det) != 1 || det[0].Kind != model.PIIPhone || det[0].Source != model.DetectorRegex {
		t.Fatalf("detections %+v", det)
	}

	clean := model.NewID()
	p.Process(p.DB, model.CTPost, clean, "just a rant", Options{})
	var n int64
	p.DB.Model(&model.PIIDetection{}).Where("content_id = ?", clean).Count(&n)
	if n != 0 {
		t.Fatal("clean text produced detections")
	}
}

func TestPurgeDeletesExpiredRows(t *testing.T) {
	p := newPipeline(t)
	now := time.Now()
	p.Now = func() time.Time { return now }
	old, fresh := model.NewID(), model.NewID()
	p.Process(p.DB, model.CTMessage, old, "a@b.com", Options{})
	now = now.Add(VaultTTL - time.Hour)
	p.Process(p.DB, model.CTMessage, fresh, "c@d.com", Options{})
	now = now.Add(2 * time.Hour)
	if err := p.Purge(t.Context()); err != nil {
		t.Fatal(err)
	}
	var ids []string
	p.DB.Model(&model.PIIVault{}).Pluck("content_id", &ids)
	if len(ids) != 1 || ids[0] != fresh.String() {
		t.Fatalf("remaining %v", ids)
	}
}

func TestReadVaultIsAudited(t *testing.T) {
	p := newPipeline(t)
	id, admin := model.NewID(), model.NewID()
	p.Process(p.DB, model.CTComment, id, "upi me riya@okicici", Options{})
	got, err := p.ReadVault(t.Context(), admin, "1.2.3.4", model.CTComment, id, "police request #42")
	if err != nil || len(got) != 1 || got[0].Raw != "upi me riya@okicici" {
		t.Fatalf("%+v %v", got, err)
	}
	var logs []model.AuditLog
	p.DB.Find(&logs, "verb = ?", "pii_vault.read")
	if len(logs) != 1 || *logs[0].ActorID != admin {
		t.Fatalf("audit %+v", logs)
	}
	var sec int64
	p.DB.Model(&model.SecurityEvent{}).Where("kind = ?", model.SecVaultAccess).Count(&sec)
	if sec != 1 {
		t.Fatal("no vault_access security event")
	}
}
