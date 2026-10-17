package identity

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"

	"metro-buddy/server/internal/pii"
)

// Aliases read like a handle someone would pick: evocative, a little Indian, never silly.
var adjectives = []string{
	"Midnight", "Monsoon", "Saffron", "Indigo", "Velvet", "Silent", "Lunar", "Solar", "Amber", "Crimson",
	"Cobalt", "Golden", "Silver", "Neon", "Electric", "Cosmic", "Stellar", "Arctic", "Misty", "Wild",
	"Gentle", "Brave", "Calm", "Clever", "Curious", "Daring", "Dreamy", "Fearless", "Free", "Kind",
	"Lucky", "Mellow", "Mystic", "Noble", "Quiet", "Rapid", "Restless", "Serene", "Swift", "Urban",
	"Wandering", "Hidden", "Bright", "Copper", "Emerald", "Ivory", "Jade", "Scarlet", "Teal", "Ruby",
	"Twilight", "Dawn", "Dusk", "Starlit", "Moonlit", "Sunlit", "Rainy", "Breezy", "Stormy", "Frosty",
	"Zen", "Rogue", "Lone", "Bold", "Vivid", "Sonic", "Royal", "Pixel", "Retro", "Turbo",
}

var nouns = []string{
	"Falcon", "Phoenix", "Tiger", "Panther", "Leopard", "Wolf", "Fox", "Raven", "Hawk", "Eagle",
	"Koel", "Peacock", "Heron", "Kingfisher", "Sparrow", "Owl", "Cheetah", "Lynx", "Dolphin", "Orca",
	"Comet", "Nebula", "Orbit", "Nova", "Meteor", "Galaxy", "Horizon", "Aurora", "Eclipse", "Zenith",
	"Voyager", "Nomad", "Ranger", "Pilot", "Rider", "Wanderer", "Drifter", "Seeker", "Dreamer", "Explorer",
	"Echo", "Pulse", "Spark", "Ember", "Blaze", "Storm", "Tide", "Breeze", "Thunder", "Ripple",
	"Summit", "Canyon", "Atlas", "Lotus", "Banyan", "Cedar", "Willow", "Orchid", "Jasmine", "Maple",
	"Raga", "Sitar", "Tabla", "Verse", "Melody", "Rhythm", "Lantern", "Compass", "Arrow", "Anchor",
}

// Words from the first generator; still valid so older aliases can be kept and reused.
var legacyWords = []string{
	"Chai", "Masala", "Sleepy", "Cosmic", "Sunny", "Rusty", "Jolly", "Witty", "Fuzzy", "Minty", "Spicy",
	"Tangy", "Crispy", "Humble", "Jazzy", "Lazy", "Merry", "Nimble", "Peppy", "Shy", "Snappy", "Tiny",
	"Zesty", "Bouncy", "Chill", "Cloudy", "Dusky", "Glowing", "Hazy", "Night", "Pastel", "Starry", "Sugar",
	"Woolly", "Filter", "Ginger", "Pista", "Pigeon", "Mynah", "Kite", "Rickshaw", "Samosa", "Jalebi",
	"Ladoo", "Pakora", "Parrot", "Panda", "Otter", "Robin", "Firefly", "Pebble", "Notebook", "Pencil",
	"Teacup", "Kettle", "Guitar", "Drum", "Rocket", "Planet", "Moon", "Star", "Wave", "River", "Island",
	"Harbor", "Garden", "Mango", "Coconut", "Biscuit", "Cookie", "Muffin", "Dosa", "Idli", "Vada",
	"Paratha", "Kulfi", "Rasgulla", "Barfi", "Halwa", "Pulao", "Chutney", "Turtle", "Squirrel",
}

var (
	adjSet  = toSet(adjectives, legacyWords)
	nounSet = toSet(nouns, legacyWords)
	aliasRe = regexp.MustCompile(`^([A-Z][a-z]+) ([A-Z][a-z]+)(?: (\d{2}))?$`)
	// A random seed for a generated avatar, or one of the 20 illustrated avatars bundled in the app.
	seedRe = regexp.MustCompile(`^(?:[a-z0-9]{12}|img-[mf](?:0[1-9]|10))$`)
)

func toSet(lists ...[]string) map[string]bool {
	m := map[string]bool{}
	for _, words := range lists {
		for _, w := range words {
			m[w] = true
		}
	}
	return m
}

func pick(n int) int {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		panic(err)
	}
	return int(v.Int64())
}

// NewAlias returns a two-word alias; numbered adds a 2-digit suffix for when the
// plain pair is already taken in the city.
func NewAlias(numbered bool) string {
	a := adjectives[pick(len(adjectives))] + " " + nouns[pick(len(nouns))]
	if numbered {
		a = fmt.Sprintf("%s %02d", a, 10+pick(90))
	}
	return a
}

// Only generator output is accepted, so an alias can never carry a name or contact detail.
func ValidAlias(a string) bool {
	m := aliasRe.FindStringSubmatch(a)
	return m != nil && adjSet[m[1]] && nounSet[m[2]] && m[1] != m[2]
}

// AliasKey is the uniqueness key: case, spaces, dots and underscores don't make a different name.
func AliasKey(a string) string {
	return strings.ToLower(strings.NewReplacer(" ", "", ".", "", "_", "").Replace(a))
}

// aliasKeySQL is AliasKey for a column, for matching names already in the database.
const aliasKeySQL = "LOWER(REPLACE(REPLACE(REPLACE(alias, ' ', ''), '.', ''), '_', ''))"

var (
	customRe = regexp.MustCompile(`^[\p{L}\p{M}][\p{L}\p{M}\p{N} ._]{2,19}$`)
	digitRe  = regexp.MustCompile(`\p{N}`)
	// Names that would read as staff, the app itself or an emergency service.
	reservedWords = []string{"admin", "moderator", "metrobuddy", "support", "official", "staff", "safetyteam", "helpline", "police", "system", "deleted"}
	blockedWords  = []string{"fuck", "shit", "bitch", "slut", "whore", "rape", "nigg", "chutiya", "madarchod", "bhenchod", "behenchod", "randi", "gandu", "bhosdi", "lund", "chut"}
)

// AliasProblem explains why a self-chosen name can't be used, or "" if it can.
// Names must not carry contact details: no phone-like digits, emails, links or @handles.
func AliasProblem(a string) string {
	switch {
	case a != strings.TrimSpace(a) || strings.Contains(a, "  "):
		return "remove extra spaces"
	case !customRe.MatchString(a):
		return "use 3 to 20 letters or numbers; spaces, dots and underscores are fine"
	case len(digitRe.FindAllString(a, -1)) > 3:
		return "use at most 3 numbers so it can't look like a phone number"
	}
	if r := pii.Mask(a, pii.Options{}); r.Flags != 0 {
		return "that looks like contact details"
	}
	key := AliasKey(a)
	for _, w := range reservedWords {
		if strings.Contains(key, w) {
			return "that name is reserved"
		}
	}
	for _, w := range blockedWords {
		if strings.Contains(key, w) {
			return "pick a kinder name"
		}
	}
	return ""
}

const seedChars = "abcdefghijklmnopqrstuvwxyz0123456789"

func NewAvatarSeed() string {
	b := make([]byte, 12)
	for i := range b {
		b[i] = seedChars[pick(len(seedChars))]
	}
	return string(b)
}

func ValidSeed(s string) bool { return seedRe.MatchString(s) }
