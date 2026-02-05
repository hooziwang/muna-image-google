package cmd

import "testing"

func TestFilterAPIKeys_NoPatterns(t *testing.T) {
	keys := []string{"abc", "def"}
	out, err := filterAPIKeys(keys, nil)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(out))
	}
}

func TestFilterAPIKeys_EmptyPattern(t *testing.T) {
	keys := []string{"abc"}
	_, err := filterAPIKeys(keys, []string{""})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestFilterAPIKeys_NoMatch(t *testing.T) {
	keys := []string{"abc", "def"}
	_, err := filterAPIKeys(keys, []string{"zzz"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestFilterAPIKeys_SingleMatch(t *testing.T) {
	keys := []string{"AAA-111", "BBB-222"}
	out, err := filterAPIKeys(keys, []string{"BBB"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 key, got %d", len(out))
	}
	if out[0] != "BBB-222" {
		t.Fatalf("expected BBB-222, got %q", out[0])
	}
}

func TestFilterAPIKeys_MultiPatternUnion(t *testing.T) {
	keys := []string{"AAA-111", "BBB-222", "CCC-333"}
	out, err := filterAPIKeys(keys, []string{"AAA", "CCC"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(out))
	}
}
