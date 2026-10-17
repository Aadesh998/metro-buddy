package identity

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"
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
	seedRe  = regexp.MustCompile(`^[a-z0-9]{12}$`)
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

func AliasKey(a string) string { return strings.ToLower(strings.ReplaceAll(a, " ", "")) }

const seedChars = "abcdefghijklmnopqrstuvwxyz0123456789"

func NewAvatarSeed() string {
	b := make([]byte, 12)
	for i := range b {
		b[i] = seedChars[pick(len(seedChars))]
	}
	return string(b)
}

func ValidSeed(s string) bool { return seedRe.MatchString(s) }
