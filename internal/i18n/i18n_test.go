package i18n

import "testing"

func TestNewBundle(t *testing.T) {
	// NewBundle panics if the source-locale files are malformed/missing.
	// This asserts the embedded catalogs parse cleanly at startup.
	b := NewBundle()
	if b == nil {
		t.Fatal("NewBundle() returned nil")
	}
}

func TestBundleLocalize(t *testing.T) {
	b := NewBundle()

	t.Run("source locale returns source string", func(t *testing.T) {
		got, err := b.Localize("en", "TaskDueSoonTitle", map[string]any{"Title": "Ship it"}, nil)
		if err != nil {
			t.Fatalf("Localize(en) error: %v", err)
		}
		want := "Task due: Ship it"
		if got != want {
			t.Errorf("Localize(en, TaskDueSoonTitle) = %q, want %q", got, want)
		}
	})

	t.Run("missing message falls back gracefully", func(t *testing.T) {
		// A message ID with no catalog entry: go-i18n returns the ID or an
		// error. MustLocalize returns the ID (no panic).
		got := b.MustLocalize("en", "NonexistentMessageID", nil, nil)
		if got != "NonexistentMessageID" {
			t.Errorf("MustLocalize(missing ID) = %q, want the ID back", got)
		}
	})

	t.Run("unsupported locale falls back to source", func(t *testing.T) {
		// "de" normalizes to "en": should return the source string.
		got, err := b.Localize("de", "TaskDueSoonTitle", map[string]any{"Title": "Ship it"}, nil)
		if err != nil {
			t.Fatalf("Localize(de) error: %v", err)
		}
		want := "Task due: Ship it"
		if got != want {
			t.Errorf("Localize(de) = %q, want %q (source fallback)", got, want)
		}
	})

	t.Run("template placeholders fill", func(t *testing.T) {
		got, err := b.Localize("en", "ChatMentionBody",
			map[string]any{"Sender": "Alice", "Conversation": "#general"}, nil)
		if err != nil {
			t.Fatalf("Localize(ChatMentionBody) error: %v", err)
		}
		want := "You were mentioned by Alice in #general"
		if got != want {
			t.Errorf("Localize(ChatMentionBody) = %q, want %q", got, want)
		}
	})
}

// TestBundlePlural_FrenchZeroOne asserts the French one={0,1} vs English
// one={1} boundary: the edge case "fr" was chosen to exercise. A message
// with "one"/"other" plural categories must select the right form based on
// PluralCount.
func TestBundlePlural_FrenchZeroOne(t *testing.T) {
	b := NewBundle()

	tests := []struct {
		locale string
		count  int
		want   string
	}{
		{"fr", 0, "0 commentaire"},
		{"en", 0, "0 comments"},
		{"fr", 1, "1 commentaire"},
		{"en", 1, "1 comment"},
		{"fr", 2, "2 commentaires"},
		{"en", 2, "2 comments"},
	}

	for _, tt := range tests {
		t.Run(tt.locale+"/"+string(rune('0'+tt.count)), func(t *testing.T) {
			got := b.MustLocalize(tt.locale, "CommentCount",
				map[string]any{"Count": tt.count},
				tt.count)
			if got != tt.want {
				t.Errorf("MustLocalize(%q, CommentCount, {Count:%d}, %d)\n  got:  %q\n  want: %q",
					tt.locale, tt.count, tt.count, got, tt.want)
			}
		})
	}
}
