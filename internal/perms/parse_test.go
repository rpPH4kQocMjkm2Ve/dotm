package perms

import (
	"testing"
)

func TestParseRulesEmpty(t *testing.T) {
	rules, err := ParseRules("")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

func TestParseRulesCommentsOnly(t *testing.T) {
	rules, err := ParseRules("# comment\n# another\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

func TestParseRulesBlankLines(t *testing.T) {
	rules, err := ParseRules("\n\n\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Errorf("expected 0 rules, got %d", len(rules))
	}
}

func TestParseRulesSingleFile(t *testing.T) {
	rules, err := ParseRules("etc/** 0644 root root\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	assertEqual(t, "pattern", r.Pattern, "etc/**")
	assertEqual(t, "dir_only", r.DirOnly, false)
	assertEqual(t, "mode", r.Mode, "0644")
	assertEqual(t, "owner", r.Owner, "root")
	assertEqual(t, "group", r.Group, "root")
	assertEqual(t, "line_num", r.LineNum, 1)
}

func TestParseRulesSingleDir(t *testing.T) {
	rules, err := ParseRules("etc/**/ 0755 root root\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	r := rules[0]
	assertEqual(t, "pattern", r.Pattern, "etc/**")
	assertEqual(t, "dir_only", r.DirOnly, true)
	assertEqual(t, "mode", r.Mode, "0755")
}

func TestParseRulesSkipMarker(t *testing.T) {
	rules, err := ParseRules("etc/** - - -\n")
	if err != nil {
		t.Fatal(err)
	}
	r := rules[0]
	assertEqual(t, "mode", r.Mode, "")
	assertEqual(t, "owner", r.Owner, "")
	assertEqual(t, "group", r.Group, "")
}

func TestParseRulesPartialSkip(t *testing.T) {
	rules, err := ParseRules("etc/** 0600 - root\n")
	if err != nil {
		t.Fatal(err)
	}
	r := rules[0]
	assertEqual(t, "mode", r.Mode, "0600")
	assertEqual(t, "owner", r.Owner, "")
	assertEqual(t, "group", r.Group, "root")
}

func TestParseRulesMultipleOrdering(t *testing.T) {
	content := `# Base
etc/**/  0755  root  root
etc/**   0644  root  root
# Sensitive
etc/security/**  0600  root  root
`
	rules, err := ParseRules(content)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	assertEqual(t, "rules[0].dir_only", rules[0].DirOnly, true)
	assertEqual(t, "rules[0].line_num", rules[0].LineNum, 2)
	assertEqual(t, "rules[1].dir_only", rules[1].DirOnly, false)
	assertEqual(t, "rules[1].line_num", rules[1].LineNum, 3)
	assertEqual(t, "rules[2].mode", rules[2].Mode, "0600")
	assertEqual(t, "rules[2].line_num", rules[2].LineNum, 5)
}

func TestParseRulesInvalidModeNotOctal(t *testing.T) {
	_, err := ParseRules("etc/** 9999 root root\n")
	assertParseError(t, err, "invalid mode")
}

func TestParseRulesInvalidModeThreeDigits(t *testing.T) {
	_, err := ParseRules("etc/** 644 root root\n")
	assertParseError(t, err, "invalid mode")
}

func TestParseRulesInvalidModeFiveDigits(t *testing.T) {
	_, err := ParseRules("etc/** 00644 root root\n")
	assertParseError(t, err, "invalid mode")
}

func TestParseRulesInvalidModeHasEight(t *testing.T) {
	_, err := ParseRules("etc/** 0889 root root\n")
	assertParseError(t, err, "invalid mode")
}

func TestParseRulesTooFewFields(t *testing.T) {
	_, err := ParseRules("etc/** 0644 root\n")
	assertParseError(t, err, "expected 4 fields")
}

func TestParseRulesTooManyFields(t *testing.T) {
	_, err := ParseRules("etc/** 0644 root root extra\n")
	assertParseError(t, err, "expected 4 fields")
}

func TestParseRulesEmptyPatternAfterSlashStrip(t *testing.T) {
	_, err := ParseRules("/ 0755 root root\n")
	assertParseError(t, err, "empty pattern")
}

func TestParseRulesLineNumbersWithBlanks(t *testing.T) {
	content := "\n# comment\n\netc/** 0644 root root\n"
	rules, err := ParseRules(content)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "line_num", rules[0].LineNum, 4)
}

func TestParseRulesTrailingSlashVariations(t *testing.T) {
	rules, err := ParseRules("etc/foo// 0755 root root\n")
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "dir_only", rules[0].DirOnly, true)
	assertEqual(t, "pattern", rules[0].Pattern, "etc/foo")
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func assertEqual[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}

func assertParseError(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", substr)
	}
	pe, ok := err.(*ParseError)
	if !ok {
		t.Fatalf("expected *ParseError, got %T: %v", err, err)
	}
	if !contains(pe.Error(), substr) {
		t.Errorf("error %q does not contain %q", pe.Error(), substr)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
