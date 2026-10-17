package identity

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

var adjectives = []string{
	"Chai", "Masala", "Monsoon", "Neon", "Sleepy", "Brave", "Quiet", "Cosmic", "Lucky", "Sunny",
	"Rusty", "Velvet", "Mellow", "Breezy", "Jolly", "Witty", "Calm", "Fuzzy", "Golden", "Silver",
	"Indigo", "Saffron", "Minty", "Spicy", "Tangy", "Crispy", "Dreamy", "Gentle", "Humble", "Jazzy",
	"Kind", "Lazy", "Merry", "Nimble", "Peppy", "Rapid", "Shy", "Snappy", "Swift", "Tiny",
	"Zesty", "Bouncy", "Chill", "Cloudy", "Dusky", "Electric", "Frosty", "Glowing", "Hazy", "Misty",
	"Night", "Pastel", "Rainy", "Starry", "Stormy", "Sugar", "Twilight", "Vivid", "Wandering", "Woolly",
	"Filter", "Ginger", "Maple", "Pista",
}

var nouns = []string{
	"Comet", "Pigeon", "Mynah", "Peacock", "Kite", "Lotus", "Banyan", "Rickshaw", "Samosa", "Jalebi",
	"Ladoo", "Pakora", "Parrot", "Koel", "Tiger", "Panda", "Otter", "Falcon", "Sparrow", "Robin",
	"Firefly", "Pebble", "Breeze", "Thunder", "Ripple", "Lantern", "Compass", "Notebook", "Pencil", "Teacup",
	"Kettle", "Guitar", "Tabla", "Sitar", "Drum", "Rocket", "Planet", "Moon", "Star", "Meteor",
	"Wave", "River", "Island", "Harbor", "Garden", "Mango", "Coconut", "Biscuit", "Cookie", "Muffin",
	"Dosa", "Idli", "Vada", "Paratha", "Kulfi", "Rasgulla", "Barfi", "Halwa", "Pulao", "Chutney",
	"Turtle", "Squirrel", "Owl", "Dolphin",
}

var (
	adjSet  = toSet(adjectives)
	nounSet = toSet(nouns)
	aliasRe = regexp.MustCompile(`^([A-Z][a-z]+) ([A-Z][a-z]+) (\d{2})$`)
	seedRe  = regexp.MustCompile(`^[a-z0-9]{12}$`)
)

func toSet(words []string) map[string]bool {
	m := make(map[string]bool, len(words))
	for _, w := range words {
		m[w] = true
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

func NewAlias() string {
	return fmt.Sprintf("%s %s %02d", adjectives[pick(len(adjectives))], nouns[pick(len(nouns))], 10+pick(90))
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
