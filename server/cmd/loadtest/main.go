// loadtest drives a running dev server (APP_ENV=development, OTP_DEV_CODE set) with a realistic
// mix of reads and writes and prints latency percentiles per endpoint:
//
//	go run ./cmd/loadtest -base http://localhost:8080 -users 100 -duration 60s
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"
)

var (
	base     = flag.String("base", "http://localhost:8080", "server base URL")
	users    = flag.Int("users", 50, "virtual users")
	duration = flag.Duration("duration", 60*time.Second, "test length")
	otp      = flag.String("otp", "123456", "dev OTP code")
	client   = &http.Client{Timeout: 15 * time.Second}
)

type stat struct {
	mu        sync.Mutex
	latencies map[string][]time.Duration
	codes     map[string]map[int]int
}

var stats = stat{latencies: map[string][]time.Duration{}, codes: map[string]map[int]int{}}

func (s *stat) add(name string, d time.Duration, code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latencies[name] = append(s.latencies[name], d)
	if s.codes[name] == nil {
		s.codes[name] = map[int]int{}
	}
	s.codes[name][code]++
}

func call(name, method, path, token, ip string, body any, out any) int {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, *base+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if ip != "" {
		req.Header.Set("X-Forwarded-For", ip)
	}
	start := time.Now()
	resp, err := client.Do(req)
	code := 0
	if err == nil {
		code = resp.StatusCode
		if out != nil {
			json.NewDecoder(resp.Body).Decode(out)
		} else {
			io.Copy(io.Discard, resp.Body)
		}
		resp.Body.Close()
	}
	if name != "" {
		stats.add(name, time.Since(start), code)
	}
	return code
}

func rnd(n int) int {
	v, _ := rand.Int(rand.Reader, big.NewInt(int64(n)))
	return int(v.Int64())
}

func uuidv7() string {
	var b [16]byte
	rand.Read(b[:])
	ms := uint64(time.Now().UnixMilli())
	var t [8]byte
	binary.BigEndian.PutUint64(t[:], ms)
	copy(b[0:6], t[2:8])
	b[6] = b[6]&0x0f | 0x70
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

type vu struct {
	token, ip, conv, bowl, post string
}

func setup(i int) (*vu, error) {
	ip := fmt.Sprintf("10.%d.%d.%d", 100+i/65025, (i/255)%255, i%255+1)
	phone := fmt.Sprintf("9%09d", rnd(1e9))
	if c := call("", "POST", "/v1/auth/otp/request", "", ip, map[string]string{"phone": phone}, nil); c != 202 {
		return nil, fmt.Errorf("otp request %d", c)
	}
	var v struct{ Session struct{ AccessToken string `json:"access_token"` } }
	call("", "POST", "/v1/auth/otp/verify", "", ip, map[string]any{"phone": phone, "code": *otp, "device": map[string]string{"platform": "android"}}, &v)
	tok := v.Session.AccessToken
	call("", "PUT", "/v1/me/age", tok, ip, map[string]string{"birth_date": "1995-05-05"}, nil)
	call("", "POST", "/v1/me/consents", tok, ip, map[string]any{"policy_version": "2026-10", "items": []map[string]any{{"purpose": "core_service", "granted": true}}}, nil)
	var cities struct{ Items []struct{ ID string } }
	call("", "GET", "/v1/cities", "", ip, nil, &cities)
	var sug struct {
		Suggestions []struct {
			Alias      string `json:"alias"`
			AvatarSeed string `json:"avatar_seed"`
		}
	}
	call("", "POST", "/v1/me/persona/suggestions", tok, ip, map[string]any{"city_id": cities.Items[0].ID}, &sug)
	if len(sug.Suggestions) == 0 {
		return nil, fmt.Errorf("no suggestions")
	}
	call("", "POST", "/v1/me/persona", tok, ip, map[string]any{"city_id": cities.Items[0].ID, "alias": sug.Suggestions[0].Alias, "avatar_seed": sug.Suggestions[0].AvatarSeed}, nil)
	var bowls struct{ Items []struct{ ID string } }
	call("", "GET", "/v1/bowls?kind=topic", tok, ip, nil, &bowls)
	b := bowls.Items[0].ID
	call("", "POST", "/v1/bowls/"+b+"/join", tok, ip, nil, nil)
	var group struct {
		ID         string
		InviteCode string `json:"invite_code"`
	}
	call("", "POST", "/v1/groups", tok, ip, map[string]string{"title": "load test group"}, &group)
	var p struct{ ID string }
	call("", "POST", "/v1/posts", tok, ip, map[string]any{"bowl_id": b, "type": "text", "title": "load test post", "body": "hello"}, &p)
	return &vu{token: tok, ip: ip, conv: group.ID, bowl: b, post: p.ID}, nil
}

func (v *vu) step() {
	switch r := rnd(100); {
	case r < 35:
		call("GET /v1/feed", "GET", "/v1/feed?sort=hot", v.token, v.ip, nil, nil)
	case r < 55:
		call("GET /v1/bowls/:id/posts", "GET", "/v1/bowls/"+v.bowl+"/posts?sort=new", v.token, v.ip, nil, nil)
	case r < 65:
		call("GET /v1/posts/:id/comments", "GET", "/v1/posts/"+v.post+"/comments", v.token, v.ip, nil, nil)
	case r < 72:
		call("POST /v1/posts/:id/comments", "POST", "/v1/posts/"+v.post+"/comments", v.token, v.ip, map[string]string{"body": "same here, call 98765 43210"}, nil)
	case r < 80:
		call("POST /v1/votes", "POST", "/v1/votes", v.token, v.ip, map[string]any{"target_type": "post", "target_id": v.post, "value": 1}, nil)
	case r < 92:
		call("POST /v1/conversations/:id/messages", "POST", "/v1/conversations/"+v.conv+"/messages", v.token, v.ip, map[string]string{"id": uuidv7(), "body": "on my way"}, nil)
	default:
		call("GET /v1/conversations", "GET", "/v1/conversations", v.token, v.ip, nil, nil)
	}
}

func pct(ds []time.Duration, p float64) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	return ds[int(float64(len(ds)-1)*p)]
}

