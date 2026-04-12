package perms

import (
	"strconv"
	"sync"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		// ** (globstar)
		{"double star matches nested", "etc/**", "etc/foo/bar/baz.conf", true},
		{"double star matches direct child", "etc/**", "etc/pacman.conf", true},
		{"double star no match outside", "etc/**", "usr/lib/foo", false},
		{"double star no match prefix", "etc/**", "etcfoo", false},
		{"double star at start", "**/*.conf", "etc/foo/bar.conf", true},
		{"double star at start direct", "**/*.conf", "bar.conf", true},
		{"double star middle", "etc/**/rules.d/**", "etc/polkit-1/rules.d/99-foo.rules", true},
		{"double star only matches deep", "**", "anything/at/all", true},
		{"double star only matches single", "**", "file", true},
		{"double star zero components", "etc/**/foo", "etc/foo", true},
		{"double star one component", "etc/**/foo", "etc/bar/foo", true},
		{"double star many components", "etc/**/foo", "etc/a/b/c/foo", true},

		// * (single star)
		{"single star matches filename", "etc/*", "etc/pacman.conf", true},
		{"single star no nested", "etc/*", "etc/foo/bar.conf", false},
		{"single star middle", "etc/*/conf.d", "etc/fontconfig/conf.d", true},

		// ?
		{"question mark single char", "etc/?.conf", "etc/a.conf", true},
		{"question mark no multi char", "etc/?.conf", "etc/ab.conf", false},

		// Literal
		{"exact match", "etc/pacman.conf", "etc/pacman.conf", true},
		{"exact no match suffix", "etc/pacman.conf", "etc/pacman.conf.bak", false},

		// Regex metacharacter escaping
		{"dot is literal", "etc/foo.conf", "etc/foo.conf", true},
		{"dot not wildcard", "etc/foo.conf", "etc/fooXconf", false},
		{"brackets literal", "etc/[foo]", "etc/[foo]", true},
		{"brackets not charset", "etc/[foo]", "etc/f", false},
		{"plus literal", "etc/foo+bar", "etc/foo+bar", true},
		{"parentheses literal", "etc/(foo)", "etc/(foo)", true},
		{"parentheses not group", "etc/(foo)", "etc/foo", false},

		// Edge cases
		{"empty pattern empty path", "", "", true},
		{"empty pattern nonempty path", "", "foo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchGlob(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("MatchGlob(%q, %q) = %v, want %v",
					tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

func TestGlobCacheEviction(t *testing.T) {
	// Fill cache beyond max size to trigger eviction.
	for i := 0; i < globCacheMaxSize+10; i++ {
		pattern := "**/pattern" + strconv.Itoa(i)
		MatchGlob(pattern, "some/path")
	}

	globCacheMu.RLock()
	size := len(globCache)
	globCacheMu.RUnlock()

	if size > globCacheMaxSize {
		t.Errorf("cache size %d exceeds max %d after eviction", size, globCacheMaxSize)
	}
	if size == 0 {
		t.Error("cache should not be completely empty after eviction")
	}
}

func TestGlobCacheDoubleCheck(t *testing.T) {
	MatchGlob("TestGlobCacheDoubleCheck/**", "TestGlobCacheDoubleCheck/path")
	MatchGlob("TestGlobCacheDoubleCheck/**", "TestGlobCacheDoubleCheck/path")

	globCacheMu.RLock()
	_, ok := globCache["TestGlobCacheDoubleCheck/**"]
	globCacheMu.RUnlock()

	if !ok {
		t.Error("pattern should be in cache")
	}
}

func TestGlobCacheConcurrency(t *testing.T) {
	// Test that concurrent access doesn't cause panic or data race.
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			pattern := "**/concurrent" + strconv.Itoa(n)
			MatchGlob(pattern, "test/path")
		}(i)
	}
	wg.Wait()
}
