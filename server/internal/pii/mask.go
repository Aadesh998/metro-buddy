package pii

import (
	"net/url"
	"regexp"
	"strings"

	"metro-buddy/server/internal/model"
)

const (
	FlagPhone int16 = 1 << iota
	FlagEmail
	FlagUPI
	FlagAadhaar
	FlagPAN
	FlagHandle
	FlagURL
	FlagAddress
)

var kindFlag = map[model.PIIKind]int16{
	model.PIIPhone: FlagPhone, model.PIIEmail: FlagEmail, model.PIIUPI: FlagUPI, model.PIIAadhaar: FlagAadhaar,
	model.PIIPAN: FlagPAN, model.PIIHandle: FlagHandle, model.PIIURL: FlagURL, model.PIIAddress: FlagAddress,
}

func Flag(k model.PIIKind) int16 { return kindFlag[k] }

const (
	TokPhone  = "[phone hidden]"
	TokEmail  = "[email hidden]"
	TokUPI    = "[upi hidden]"
	TokID     = "[id hidden]"
	TokHandle = "[handle hidden]"
	TokLink   = "[link hidden]"
)

type Result struct {
	Text  string
	Flags int16
	Kinds []model.PIIKind
}

func (r *Result) add(k model.PIIKind) {
	if r.Flags&kindFlag[k] == 0 {
		r.Kinds = append(r.Kinds, k)
	}
	r.Flags |= kindFlag[k]
}

// Allow lifts masking for specific kinds; used only after a mutual contact-level reveal.
type Options struct {
	Allow map[model.PIIKind]bool
}

var allowedHosts = []string{
	"delhimetrorail.com", "dmrc.org", "bmrc.co.in", "mmrda.maharashtra.gov.in", "ltmetro.com",
	"chennaimetrorail.org", "kmrc.in", "punemetrorail.org", "wikipedia.org", "telemanas.mohfw.gov.in",
	"metrobuddy.in",
}

const socialPlatforms = `insta(?:gram)?|ig|snap(?:chat)?|sc|telegram|tele|tg|twitter|tw|fb|facebook|discord|whatsapp|wa|threads|linkedin|reddit|bumble|hinge|tinder`

var (
	socialURLRe = regexp.MustCompile(`(?i)\b(?:https?://)?(?:www\.)?(?:wa\.me|t\.me|telegram\.me|instagram\.com|instagr\.am|snapchat\.com|facebook\.com|fb\.com|m\.me|twitter\.com|x\.com|discord\.gg|discord\.com|threads\.net|linkedin\.com)/[^\s]*`)
	urlRe       = regexp.MustCompile(`(?i)\b(?:https?://|www\.)[^\s<>"]+|\b[a-z0-9][a-z0-9-]{0,62}(?:\.[a-z0-9-]{1,63})*\.(?:com|in|net|org|io|me|co|ly|gg|app|xyz|info|link|site|online|club|live|tv|us|uk)(?:/[^\s<>"]*)?\b`)
	emailRe     = regexp.MustCompile(`(?i)\b[a-z0-9][a-z0-9._%+\-]*@[a-z0-9.\-]+\.[a-z]{2,}\b`)
	// Spelled-out separators: "riya at gmail dot com", "riya(at)gmail(dot)com".
	emailObfRe  = regexp.MustCompile(`(?i)\b[a-z0-9][a-z0-9._%+\-]*\s*(?:\(at\)|\[at\]|\{at\}|\s+at\s+)\s*(?:gmail|yahoo|outlook|hotmail|icloud|rediffmail|rediff|proton|protonmail|live|ymail)\s*(?:\(dot\)|\[dot\]|\{dot\}|\s+dot\s+|\.)\s*(?:com|in|co\.in|me)\b`)
	upiRe       = regexp.MustCompile(`(?i)\b[a-z0-9][a-z0-9._\-]{1,}@[a-z]{2,}\b`)
	platformRe  = regexp.MustCompile(`(?i)\b(` + socialPlatforms + `)\b(\s*(?:id|handle|username|user|acc|account|pe|par|me)?\s*(?:[:\-=]|is|hai|->|=>)?\s*)@?([a-z0-9_][a-z0-9_.]{1,29})`)
	atHandleRe  = regexp.MustCompile(`(?i)(^|[^a-z0-9_.@\]])@([a-z0-9_][a-z0-9_.]{1,29})`)
	panRe       = regexp.MustCompile(`(?i)\b[a-z]{3}[abcfghljpt][a-z]\s?\d{4}\s?[a-z]\b`)
	digitSpanRe = regexp.MustCompile(`\+?\d(?:[\s\-.()/]{0,3}\d)+`)
	nonDigitRe  = regexp.MustCompile(`\D`)
)

// Words that follow a platform name in normal speech; without this "fb is down" would mask "down".
var platformStopwords = map[string]bool{
	"is": true, "was": true, "and": true, "or": true, "the": true, "a": true, "down": true, "story": true,
	"stories": true, "reels": true, "reel": true, "post": true, "posts": true, "pe": true, "par": true, "hai": true,
	"group": true, "groups": true, "chat": true, "call": true, "me": true, "pr": true, "ka": true, "ki": true,
	"ke": true, "nahi": true, "not": true, "app": true, "account": true, "id": true, "dm": true, "dms": true,
	"to": true, "for": true, "in": true, "on": true, "it": true, "this": true, "that": true, "with": true,
}

