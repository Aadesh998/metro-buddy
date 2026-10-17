package safety

import (
	"regexp"
	"strings"
)

// Phrases that suggest someone may be at risk, in English, Hindi and Hinglish. Matching only
// ever offers helplines to the writer; it never hides content or opens a moderation case.
var distressPatterns = []string{
	`kill (my ?self|myself)`, `want(ed)? to die`, `wanna die`, `end (it all|my life)`, `suicid`,
	`no reason to live`, `don'?t want to (live|be alive|exist)`, `better off (dead|without me)`,
	`(cut|cutting|hurt|hurting) myself`, `self[ -]?harm`, `take my (own )?life`, `can'?t go on`,
	`jump (off|in front of)`, `overdose`, `goodbye forever`,
	`khud ?kushi`, `aatmahatya`, `mar(na)? (jaana|jana) chaht?(a|i)`, `jeena nahi`, `jeene ka (mann|man) nahi`,
	`mujhe mar(na)? hai`, `zindagi khatam`, `sab khatam kar`,
	`मरना चाहत`, `आत्महत्या`, `खुदकुशी`, `जीना नहीं`, `ज़िंदगी खत्म`, `जिंदगी खत्म`,
}

var distressRe = regexp.MustCompile(`(?i)(` + strings.Join(distressPatterns, "|") + `)`)

// Negations of the form "not suicidal" or "jk" are deliberately not special-cased: offering a
// helpline to someone who didn't need it costs little, missing someone who did costs a lot.
func Distress(text string) bool {
	return distressRe.MatchString(text)
}
