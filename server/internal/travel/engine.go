package travel

import (
	"math"
	"time"

	"github.com/google/uuid"
)

// Match engine (F42). Pure functions so the scoring is unit-testable without a database.
const (
	MinOverlap  = 0.4
	MaxDelta    = 20 * time.Minute
	wOverlap    = 0.45
	wTime       = 0.20
	wVibe       = 0.30
	wTrust      = 0.05
	TopN        = 10
	IntentTTL   = 45 * time.Minute
	MaxLeadTime = 45 * time.Minute
)

// Overlap finds the longest run of consecutive stations both trips ride in the same
// direction. Sharing stations in opposite directions isn't sharing a train.
func Overlap(a, b []uuid.UUID) []uuid.UUID {
	best, end := 0, 0
	prev := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
				if cur[j] > best {
					best, end = cur[j], i
				}
			}
		}
		prev = cur
	}
	if best == 0 {
		return nil
	}
	return a[end-best : end]
}

func OverlapRatio(a, b, seg []uuid.UUID) float64 {
	shorter := min(len(a), len(b))
	if shorter == 0 || len(seg) < 2 {
		return 0
	}
	return float64(len(seg)) / float64(shorter)
}

func TimeFit(a, b time.Time) float64 {
	d := math.Abs(a.Sub(b).Minutes())
	return math.Max(0, 1-d/MaxDelta.Minutes())
}

func Jaccard(a, b []string) (float64, []string) {
	if len(a) == 0 && len(b) == 0 {
		return 0, nil
	}
	set := map[string]bool{}
	for _, x := range a {
		set[x] = true
	}
	var shared []string
	union := len(set)
	seen := map[string]bool{}
	for _, x := range b {
		if seen[x] {
			continue
		}
		seen[x] = true
		if set[x] {
			shared = append(shared, x)
		} else {
			union++
		}
	}
	return float64(len(shared)) / float64(union), shared
}

func Score(overlap, timeFit, vibe float64, trustScore int) float64 {
	tb := math.Max(0, math.Min(1, float64(trustScore)/100))
	return math.Round((wOverlap*overlap+wTime*timeFit+wVibe*vibe+wTrust*tb)*1000) / 1000
}

// Person is what preference filters need about one side. Gender and age are used only to
// filter; they are never shown to the other person.
type Person struct {
	Gender         string
	Age            int
	Verified       bool
	SameGender     bool
	VerifiedOnly   bool
	AgeMin, AgeMax int
}

// Accepts reports whether p's preferences allow being matched with other.
func (p Person) Accepts(other Person) bool {
	if p.SameGender && (p.Gender == "" || p.Gender == "prefer_not_to_say" || p.Gender != other.Gender) {
		return false
	}
	if p.VerifiedOnly && !other.Verified {
		return false
	}
	if p.AgeMin > 0 && other.Age < p.AgeMin {
		return false
	}
	if p.AgeMax > 0 && other.Age > p.AgeMax {
		return false
	}
	return true
}

// Compatible applies both people's preferences: a match must be acceptable to each side.
func Compatible(a, b Person) bool { return a.Accepts(b) && b.Accepts(a) }
