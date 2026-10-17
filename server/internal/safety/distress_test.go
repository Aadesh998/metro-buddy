package safety

import "testing"

func TestDistressDetector(t *testing.T) {
	yes := []string{
		"honestly I just want to die", "thinking about ending... no, end it all tonight", "I keep cutting myself",
		"no reason to live anymore", "everyone is better off without me", "mujhe marna hai yaar", "khudkushi ka soch raha",
		"jeene ka mann nahi karta", "मैं मरना चाहता हूँ", "Suicidal thoughts again",
	}
	no := []string{
		"this traffic is killing me lol", "the meeting was dead boring", "I'd die for a seat on the Yellow line",
		"killed it in the presentation", "metro AC is a lifesaver",
	}
	for _, s := range yes {
		if !Distress(s) {
			t.Errorf("missed %q", s)
		}
	}
	for _, s := range no {
		if Distress(s) {
			t.Errorf("false alarm %q", s)
		}
	}
}
