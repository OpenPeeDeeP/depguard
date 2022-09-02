package depguard

import (
	"os"
	"sort"
	"testing"

	"github.com/gobwas/glob"
)

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

var (
	prefixList = []string{
		"some/package/a",
		"some/package/b",
		"some/package/c/",
		"some/pkg/c",
		"some/pkg/d",
		"some/pkg/e",
	}

	globList = []glob.Glob{
		glob.MustCompile("some/*/a", '/'),
		glob.MustCompile("some/**/a", '/'),
	}
)

func testStrInPrefixList(str string, expect bool) func(t *testing.T) {
	return func(t *testing.T) {
		if strInPrefixList(str, prefixList) != expect {
			t.Fail()
		}
	}
}

func TestStrInPrefixList(t *testing.T) {
	sort.Strings(prefixList)
	t.Run("full_match_start", testStrInPrefixList("some/package/a", true))
	t.Run("full_match", testStrInPrefixList("some/package/b", true))
	t.Run("full_match_end", testStrInPrefixList("some/pkg/e", true))
	t.Run("no_match_end", testStrInPrefixList("zome/pkg/e", false))
	t.Run("no_match_start", testStrInPrefixList("aome/pkg/e", false))
	t.Run("match_start", testStrInPrefixList("some/package/a/files", true))
	t.Run("match_middle", testStrInPrefixList("some/pkg/c/files", true))
	t.Run("match_end", testStrInPrefixList("some/pkg/e/files", true))
	t.Run("no_match_trailing", testStrInPrefixList("some/package/c", false))
}

func testStrInGlobList(str string, expect bool) func(t *testing.T) {
	return func(t *testing.T) {
		if strInGlobList(str, globList) != expect {
			t.Fail()
		}
	}
}

func TestStrInGlobList(t *testing.T) {
	t.Run("match_first", testStrInGlobList("some/foo/a", true))
	t.Run("match", testStrInGlobList("some/foo/bar/a", true))
	t.Run("no_match", testStrInGlobList("some/foo/b", false))
}
