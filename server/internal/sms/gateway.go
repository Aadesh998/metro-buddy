package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

// Gateway sends through SMS Gateway for Android (open source, github.com/capcom6/android-sms-gateway):
// an app on an Android phone that sends ordinary SMS from its own SIM. Use the public cloud relay
// (https://api.sms-gate.app/3rdparty/v1/messages) or a self-hosted private server.
type Gateway struct {
	URL, User, Password string
}

func (g Gateway) send(ctx context.Context, to, text string) error {
	if g.URL == "" || g.User == "" {
		return ErrNotConfigured
	}
	body, _ := json.Marshal(map[string]any{"textMessage": map[string]string{"text": text}, "phoneNumbers": []string{to}})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, g.URL, bytes.NewReader(body))
	req.SetBasicAuth(g.User, g.Password)
	req.Header.Set("Content-Type", "application/json")
	return do(req)
}

func (g Gateway) SendOTP(ctx context.Context, e164, code string) error {
	return g.send(ctx, e164, code+" is your Metro Buddy code. It expires in 5 minutes. Never share it.")
}

func (g Gateway) SendSOS(ctx context.Context, e164, alias, link string) error {
	return g.send(ctx, e164, "SOS from "+alias+" on Metro Buddy. Live trip: "+link+" If in danger call 112.")
}
