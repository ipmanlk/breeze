package seed

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"ipmanlk/breeze/internal/lexorank"
)

// newUUID generates a new UUID string.
func newUUID() string {
	return uuid.New().String()
}

// slug converts a name to a URL-friendly slug.
func slug(name string) string {
	s := strings.ToLower(name)
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else {
			result.WriteRune('-')
		}
	}
	s = strings.Trim(result.String(), "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

// randomInt returns a pseudo-random int.
func randomInt(max int) int {
	return int(time.Now().UnixNano() % int64(max))
}

// intPtr returns a pointer to the given int.
func intPtr(i int) *int {
	return &i
}

// timePtr returns a pointer to the given time.Time.
func timePtr(t time.Time) *time.Time {
	return &t
}

// formatTime formats a time as "2006-01-02 15:04:05" UTC.
func formatTime(t time.Time) string {
	return t.UTC().Format("2006-01-02 15:04:05")
}

// nextPositionKey generates a new lexorank key.
func nextPositionKey(lastKey *string) string {
	var prev string
	if lastKey != nil {
		prev = *lastKey
	}
	key, err := lexorank.GenerateKeyBetween(prev, "")
	if err != nil {
		key = lexorank.FirstKey()
	}
	*lastKey = key
	return key
}
