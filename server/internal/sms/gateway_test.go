package sms

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGatewaySendsTextToOnePhone(t *testing.T) {
	var got struct {
		TextMessage  struct{ Text string } `json:"textMessage"`
		PhoneNumbers []string              `json:"phoneNumbers"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, _ := r.BasicAuth(); u != "u" || p != "p" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	g := Gateway{URL: srv.URL, User: "u", Password: "p"}
	if err := g.SendOTP(context.Background(), "+919876543210", "123456"); err != nil {
		t.Fatal(err)
	}
	if got.PhoneNumbers[0] != "+919876543210" || got.TextMessage.Text[:6] != "123456" {
		t.Fatalf("%+v", got)
	}
	if (Gateway{URL: srv.URL, User: "u", Password: "x"}).SendOTP(context.Background(), "+91", "1") == nil {
		t.Fatal("bad credentials accepted")
	}
	if (Gateway{}).SendOTP(context.Background(), "+91", "1") != ErrNotConfigured {
		t.Fatal("unconfigured gateway should say so")
	}
}
