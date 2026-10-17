package sms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Sender interface {
	SendOTP(ctx context.Context, e164, code string) error
	SendSOS(ctx context.Context, e164, alias, link string) error
}

type Dev struct{ Log *slog.Logger }

func (d Dev) SendOTP(context.Context, string, string) error {
	d.Log.Info("sms otp (dev, not sent)")
	return nil
}

func (d Dev) SendSOS(context.Context, string, string, string) error {
	d.Log.Info("sms sos (dev, not sent)")
	return nil
}

var client = &http.Client{Timeout: 10 * time.Second}

type MSG91 struct {
	AuthKey       string
	OTPTemplateID string
	SOSFlowID     string
}

func digitsOnly(e164 string) string { return strings.TrimPrefix(e164, "+") }

// ErrNotConfigured means the provider has no credentials; callers surface it instead of
// pretending a message went out.
var ErrNotConfigured = errors.New("sms provider not configured")

func (m MSG91) SendOTP(ctx context.Context, e164, code string) error {
	if m.AuthKey == "" || m.OTPTemplateID == "" {
		return ErrNotConfigured
	}
	q := url.Values{"template_id": {m.OTPTemplateID}, "mobile": {digitsOnly(e164)}, "otp": {code}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://control.msg91.com/api/v5/otp?"+q.Encode(), nil)
	req.Header.Set("authkey", m.AuthKey)
	return doMSG91(req)
}

func (m MSG91) SendSOS(ctx context.Context, e164, alias, link string) error {
	if m.AuthKey == "" || m.SOSFlowID == "" {
		return ErrNotConfigured
	}
	body, _ := json.Marshal(map[string]any{
		"template_id": m.SOSFlowID,
		"recipients":  []map[string]string{{"mobiles": digitsOnly(e164), "alias": alias, "link": link}},
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "https://control.msg91.com/api/v5/flow/", bytes.NewReader(body))
	req.Header.Set("authkey", m.AuthKey)
	req.Header.Set("Content-Type", "application/json")
	return doMSG91(req)
}

// MSG91 reports most failures (bad key, template, DLT) as HTTP 200 with {"type":"error"}.
func doMSG91(req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sms provider unreachable")
	}
	defer resp.Body.Close()
	var body struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&body)
	if resp.StatusCode >= 300 || body.Type == "error" {
		return fmt.Errorf("sms provider rejected the message (%d)", resp.StatusCode)
	}
	return nil
}

func do(req *http.Request) error {
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sms provider unreachable")
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("sms provider returned %d", resp.StatusCode)
	}
	return nil
}

type Twilio struct {
	AccountSID string
	AuthToken  string
	From       string
}

func (t Twilio) send(ctx context.Context, to, text string) error {
	form := url.Values{"To": {to}, "From": {t.From}, "Body": {text}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://api.twilio.com/2010-04-01/Accounts/"+t.AccountSID+"/Messages.json", strings.NewReader(form.Encode()))
	req.SetBasicAuth(t.AccountSID, t.AuthToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return do(req)
}

func (t Twilio) SendOTP(ctx context.Context, e164, code string) error {
	return t.send(ctx, e164, code+" is your Metro Buddy code. It expires in 5 minutes. Never share it.")
}

func (t Twilio) SendSOS(ctx context.Context, e164, alias, link string) error {
	return t.send(ctx, e164, "SOS from "+alias+" on Metro Buddy. Live trip: "+link+" If in danger call 112.")
}

type Fallback struct {
	Primary, Secondary Sender
	Log                *slog.Logger
}

func (f Fallback) SendOTP(ctx context.Context, e164, code string) error {
	if err := f.Primary.SendOTP(ctx, e164, code); err != nil {
		f.Log.Warn("primary sms failed, falling back", "err", err.Error())
		return f.Secondary.SendOTP(ctx, e164, code)
	}
	return nil
}

func (f Fallback) SendSOS(ctx context.Context, e164, alias, link string) error {
	if err := f.Primary.SendSOS(ctx, e164, alias, link); err != nil {
		f.Log.Warn("primary sms failed, falling back", "err", err.Error())
		return f.Secondary.SendSOS(ctx, e164, alias, link)
	}
	return nil
}
