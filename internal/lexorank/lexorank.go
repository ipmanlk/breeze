package lexorank

import (
	"errors"
	"strings"
)

const base = 36

var digits = "0123456789abcdefghijklmnopqrstuvwxyz"

func digitVal(c byte) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	if c >= 'a' && c <= 'z' {
		return int(c - 'a' + 10)
	}
	return -1
}

func valChar(v int) byte {
	if v < 0 || v >= base {
		return '0'
	}
	return digits[v]
}

func validate(s string) error {
	for i := 0; i < len(s); i++ {
		if digitVal(s[i]) < 0 {
			return errors.New("invalid character in key: " + string(s[i]))
		}
	}
	return nil
}

func before(b string) (string, error) {
	for i := 0; i < len(b); i++ {
		db := digitVal(b[i])
		if db > 0 {
			return b[:i] + string(valChar(db/2)), nil
		}
	}
	return "", errors.New("no room before " + b)
}

func after(a string) (string, error) {
	for i := 0; i < len(a); i++ {
		da := digitVal(a[i])
		if da < base-1 {
			avg := (da + base) / 2
			return a[:i] + string(valChar(avg)), nil
		}
	}
	return a + "z", nil
}

func GenerateKeyBetween(a, b string) (string, error) {
	if a != "" {
		if err := validate(a); err != nil {
			return "", err
		}
	}
	if b != "" {
		if err := validate(b); err != nil {
			return "", err
		}
	}

	if a == "" && b == "" {
		return "h", nil
	}
	if a == "" {
		return before(b)
	}
	if b == "" {
		return after(a)
	}

	if a >= b {
		if a == b {
			return "", errors.New("keys are identical")
		}
		return "", errors.New("a must be less than b: " + a + " >= " + b)
	}

	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}

	prefix := a[:i]

	if i >= len(a) {
		db := digitVal(b[i])
		avg := db / 2
		if avg < db {
			return prefix + string(valChar(avg)), nil
		}
		return "", errors.New("no room between " + a + " and " + b)
	}

	if i >= len(b) {
		return "", errors.New("no room between " + a + " and " + b)
	}

	da := digitVal(a[i])
	db := digitVal(b[i])

	if da >= db {
		return "", errors.New("invalid ordering: " + a + " vs " + b)
	}

	avg := (da + db) / 2

	if avg > da {
		return prefix + string(valChar(avg)), nil
	}

	result := prefix + string(a[i])

	for j := i + 1; j < len(a); j++ {
		dj := digitVal(a[j])
		if dj < base-1 {
			return result + a[i+1:j] + string(valChar(dj+1)), nil
		}
		result += string(a[j])
	}

	return result + "5", nil
}

func FirstKey() string {
	return "h"
}

func IsValidKey(s string) bool {
	return len(s) > 0 && strings.IndexFunc(s, func(r rune) bool {
		return digitVal(byte(r)) < 0
	}) == -1
}
