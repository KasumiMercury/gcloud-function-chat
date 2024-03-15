package functions

import (
	"github.com/rivo/uniseg"
	"strings"
)

func RemoveEmoji(s string) string {
	var resRunes []rune

	gr := uniseg.NewGraphemes(s)
	for gr.Next() {
		// Get the current rune
		r := gr.Runes()

		if isEmoji(r[0]) {
			continue
		}

		// If the rune is not an emoji, add it to the result
		resRunes = append(resRunes, r...)
	}

	// Convert the runes back to a string
	res := string(resRunes)
	res = strings.TrimSpace(res)

	if res == "" {
		return ""
	}

	// Return the normalized string
	return res
}

func isEmoji(r rune) bool {
	if isEmojiInRange(r) {
		return true
	}

	if isSymbolInRange(r) {
		return true
	}

	if isSupplementaryEmojiInRange(r) {
		return true
	}

	if isDescriptorInRange(r) {
		return true
	}

	return false
}

func isEmojiInRange(r rune) bool {
	return (r >= 0x1F600 && r <= 0x1F64F) ||
		(r >= 0x1F300 && r <= 0x1F5FF) ||
		(r >= 0x1F680 && r <= 0x1F6FF)
}

func isSymbolInRange(r rune) bool {
	return (r >= 0x2600 && r <= 0x26FF) ||
		(r >= 0x2700 && r <= 0x27BF) ||
		(r >= 0x2000 && r <= 0x2B00) ||
		(r >= 0x1F300 && r <= 0x1F5FF) ||
		(r >= 0x1F680 && r <= 0x1F64F) ||
		(r >= 0x1F900 && r <= 0x1F9FF) ||
		(r >= 0x1FA70 && r <= 0x1FAFF) ||
		(r >= 0x1F6B0 && r <= 0x1F6FF) ||
		(r >= 0x1F780 && r <= 0x1F7F0) ||
		(r >= 0x1F100 && r <= 0x1F1FF)
}

func isSupplementaryEmojiInRange(r rune) bool {
	return r >= 0x1F900 && r <= 0x1F9FF
}

func isDescriptorInRange(r rune) bool {
	return r >= 0x1F000 && r <= 0x1F0FF
}
