package gateway

import (
	"unicode/utf16"
)

// truncateUTF16Suffix removes the last n UTF-16 code units from s.
// Agent turn_recover TextChars use JavaScript/UTF-16 lengths, matching the
// desktop chat reducer.
func truncateUTF16Suffix(s string, n int) string {
	if n <= 0 {
		return s
	}
	encoded := utf16.Encode([]rune(s))
	if n >= len(encoded) {
		return ""
	}
	return string(utf16.Decode(encoded[:len(encoded)-n]))
}
