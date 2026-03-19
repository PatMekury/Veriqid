// Package mnemonic provides a simple 12-word mnemonic encoding for Veriqid identity keys.
//
// Design:
//   - 256 carefully-chosen, kid-friendly English words (1 byte per word)
//   - 12 words = 12 bytes (96 bits) of entropy
//   - SHA-256(12 bytes) = 32-byte MSK (master secret key)
//   - The mnemonic is the portable, human-readable transport format
//   - The MSK is the cryptographic secret used for proofs
//
// Flow:
//   Parent Portal generates → 12-word phrase → given to child
//   Child pastes into extension → decoded to 12 bytes → SHA-256 → 32-byte MSK
//   Extension sends MSK to bridge → bridge generates proofs
package mnemonic

import (
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"strings"
)

// Generate creates a new random 12-word mnemonic phrase and returns:
//   - The 12-word phrase (space-separated)
//   - The derived 32-byte MSK
//   - An error if random generation fails
func Generate() (phrase string, msk []byte, err error) {
	// Generate 12 random bytes
	entropy := make([]byte, 12)
	if _, err := rand.Read(entropy); err != nil {
		return "", nil, fmt.Errorf("failed to generate random entropy: %w", err)
	}

	// Encode as 12 words
	words := make([]string, 12)
	for i, b := range entropy {
		words[i] = wordlist[b]
	}
	phrase = strings.Join(words, " ")

	// Derive 32-byte MSK via SHA-256
	hash := sha256.Sum256(entropy)
	msk = hash[:]

	return phrase, msk, nil
}

// Decode converts a 12-word mnemonic phrase back to the 32-byte MSK.
// Returns the raw 12-byte entropy and the derived 32-byte MSK.
func Decode(phrase string) (entropy []byte, msk []byte, err error) {
	words := strings.Fields(strings.TrimSpace(strings.ToLower(phrase)))
	if len(words) != 12 {
		return nil, nil, fmt.Errorf("expected 12 words, got %d", len(words))
	}

	entropy = make([]byte, 12)
	for i, word := range words {
		idx, ok := wordIndex[word]
		if !ok {
			return nil, nil, fmt.Errorf("unknown word at position %d: %q", i+1, word)
		}
		entropy[i] = idx
	}

	hash := sha256.Sum256(entropy)
	msk = hash[:]

	return entropy, msk, nil
}

// Validate checks if a mnemonic phrase is valid (12 known words).
func Validate(phrase string) error {
	_, _, err := Decode(phrase)
	return err
}

// wordIndex is the reverse lookup: word → byte value
var wordIndex map[string]byte

func init() {
	wordIndex = make(map[string]byte, 256)
	for i, w := range wordlist {
		wordIndex[w] = byte(i)
	}
}

// wordlist contains 256 distinct, memorable English words.
// Each word maps to one byte value (0–255).
// Words are chosen to be: distinct from each other, easy to spell,
// kid-friendly, and visually unambiguous.
var wordlist = [256]string{
	// 0x00–0x0F
	"apple", "arrow", "badge", "beach", "bird", "bloom", "boat", "brave",
	"brick", "bridge", "bright", "brook", "brush", "cabin", "candy", "cape",
	// 0x10–0x1F
	"castle", "chain", "chalk", "chase", "chest", "cliff", "cloud", "coast",
	"comet", "coral", "crane", "creek", "crown", "crystal", "dance", "dawn",
	// 0x20–0x2F
	"delta", "desert", "dock", "dolphin", "dragon", "dream", "drift", "drum",
	"eagle", "earth", "ember", "falcon", "field", "flame", "flash", "float",
	// 0x30–0x3F
	"flood", "flute", "forest", "forge", "fossil", "frost", "garden", "gate",
	"gem", "glade", "globe", "golden", "grain", "grape", "grass", "grove",
	// 0x40–0x4F
	"harbor", "hawk", "haven", "heart", "hedge", "hero", "hill", "hollow",
	"honey", "horizon", "horse", "house", "island", "ivory", "jade", "jewel",
	// 0x50–0x5F
	"jungle", "kite", "knight", "lake", "lamp", "lark", "leaf", "legend",
	"lemon", "light", "lily", "lion", "lodge", "lotus", "lunar", "magic",
	// 0x60–0x6F
	"maple", "marble", "marsh", "meadow", "melody", "mesa", "meteor", "mint",
	"mirror", "moon", "moss", "mountain", "mural", "nebula", "nest", "noble",
	// 0x70–0x7F
	"north", "nova", "oak", "oasis", "ocean", "olive", "orbit", "orchid",
	"osprey", "otter", "owl", "palm", "panda", "panther", "path", "pearl",
	// 0x80–0x8F
	"pebble", "pepper", "phoenix", "pilot", "pine", "pixel", "plain", "planet",
	"plum", "polar", "pond", "prism", "pulse", "puzzle", "quail", "quartz",
	// 0x90–0x9F
	"quest", "rabbit", "rain", "rainbow", "raven", "reef", "ridge", "ring",
	"river", "robin", "rocket", "rose", "ruby", "sage", "sail", "salmon",
	// 0xA0–0xAF
	"sand", "sapphire", "scout", "seed", "shadow", "shark", "shell", "shield",
	"shore", "silver", "sky", "slate", "snow", "solar", "spark", "spear",
	// 0xB0–0xBF
	"spiral", "spring", "spruce", "square", "star", "steam", "steel", "stone",
	"storm", "stream", "summit", "sun", "surf", "swan", "swift", "sword",
	// 0xC0–0xCF
	"terra", "thistle", "thorn", "thunder", "tide", "tiger", "timber", "torch",
	"tower", "trail", "tree", "tropic", "tulip", "tunnel", "turtle", "valley",
	// 0xD0–0xDF
	"vapor", "vault", "velvet", "vine", "violet", "vista", "voice", "volcano",
	"voyage", "walnut", "wave", "whale", "wheat", "willow", "wind", "winter",
	// 0xE0–0xEF
	"wolf", "wonder", "wood", "wren", "yarn", "yew", "zenith", "zephyr",
	"zinc", "atlas", "aurora", "blaze", "breeze", "canyon", "cedar", "cider",
	// 0xF0–0xFF
	"crest", "dusk", "fern", "flint", "glacier", "glow", "heron", "indigo",
	"jasper", "lava", "mangrove", "nectar", "onyx", "opal", "peak", "rapid",
}
