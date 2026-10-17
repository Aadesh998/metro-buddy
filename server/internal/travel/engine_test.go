package travel

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.New()
	}
	return out
}

func TestOverlapIsSameDirectionContiguous(t *testing.T) {
	s := ids(10)
	a := s[0:7]
	b := s[3:10]
	seg := Overlap(a, b)
	if len(seg) != 4 || seg[0] != s[3] || seg[3] != s[6] {
		t.Fatalf("segment %v", seg)
	}
	if r := OverlapRatio(a, b, seg); r < 0.57 || r > 0.58 {
		t.Fatalf("ratio %v", r)
	}
	rev := make([]uuid.UUID, len(b))
	for i := range b {
		rev[i] = b[len(b)-1-i]
	}
	if seg := Overlap(a, rev); len(seg) > 1 {
		t.Fatalf("opposite directions overlapped: %d", len(seg))
	}
}

func TestTimeFitAndJaccard(t *testing.T) {
	now := time.Now()
	if TimeFit(now, now.Add(5*time.Minute)) != 0.75 || TimeFit(now, now.Add(25*time.Minute)) != 0 {
		t.Fatal("time fit")
	}
	j, shared := Jaccard([]string{"chai", "cricket", "books"}, []string{"chai", "books", "gym", "chai"})
	if j != 0.5 || len(shared) != 2 {
		t.Fatalf("%v %v", j, shared)
	}
}

func TestScoreWeights(t *testing.T) {
	if got := Score(1, 1, 1, 100); got != 1 {
		t.Fatalf("max %v", got)
	}
	if got := Score(0.5, 0.75, 0.5, 50); got != 0.55 {
		t.Fatalf("got %v want 0.55 (0.225+0.15+0.15+0.025)", got)
	}
}

func TestPreferencesFilterBothWays(t *testing.T) {
	woman := Person{Gender: "woman", Age: 26, SameGender: true}
	man := Person{Gender: "man", Age: 30}
	if Compatible(woman, man) || Compatible(man, woman) {
		t.Fatal("same-gender preference ignored")
	}
	if !Compatible(woman, Person{Gender: "woman", Age: 40}) {
		t.Fatal("two women should match")
	}
	if Compatible(Person{VerifiedOnly: true}, Person{}) {
		t.Fatal("verified-only ignored")
	}
	if Compatible(Person{AgeMin: 25, AgeMax: 30}, Person{Age: 35}) || !Compatible(Person{AgeMin: 25, AgeMax: 30}, Person{Age: 28}) {
		t.Fatal("age band")
	}
	if Compatible(Person{SameGender: true, Gender: "prefer_not_to_say"}, Person{Gender: "prefer_not_to_say"}) {
		t.Fatal("undisclosed gender can't satisfy same-gender")
	}
}
