package logx

import (
	"io"
	"log/slog"
	"regexp"
	"strings"
)

var deniedKeys = map[string]bool{
	"phone": true, "phone_e164": true, "otp": true, "code": true, "token": true,
	"access_token": true, "refresh_token": true, "authorization": true, "password": true,
	"secret": true, "body": true, "text": true, "raw": true, "lat": true, "lng": true,
	"latitude": true, "longitude": true, "email": true, "ticket": true, "cookie": true,
}

var (
	phoneRe = regexp.MustCompile(`(?:\+?91[\s-]?)?[6-9]\d{4}[\s-]?\d{5}`)
	emailRe = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
)

const redacted = "[redacted]"

func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level, ReplaceAttr: scrub}))
}

func scrub(_ []string, a slog.Attr) slog.Attr {
	if deniedKeys[strings.ToLower(a.Key)] {
		return slog.String(a.Key, redacted)
	}
	if a.Value.Kind() == slog.KindString {
		s := a.Value.String()
		if phoneRe.MatchString(s) || emailRe.MatchString(s) {
			s = phoneRe.ReplaceAllString(s, redacted)
			s = emailRe.ReplaceAllString(s, redacted)
			return slog.String(a.Key, s)
		}
	}
	return a
}
