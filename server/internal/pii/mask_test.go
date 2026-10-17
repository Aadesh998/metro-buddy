package pii

import (
	"strings"
	"testing"

	"metro-buddy/server/internal/model"
)

// withCheckDigit appends the Verhoeff check digit so test Aadhaar numbers are valid.
func withCheckDigit(base string) string {
	inv := [10]int{0, 4, 3, 2, 1, 5, 6, 7, 8, 9}
	c := 0
	for i := range len(base) {
		d := int(base[len(base)-1-i] - '0')
		c = verhoeffD[c][verhoeffP[(i+1)%8][d]]
	}
	return base + string(rune('0'+inv[c]))
}

type sample struct {
	in   string
	kind model.PIIKind
	leak string
}

func corpus() []sample {
	aad := withCheckDigit("23456789012")
	aadSpaced := aad[:4] + " " + aad[4:8] + " " + aad[8:]
	return []sample{
		{"call me 9876543210", model.PIIPhone, "9876543210"},
		{"my no is 98765 43210", model.PIIPhone, "43210"},
		{"98765-43210 pe whatsapp karo", model.PIIPhone, "43210"},
		{"+91 98765 43210", model.PIIPhone, "43210"},
		{"+91-9876543210", model.PIIPhone, "9876543210"},
		{"091 98765 43210", model.PIIPhone, "43210"},
		{"09876543210 call", model.PIIPhone, "9876543210"},
		{"9 8 7 6 5 4 3 2 1 0", model.PIIPhone, "5 4 3 2 1"},
		{"98.76.54.32.10", model.PIIPhone, "54.32"},
		{"(987) 654-3210", model.PIIPhone, "654-3210"},
		{"num: 7012345678.", model.PIIPhone, "7012345678"},
		{"text 6012345678 after 6pm", model.PIIPhone, "6012345678"},
		{"9876 543 210 hai mera", model.PIIPhone, "543 210"},
		{"919876543210", model.PIIPhone, "9876543210"},
		{"98765/43210", model.PIIPhone, "43210"},
		{"riya.sharma@gmail.com", model.PIIEmail, "riya.sharma"},
		{"mail me at Riya_23@Yahoo.co.in pls", model.PIIEmail, "Riya_23"},
		{"riya at gmail dot com", model.PIIEmail, "riya at gmail"},
		{"riya(at)gmail(dot)com", model.PIIEmail, "riya(at)"},
		{"riya [at] outlook [dot] com", model.PIIEmail, "riya [at]"},
		{"work: a.b+metro@company.org", model.PIIEmail, "a.b+metro"},
		{"pay me on riya@okicici", model.PIIUPI, "riya@okicici"},
		{"upi 9876543210@ybl", model.PIIUPI, "@ybl"},
		{"send to rahul.k@paytm", model.PIIUPI, "rahul.k"},
		{"gpay amit-23@okaxis", model.PIIUPI, "amit-23"},
		{"aadhaar " + aad, model.PIIAadhaar, aad},
		{"my aadhar is " + aadSpaced, model.PIIAadhaar, aadSpaced[5:9]},
		{"PAN ABCPE1234F", model.PIIPAN, "ABCPE1234F"},
		{"pan no abcpe1234f", model.PIIPAN, "abcpe1234f"},
		{"PAN: AAACT 1234 Q", model.PIIPAN, "1234"},
		{"follow @riya_23", model.PIIHandle, "riya_23"},
		{"insta: riya.sharma", model.PIIHandle, "riya.sharma"},
		{"my insta is riya_23", model.PIIHandle, "riya_23"},
		{"ig - metro.girl", model.PIIHandle, "metro.girl"},
		{"snap me: coolguy99", model.PIIHandle, "coolguy99"},
		{"snapchat id rahul_k", model.PIIHandle, "rahul_k"},
		{"telegram @delhi_rider", model.PIIHandle, "delhi_rider"},
		{"tg: delhirider", model.PIIHandle, "delhirider"},
		{"instagram pe riyaaa_x hai", model.PIIHandle, "riyaaa_x"},
		{"wa.me/919876543210", model.PIIHandle, "919876543210"},
		{"https://t.me/delhirider", model.PIIHandle, "delhirider"},
		{"instagram.com/riya_23", model.PIIHandle, "riya_23"},
		{"discord.gg/abcd", model.PIIHandle, "abcd"},
		{"check https://my-site.com/x", model.PIIURL, "my-site"},
		{"www.example.net is good", model.PIIURL, "example"},
		{"visit coolblog.in/post/1", model.PIIURL, "coolblog"},
		{"bit.ly/3abcde", model.PIIURL, "3abcde"},
		{"http://192.168.1.10:8080/x", model.PIIURL, "192.168"},
	}
}

func TestMaskerRecallOnCorpus(t *testing.T) {
	cs := corpus()
	caught := 0
	for _, c := range cs {
		r := Mask(c.in, Options{})
		if !strings.Contains(r.Text, c.leak) && r.Flags&Flag(c.kind) != 0 {
			caught++
			continue
		}
		t.Logf("missed %s: %q -> %q (flags %b)", c.kind, c.in, r.Text, r.Flags)
	}
	recall := float64(caught) / float64(len(cs))
	if recall < 0.95 {
		t.Fatalf("recall %.2f < 0.95 (%d/%d)", recall, caught, len(cs))
	}
	t.Logf("recall %.3f (%d/%d)", recall, caught, len(cs))
}

func TestMaskerLeavesOrdinaryTextAlone(t *testing.T) {
	clean := []string{
		"Train was 15 minutes late at Rajiv Chowk again",
		"Yellow line coach 3 is always packed between 9 and 10",
		"I paid 250 rupees for the smart card in 2024",
		"Meeting at 6:30, then 7:45 train",
		"fb is down again lol",
		"insta reels are a waste of time",
		"Score was 3-2 in the match",
		"email me later? no, just post here",
		"Platform 2, gate number 4",
		"Read more on delhimetrorail.com/travel-info",
		"The 12345 train never comes",
		"we had 100 people at the event and 2500 online",
		"Is Huda City Centre at sector 29?",
	}
	for _, s := range clean {
		if r := Mask(s, Options{}); r.Text != s || r.Flags != 0 {
			t.Errorf("false positive: %q -> %q", s, r.Text)
		}
	}
}

func TestMaskerAllowListsContactAfterReveal(t *testing.T) {
	in := "call 9876543210 or insta: riya_23, email a@b.com"
	r := Mask(in, Options{Allow: map[model.PIIKind]bool{model.PIIPhone: true, model.PIIHandle: true}})
	if !strings.Contains(r.Text, "9876543210") || !strings.Contains(r.Text, "riya_23") {
		t.Fatalf("revealed kinds still masked: %q", r.Text)
	}
	if strings.Contains(r.Text, "a@b.com") {
		t.Fatalf("email should stay masked: %q", r.Text)
	}
}

func TestMaskerReportsKindsOnce(t *testing.T) {
	r := Mask("9876543210 and 9123456789 and a@b.com", Options{})
	if len(r.Kinds) != 2 || r.Flags != FlagPhone|FlagEmail {
		t.Fatalf("kinds=%v flags=%b", r.Kinds, r.Flags)
	}
}

func TestVerhoeff(t *testing.T) {
	v := withCheckDigit("23456789012")
	if !Verhoeff(v) {
		t.Fatal("valid number rejected")
	}
	bad := v[:11] + string(rune('0'+(int(v[11]-'0')+1)%10))
	if Verhoeff(bad) {
		t.Fatal("invalid number accepted")
	}
}
