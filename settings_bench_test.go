package depguard

import (
	"testing"

	"github.com/OpenPeeDeeP/depguard/v2/internal/utils"
)

// Benchmarks for the matching hot path. importAllowed runs once per import per
// list that applies to a file, so it has to stay fast and allocation free.
// Compare against a baseline with:
//
//	go test -run '^$' -bench . -benchmem -count 10 . > old.txt
//	(make the change)
//	go test -run '^$' -bench . -benchmem -count 10 . > new.txt
//	benchstat old.txt new.txt

// The real $gostd expander, so the benchmarks see a real-sized list. Package
// level variables are initialized before settings_test.go's init() swaps in
// a fake.
var realGoStdExpander = utils.PackageExpandable["$gostd"]

// useRealGoStd makes compile() expand $gostd with the real std list for the
// rest of the benchmark or test, instead of the fake.
func useRealGoStd(tb testing.TB) {
	tb.Helper()
	fake := utils.PackageExpandable["$gostd"]
	utils.PackageExpandable["$gostd"] = realGoStdExpander
	tb.Cleanup(func() { utils.PackageExpandable["$gostd"] = fake })
}

// A strict list shaped like a real config: the real $gostd (about 180 exact
// entries), 20 prefixes and 10 deny entries, some nested under allowed ones.
var benchAllow = []string{
	"$gostd",
	"github.com/org/",
	"github.com/org/legacy/auth/",
	"go.opentelemetry.io/",
	"golang.org/x/",
	"google.golang.org/grpc",
	"google.golang.org/protobuf",
	"github.com/stretchr/testify",
	"github.com/google/go-cmp",
	"github.com/google/uuid",
	"github.com/spf13/cobra",
	"github.com/spf13/viper",
	"github.com/prometheus/client_golang",
	"github.com/rs/zerolog",
	"github.com/jackc/pgx/v5",
	"github.com/redis/go-redis/v9",
	"github.com/aws/aws-sdk-go-v2/",
	"gopkg.in/yaml.v3",
	"k8s.io/apimachinery/",
	"k8s.io/client-go/",
	"sigs.k8s.io/controller-runtime",
}

var benchDeny = map[string]string{
	"math/rand$":                  "use math/rand/v2",
	"io/ioutil$":                  "use io and os",
	"reflect$":                    "avoid reflection",
	"github.com/org/legacy/":      "use github.com/org/v2",
	"github.com/org/legacy/db$":   "use github.com/org/v2/db",
	"github.com/pkg/errors":       "use errors and fmt.Errorf",
	"github.com/sirupsen/logrus":  "use github.com/rs/zerolog",
	"github.com/golang/protobuf":  "use google.golang.org/protobuf",
	"golang.org/x/exp/":           "no experimental packages",
	"k8s.io/client-go/deprecated": "use k8s.io/client-go/kubernetes",
}

// Imports by what they hit, so a regression in one kind of lookup shows up
// on its own.
var benchImports = []struct {
	name    string
	imports []string
}{
	{"std", []string{"fmt", "net/http", "encoding/json", "os", "strings", "context", "sync", "time", "net/http/httptest", "crypto/tls"}},
	{"prefix", []string{
		"github.com/org/app/internal/db", "go.opentelemetry.io/otel/trace", "golang.org/x/sync/errgroup",
		"google.golang.org/grpc/codes", "github.com/stretchr/testify/require", "github.com/aws/aws-sdk-go-v2/service/s3",
		"k8s.io/client-go/kubernetes", "github.com/org/legacy/auth/token",
	}},
	{"deny", []string{
		"math/rand", "io/ioutil", "github.com/org/legacy/api", "github.com/org/legacy/db",
		"github.com/pkg/errors", "golang.org/x/exp/slices", "k8s.io/client-go/deprecated/foo",
	}},
	{"miss", []string{
		"example.com/foo", "gopkg.in/yaml.v2", "github.com/gin-gonic/gin", "bitbucket.org/team/repo",
		"iox/foo", "go.uber.org/zap", "github.com/orgx/app",
	}},
}

func benchList(b *testing.B) *list {
	b.Helper()
	useRealGoStd(b)
	li, err := (&List{ListMode: "strict", Allow: benchAllow, Deny: benchDeny}).compile()
	if err != nil {
		b.Fatalf("compiling bench list: %v", err)
	}
	return li
}

// TestBenchImports keeps the benchmark honest: each group must hit what its
// name says, or a sub-benchmark quietly measures a different path.
func TestBenchImports(t *testing.T) {
	useRealGoStd(t)
	li, err := (&List{ListMode: "strict", Allow: benchAllow, Deny: benchDeny}).compile()
	if err != nil {
		t.Fatalf("compiling bench list: %v", err)
	}
	for _, c := range benchImports {
		for _, imp := range c.imports {
			allowed, sug := li.importAllowed(imp)
			var ok bool
			switch c.name {
			case "std", "prefix":
				ok = allowed
			case "deny":
				ok = !allowed && sug != ""
			case "miss":
				ok = !allowed && sug == ""
			}
			if !ok {
				t.Errorf("%s: import %q: got (%v, %q)", c.name, imp, allowed, sug)
			}
		}
	}
}

var benchSinkAllowed bool
var benchSinkSug string

func BenchmarkImportAllowed(b *testing.B) {
	li := benchList(b)
	var all []string
	for _, c := range benchImports {
		all = append(all, c.imports...)
	}
	run := func(imps []string) func(*testing.B) {
		return func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; b.Loop(); i++ {
				benchSinkAllowed, benchSinkSug = li.importAllowed(imps[i%len(imps)])
			}
		}
	}
	b.Run("mixed", run(all))
	for _, c := range benchImports {
		b.Run(c.name, run(c.imports))
	}
}

// BenchmarkCompile measures building one list, $gostd expansion included.
// It runs once per list per process, so it matters far less than
// importAllowed. The real $gostd list is cached after its first call, as in
// a real run.
func BenchmarkCompile(b *testing.B) {
	useRealGoStd(b)
	l := &List{ListMode: "strict", Allow: benchAllow, Deny: benchDeny}
	if _, err := l.compile(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := l.compile(); err != nil {
			b.Fatal(err)
		}
	}
}
