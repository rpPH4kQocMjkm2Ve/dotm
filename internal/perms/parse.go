package perms

import (
	"fmt"
	"regexp"
	"strings"
)

// PermRule is a single rule from the perms file.
type PermRule struct {
	Pattern string
	DirOnly bool   // trailing / in pattern means "directories only"
	Mode    string // "0644" or "" if skipped
	Owner   string // username or "" if skipped
	Group   string // group name or "" if skipped
	LineNum int
}

// ParseError is returned on malformed perms content.
type ParseError struct {
	LineNum int
	Message string
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d: %s", e.LineNum, e.Message)
}

var modeRe = regexp.MustCompile(`^0[0-7]{3}$`)

// ParseRules parses a perms file content into a list of rules.
//
// Format per line:
//
//	<glob-pattern>  <mode|->  <owner|->  <group|->
//
// Pattern ending with / matches directories only; without — files only.
// - means "don't change this attribute".
func ParseRules(content string) ([]PermRule, error) {
	var rules []PermRule

	for lineNum, rawLine := range strings.Split(content, "\n") {
		lineNum++ // 1-based
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) != 4 {
			return nil, &ParseError{
				LineNum: lineNum,
				Message: fmt.Sprintf(
					"expected 4 fields (pattern mode owner group), got %d: %q",
					len(parts), rawLine),
			}
		}

		pattern, modeStr, ownerStr, groupStr := parts[0], parts[1], parts[2], parts[3]

		var mode string
		if modeStr != "-" {
			if !modeRe.MatchString(modeStr) {
				return nil, &ParseError{
					LineNum: lineNum,
					Message: fmt.Sprintf(
						"invalid mode %q, expected 4-digit octal (e.g. 0644) or '-'",
						modeStr),
				}
			}
			mode = modeStr
		}

		owner := ownerStr
		if owner == "-" {
			owner = ""
		}
		group := groupStr
		if group == "-" {
			group = ""
		}

		dirOnly := strings.HasSuffix(pattern, "/")
		pattern = strings.TrimRight(pattern, "/")

		if pattern == "" {
			return nil, &ParseError{
				LineNum: lineNum,
				Message: "empty pattern",
			}
		}

		rules = append(rules, PermRule{
			Pattern: pattern,
			DirOnly: dirOnly,
			Mode:    mode,
			Owner:   owner,
			Group:   group,
			LineNum: lineNum,
		})
	}

	return rules, nil
}