func main() {
	flag.Parse()
	fmt.Printf("setting up %d users...\n", *users)
	vus := make([]*vu, 0, *users)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10)
	for i := range *users {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			v, err := setup(i)
			if err != nil {
				fmt.Fprintln(os.Stderr, "setup:", err)
				return
			}
			mu.Lock()
			vus = append(vus, v)
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	fmt.Printf("running %d virtual users for %s...\n", len(vus), *duration)
	stop := time.Now().Add(*duration)
	start := time.Now()
	for _, v := range vus {
		wg.Add(1)
		go func(v *vu) {
			defer wg.Done()
			for time.Now().Before(stop) {
				v.step()
				time.Sleep(time.Duration(200+rnd(600)) * time.Millisecond)
			}
		}(v)
	}
	wg.Wait()
	elapsed := time.Since(start)

	names := make([]string, 0, len(stats.latencies))
	total, fails, limited := 0, 0, 0
	for n := range stats.latencies {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Printf("\n%-38s %7s %8s %8s %8s %6s %5s\n", "endpoint", "reqs", "p50", "p95", "p99", "429", "err")
	for _, n := range names {
		ds := stats.latencies[n]
		sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
		errs := 0
		for code, c := range stats.codes[n] {
			if code == 0 || code >= 500 {
				errs += c
			}
		}
		total += len(ds)
		fails += errs
		limited += stats.codes[n][429]
		fmt.Printf("%-38s %7d %8s %8s %8s %6d %5d\n", n, len(ds), pct(ds, .5).Round(100*time.Microsecond), pct(ds, .95).Round(100*time.Microsecond), pct(ds, .99).Round(100*time.Microsecond), stats.codes[n][429], errs)
	}
	fmt.Printf("\ntotal %d requests in %s (%.0f req/s), %d rate-limited, %d errors (%.2f%%)\n",
		total, elapsed.Round(time.Second), float64(total)/elapsed.Seconds(), limited, fails, 100*float64(fails)/float64(max(total, 1)))
	if fails > 0 {
		os.Exit(1)
	}
}
