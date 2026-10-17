package attest

import (
	"testing"
	"time"
)

func TestPlayVerdictNeedsNonceAppAndDevice(t *testing.T) {
	var v integrityVerdict
	p := &v.TokenPayloadExternal
	p.RequestDetails.Nonce, p.RequestDetails.RequestPackageName = "n1", "in.metrobuddy.app"
	p.AppIntegrity.AppRecognitionVerdict = "PLAY_RECOGNIZED"
	p.DeviceIntegrity.DeviceRecognitionVerdict = []string{"MEETS_DEVICE_INTEGRITY"}
	if err := checkVerdict(v, "in.metrobuddy.app", "n1"); err != nil {
		t.Fatal(err)
	}
	if checkVerdict(v, "in.metrobuddy.app", "other") == nil {
		t.Fatal("replayed nonce accepted")
	}
	p.DeviceIntegrity.DeviceRecognitionVerdict = []string{"MEETS_VIRTUAL_INTEGRITY"}
	if checkVerdict(v, "in.metrobuddy.app", "n1") == nil {
		t.Fatal("emulator accepted")
	}
}

func TestAppAttestRejectsGarbage(t *testing.T) {
	if VerifyAppAttest([]byte{0xa0}, []byte("k"), []byte("c"), "TEAM.in.metrobuddy.app", false, time.Now()) == nil {
		t.Fatal("empty attestation accepted")
	}
}
