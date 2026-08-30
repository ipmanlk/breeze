package lexorank_test

import (
	"sort"
	"testing"

	"ipmanlk/breeze/internal/lexorank"
)

func TestGenerateKeyBetween(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want string
	}{
		{"both empty", "", "", "h"},
		{"before h", "", "h", "8"},
		{"after h", "h", "", "q"},
		{"between 0-z", "0", "z", "h"},
		{"between adjacent 0-1", "0", "1", "05"},
		{"between 00-1", "00", "1", "01"},
		{"after z", "z", "", "zz"},
		{"between a-f", "a", "f", "c"},
		{"between 8-g", "8", "g", "c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lexorank.GenerateKeyBetween(tt.a, tt.b)
			if err != nil {
				t.Fatalf("GenerateKeyBetween(%q, %q) error: %v", tt.a, tt.b, err)
			}
			if got != tt.want {
				t.Errorf("GenerateKeyBetween(%q, %q) = %q, want %q", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestGenerateKeyBetweenErrors(t *testing.T) {
	tests := []struct {
		name string
		a, b string
	}{
		{"before 0", "", "0"},
		{"identical", "h", "h"},
		{"invalid a", "ab!", "xyz"},
		{"invalid b", "abc", "xy!"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := lexorank.GenerateKeyBetween(tt.a, tt.b)
			if err == nil {
				t.Errorf("GenerateKeyBetween(%q, %q) should error", tt.a, tt.b)
			}
		})
	}
}

func TestOrderingInvariant(t *testing.T) {
	keys := []string{lexorank.FirstKey()}
	for range 500 {
		key, err := lexorank.GenerateKeyBetween(keys[len(keys)-1], "")
		if err != nil {
			t.Fatal(err)
		}
		if key <= keys[len(keys)-1] {
			t.Fatalf("key %q not after %q", key, keys[len(keys)-1])
		}
		keys = append(keys, key)
	}
	if !sort.StringsAreSorted(keys) {
		t.Error("final keys not sorted")
	}
}

func TestBeforeLowers(t *testing.T) {
	keys := []string{lexorank.FirstKey()}
	succeeded := 0
	for range 10 {
		key, err := lexorank.GenerateKeyBetween("", keys[0])
		if err != nil {
			break
		}
		if key >= keys[0] {
			t.Fatalf("key %q not before %q", key, keys[0])
		}
		keys = append([]string{key}, keys...)
		succeeded++
	}
	if succeeded < 4 {
		t.Errorf("expected at least 4 before-insertions, got %d", succeeded)
	}
}

func TestInsertBetweenCreatesStrictOrder(t *testing.T) {
	pairs := []struct{ a, b string }{
		{"0", "z"},
		{"h", "s"},
		{"a", "f"},
		{"00", "01"},
		{"z", "zz"},
		{"a0", "b0"},
	}

	for _, p := range pairs {
		key, err := lexorank.GenerateKeyBetween(p.a, p.b)
		if err != nil {
			t.Logf("Cannot generate between %q and %q: %v", p.a, p.b, err)
			continue
		}
		if !(key > p.a) {
			t.Errorf("key %q not > %q", key, p.a)
		}
		if !(key < p.b) {
			t.Errorf("key %q not < %q", key, p.b)
		}
	}
}

func TestInvalidInput(t *testing.T) {
	_, err := lexorank.GenerateKeyBetween("abc!", "xyz")
	if err == nil {
		t.Error("expected error for invalid characters")
	}

	if lexorank.IsValidKey("abc!") {
		t.Error("IsValidKey should return false for invalid key")
	}

	if !lexorank.IsValidKey("a0b3z") {
		t.Error("IsValidKey should return true for valid key")
	}
}

func TestConsecutiveInsertsBetweenAdjacent(t *testing.T) {
	allKeys := []string{"0", "z"}
	for range 20 {
		var newKeys []string
		for i := 0; i < len(allKeys)-1; i++ {
			key, err := lexorank.GenerateKeyBetween(allKeys[i], allKeys[i+1])
			if err != nil {
				continue
			}
			if key <= allKeys[i] || key >= allKeys[i+1] {
				t.Fatalf("%q not between %q and %q", key, allKeys[i], allKeys[i+1])
			}
			newKeys = append(newKeys, key)
		}
		allKeys = append(allKeys, newKeys...)
		sort.Strings(allKeys)
	}
	if !sort.StringsAreSorted(allKeys) {
		t.Error("final keys not sorted")
	}
}

func TestAfterDoesNotError(t *testing.T) {
	key := lexorank.FirstKey()
	for range 1000 {
		var err error
		key, err = lexorank.GenerateKeyBetween(key, "")
		if err != nil {
			t.Fatalf("error at %q: %v", key, err)
		}
	}
}

func TestIsValidKey(t *testing.T) {
	if lexorank.IsValidKey("") {
		t.Error("empty string should not be valid")
	}
	if lexorank.IsValidKey("h") != true {
		t.Error("'h' should be valid")
	}
	if lexorank.IsValidKey("0a9z") != true {
		t.Error("'0a9z' should be valid")
	}
}

func TestFirstKey(t *testing.T) {
	if lexorank.FirstKey() != "h" {
		t.Errorf("expected h, got %s", lexorank.FirstKey())
	}
}