func Mask(text string, opt Options) Result {
	r := Result{Text: text}
	allow := func(k model.PIIKind) bool { return opt.Allow[k] }

	if !allow(model.PIIHandle) {
		r.Text = replaceAll(socialURLRe, r.Text, func(string) string { r.add(model.PIIHandle); return TokHandle })
	}
	if !allow(model.PIIEmail) {
		r.Text = replaceAll(emailRe, r.Text, func(string) string { r.add(model.PIIEmail); return TokEmail })
		r.Text = replaceAll(emailObfRe, r.Text, func(string) string { r.add(model.PIIEmail); return TokEmail })
	}
	r.Text = replaceAll(urlRe, r.Text, func(m string) string {
		if strings.Contains(m, "[") || hostAllowed(m) {
			return m
		}
		r.add(model.PIIURL)
		return TokLink
	})
	if !allow(model.PIIUPI) {
		r.Text = replaceAll(upiRe, r.Text, func(string) string { r.add(model.PIIUPI); return TokUPI })
	}
	if !allow(model.PIIHandle) {
		r.Text = platformRe.ReplaceAllStringFunc(r.Text, func(m string) string {
			sub := platformRe.FindStringSubmatch(m)
			handle := strings.ToLower(sub[3])
			if platformStopwords[handle] {
				return m
			}
			r.add(model.PIIHandle)
			return sub[1] + sub[2] + TokHandle
		})
		r.Text = atHandleRe.ReplaceAllStringFunc(r.Text, func(m string) string {
			sub := atHandleRe.FindStringSubmatch(m)
			r.add(model.PIIHandle)
			return sub[1] + TokHandle
		})
	}
	r.Text = replaceAll(panRe, r.Text, func(m string) string {
		if !hasDigit(m) {
			return m
		}
		r.add(model.PIIPAN)
		return TokID
	})
	r.Text = replaceAll(digitSpanRe, r.Text, func(m string) string {
		d := nonDigitRe.ReplaceAllString(m, "")
		switch {
		case len(d) == 12 && d[0] >= '2' && Verhoeff(d):
			r.add(model.PIIAadhaar)
			return TokID
		case isIndianMobile(d):
			if allow(model.PIIPhone) {
				return m
			}
			r.add(model.PIIPhone)
			return TokPhone
		case len(d) > 12:
			// Long digit runs usually hide a phone or account number among them.
			if allow(model.PIIPhone) {
				return m
			}
			r.add(model.PIIPhone)
			return TokPhone
		}
		return m
	})
	return r
}

func isIndianMobile(d string) bool {
	switch {
	case len(d) == 10:
		return d[0] >= '6'
	case len(d) == 11 && d[0] == '0':
		return d[1] >= '6'
	case len(d) == 12 && strings.HasPrefix(d, "91"):
		return d[2] >= '6'
	case len(d) == 13 && strings.HasPrefix(d, "091"):
		return d[3] >= '6'
	}
	return false
}

func hostAllowed(m string) bool {
	raw := m
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range allowedHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

func hasDigit(s string) bool { return strings.ContainsAny(s, "0123456789") }

func replaceAll(re *regexp.Regexp, s string, f func(string) string) string {
	return re.ReplaceAllStringFunc(s, f)
}

var (
	verhoeffD = [10][10]int{
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, {1, 2, 3, 4, 0, 6, 7, 8, 9, 5}, {2, 3, 4, 0, 1, 7, 8, 9, 5, 6},
		{3, 4, 0, 1, 2, 8, 9, 5, 6, 7}, {4, 0, 1, 2, 3, 9, 5, 6, 7, 8}, {5, 9, 8, 7, 6, 0, 4, 3, 2, 1},
		{6, 5, 9, 8, 7, 1, 0, 4, 3, 2}, {7, 6, 5, 9, 8, 2, 1, 0, 4, 3}, {8, 7, 6, 5, 9, 3, 2, 1, 0, 4},
		{9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
	}
	verhoeffP = [8][10]int{
		{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}, {1, 5, 7, 6, 2, 8, 3, 0, 9, 4}, {5, 8, 0, 3, 7, 9, 6, 1, 4, 2},
		{8, 9, 1, 6, 0, 4, 3, 5, 2, 7}, {9, 4, 5, 3, 1, 2, 6, 8, 7, 0}, {4, 2, 8, 6, 5, 7, 3, 9, 0, 1},
		{2, 7, 9, 3, 8, 0, 6, 4, 1, 5}, {7, 0, 4, 6, 9, 1, 3, 2, 5, 8},
	}
)

func Verhoeff(digits string) bool {
	c := 0
	for i := range len(digits) {
		d := int(digits[len(digits)-1-i] - '0')
		if d < 0 || d > 9 {
			return false
		}
		c = verhoeffD[c][verhoeffP[i%8][d]]
	}
	return c == 0
}
