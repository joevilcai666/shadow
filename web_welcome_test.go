package shadow

import (
	"os"
	"strings"
	"testing"
)

func TestWelcomeEmptyStateCanShowAhaDemo(t *testing.T) {
	content, err := os.ReadFile("web/src/pages/Welcome.tsx")
	if err != nil {
		t.Fatalf("read Welcome.tsx: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "View Aha Demo") {
		t.Fatal("Welcome empty state should offer the seeded Aha demo when no candidate rules exist")
	}
	if !strings.Contains(text, "onClick={skipAll}") {
		t.Fatal("Welcome empty state should route to the demo step, not only enter the console")
	}
}
