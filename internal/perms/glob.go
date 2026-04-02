package perms

import (
	"regexp"
	"strings"
	"sync"
)

// MatchGlob checks if path matches pattern (with ** support).
//
// Supports:
//   - **  zero or more complete path segments
//   - *   anything except /
//   - ?   any single character except /
//
// All other characters are matched literally (regex metacharacters escaped).
func MatchGlob(pattern, path string) bool {
	return compileGlob(pattern).MatchString(path)
}

var (
	globCache   = make(map[string]*regexp.Regexp)
	globCacheMu sync.RWMutex
)

func compileGlob(pattern string) *regexp.Regexp {
	// Fast path: read lock for cache hits.
	globCacheMu.RLock()
	if cached, ok := globCache[pattern]; ok {
		globCacheMu.RUnlock()
		return cached
	}
	globCacheMu.RUnlock()

	// Slow path: write lock for cache misses.
	globCacheMu.Lock()
	defer globCacheMu.Unlock()

	// Double-check after acquiring write lock.
	if cached, ok := globCache[pattern]; ok {
		return cached
	}

	regex := buildGlobRegex(pattern)
	compiled := regexp.MustCompile("^" + regex + "$")
	globCache[pattern] = compiled
	return compiled
}

func buildGlobRegex(pattern string) string {
	segments := strings.Split(pattern, "**")

	if len(segments) == 1 {
		return escapeGlobSegment(segments[0])
	}

	escaped := make([]string, len(segments))
	for i, s := range segments {
		escaped[i] = escapeGlobSegment(s)
	}

	var b strings.Builder
	b.WriteString(escaped[0])

	for i := 1; i < len(escaped); i++ {
		leftSlash := strings.HasSuffix(segments[i-1], "/")
		rightSlash := strings.HasPrefix(segments[i], "/")

		switch {
		case leftSlash && rightSlash:
			// a/ ** /b  →  a(/.*)?/b
			// Matches: a/b, a/x/b, a/x/y/b
			s := b.String()
			b.Reset()
			b.WriteString(s[:len(s)-1]) // drop trailing /
			b.WriteString("(/.*)?")
			b.WriteString(escaped[i])

		case leftSlash && escaped[i] == "":
			// a/ ** (end)  →  a(/.*)?
			// Matches: a, a/x, a/x/y
			s := b.String()
			b.Reset()
			b.WriteString(s[:len(s)-1])
			b.WriteString("(/.*)?")

		case rightSlash:
			// ** /b (start)  →  (.*/)?b
			// Matches: b, x/b, x/y/b
			b.WriteString("(.*/)?")
			// Skip leading / from right segment.
			b.WriteString(escapeGlobSegment(segments[i][1:]))

		default:
			// Bare ** without / boundaries  →  .*
			b.WriteString(".*")
			b.WriteString(escaped[i])
		}
	}

	return b.String()
}

func escapeGlobSegment(segment string) string {
	var b strings.Builder
	for _, ch := range segment {
		switch ch {
		case '*':
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		case '.', '+', '(', ')', '{', '}', '[', ']', '^', '$', '|', '\\':
			b.WriteByte('\\')
			b.WriteRune(ch)
		default:
			b.WriteRune(ch)
		}
	}
	return b.String()
}
