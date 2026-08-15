package archtest_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/archtest"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/assertion"
	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/common/projection/cycles"
	filesassertion "github.com/LukasNiessen/ArchUnitGo/files/assertion"
	layersassertion "github.com/LukasNiessen/ArchUnitGo/layers/assertion"
)

// theEmptyTestHint is the note a report adds to a rule that selected nothing, written out here rather than
// read from the package: the wording is the deliverable, so a test that took it from the code it tests
// would agree with any rewording of it.
const theEmptyTestHint = "an empty rule would hold forever, so selecting nothing is a violation rather " +
	"than a pass (AllowEmptyTests opts out)"

func TestEveryViolationTheLibraryReportsIsPhrasedFromItsOwnData(t *testing.T) {
	// The whole message policy of the library, pinned one violation at a time: the subject that disagreed
	// with the rule, the requirement it broke in the words the rule was written in, and what was found
	// instead. Exact strings, because the phrasing is what this layer exists to decide.
	for _, wanted := range []struct {
		name      string
		violation kernel.Violation
		message   string
	}{
		{
			name:      "a name a file should have and does not",
			violation: filesassertion.NewNamingViolation("internal/db/conn.go", filenameMatcher(t, "*_service.go"), kernel.Should),
			message:   `internal/db/conn.go: should, filename matches "*_service.go"; it does not`,
		},
		{
			name:      "a name a file should not have and does",
			violation: filesassertion.NewNamingViolation("internal/db/conn_test.go", filenameMatcher(t, "*_test.go"), kernel.ShouldNot),
			message:   `internal/db/conn_test.go: should not, filename matches "*_test.go"; it does`,
		},
		{
			name:      "a folder a file should be in and is not",
			violation: filesassertion.NewNamingViolation("internal/db/conn.go", folderMatcher(t, "internal/api/**"), kernel.Should),
			message:   `internal/db/conn.go: should, path without filename matches "internal/api/**"; it does not`,
		},
		{
			name:      "a predicate of the user's own that a file does not satisfy",
			violation: filesassertion.NewAdherenceViolation("internal/db/conn.go", "be at most 400 lines long", kernel.Should),
			message:   `internal/db/conn.go: should, adhere to "be at most 400 lines long"; it does not`,
		},
		{
			name:      "a predicate of the user's own that a file satisfies where the rule forbade it",
			violation: filesassertion.NewAdherenceViolation("internal/db/conn.go", "import the filesystem", kernel.ShouldNot),
			message:   `internal/db/conn.go: should not, adhere to "import the filesystem"; it does`,
		},
		{
			name: "a dependency a file should not have and does",
			violation: filesassertion.NewDependencyViolation(
				"internal/api/handler.go",
				[]matching.Filter{folderMatcher(t, "internal/db/**")},
				[]string{"internal/db/tx.go", "internal/db/conn.go"},
				kernel.ShouldNot,
			),
			// The dependencies sorted, as the violation itself sorted them, so that a report of a
			// hand-built violation reads like one of a projected graph.
			message: `internal/api/handler.go: should not, depend on files, path without filename matches "internal/db/**"; ` +
				`it depends on internal/db/conn.go, internal/db/tx.go`,
		},
		{
			name: "a dependency a file should have and has on none of them",
			violation: filesassertion.NewDependencyViolation(
				"internal/api/handler.go",
				[]matching.Filter{folderMatcher(t, "internal/db/**"), filenameMatcher(t, "*.go")},
				nil,
				kernel.Should,
			),
			// Both object selectors, in the order they were chained, because either of them can be the one
			// the reader has to go and fix.
			message: `internal/api/handler.go: should, depend on files, path without filename matches "internal/db/**", ` +
				`filename matches "*.go"; it depends on none of them`,
		},
		{
			name: "a dependency on any file at all",
			violation: filesassertion.NewDependencyViolation(
				"internal/api/handler.go", nil, []string{"internal/db/conn.go"}, kernel.ShouldNot,
			),
			message: `internal/api/handler.go: should not, depend on files; it depends on internal/db/conn.go`,
		},
		{
			name: "an external module a file should not depend on and does",
			violation: filesassertion.NewExternalDependencyViolation(
				"internal/domain/order.go",
				[]matching.Filter{pathMatcher(t, "*.*/**")},
				[]string{"gorm.io/gorm", "github.com/gin-gonic/gin"},
				kernel.ShouldNot,
			),
			// The import paths sorted, as the violation itself sorted them, and each spelled as the file wrote
			// it: a package of a module, not the module it was published as.
			message: `internal/domain/order.go: should not, depend on external modules, path matches "*.*/**"; ` +
				`it depends on github.com/gin-gonic/gin, gorm.io/gorm`,
		},
		{
			name: "several external modules any one of which would do, and none of them does",
			violation: filesassertion.NewExternalDependencyViolation(
				"internal/adapter/store.go",
				[]matching.Filter{pathMatcher(t, "gorm.io/**"), pathMatcher(t, "github.com/lib/pq")},
				nil,
				kernel.Should,
			),
			// Joined with `or`, where the object of `depend on files` is joined with a comma: these are
			// alternatives, and a report that spelled them as one requirement would name a module that
			// cannot exist.
			message: `internal/adapter/store.go: should, depend on external modules, path matches "gorm.io/**" or ` +
				`path matches "github.com/lib/pq"; it depends on none of them`,
		},
		{
			name: "a dependency on anything outside the project at all",
			violation: filesassertion.NewExternalDependencyViolation(
				"internal/domain/order.go", nil, []string{"net/http"}, kernel.ShouldNot,
			),
			// The standard library is external too, because what leaves the project was decided in extraction
			// and this layer does not re-decide it.
			message: `internal/domain/order.go: should not, depend on external modules; it depends on net/http`,
		},
		{
			name:      "a rule that selected nothing",
			violation: kernel.NewEmptyTestViolation("files", folderMatcher(t, "internal/renamed/**")),
			message:   `no files matched: path without filename matches "internal/renamed/**"; ` + theEmptyTestHint,
		},
		{
			name:      "the object half of a relational rule that selected nothing",
			violation: kernel.NewEmptyTestViolation("files to depend on", folderMatcher(t, "internal/renamed/**")),
			message: `no files to depend on matched: path without filename matches "internal/renamed/**"; ` +
				theEmptyTestHint,
		},
		{
			name:      "a rule that selected nothing and says nothing about what",
			violation: kernel.NewEmptyTestViolation(""),
			message:   "nothing matched; " + theEmptyTestHint,
		},
		{
			name:      "a cycle between two files",
			violation: filesassertion.NewCycleViolation(cycle(t, "internal/api/handler.go", "internal/db/conn.go")),
			message: "internal/api/handler.go: should, have no cycles; it depends on itself through " +
				"internal/api/handler.go -> internal/db/conn.go -> internal/api/handler.go",
		},
		{
			name: "a cycle between three files",
			violation: filesassertion.NewCycleViolation(
				cycle(t, "internal/api/handler.go", "internal/db/conn.go", "internal/db/query.go"),
			),
			message: "internal/api/handler.go: should, have no cycles; it depends on itself through " +
				"internal/api/handler.go -> internal/db/conn.go -> internal/db/query.go -> internal/api/handler.go",
		},
		{
			// Not something the enumeration can produce, and a report of it still names no file that is
			// not there.
			name:      "a cycle built by hand out of nothing",
			violation: filesassertion.CycleViolation{},
			message:   "a cycle through no files: should, have no cycles",
		},
		{
			name: "a layer that may not depend on another and does",
			violation: layersassertion.NewDependencyViolation(
				layersassertion.NewClause("db", []string{"api"}, kernel.ShouldNot), "api",
				extraction.NewEdge("internal/db/conn.go", "internal/api/router.go", false, extraction.ImportKindPlain),
			),
			// The mood is `may not` rather than `should not`: a layer policy is the one family whose user
			// spells the mood as part of the predicate. The pair of layers is the offense and the files are
			// where to look, in that order.
			message: `layer "db": may not depend on layers "api"; ` +
				"it depends on api through internal/db/conn.go -> internal/api/router.go",
		},
		{
			name: "a layer that depends on one an allowlist did not name",
			violation: layersassertion.NewDependencyViolation(
				layersassertion.NewClause("api", []string{"domain", "db"}, kernel.Should), "transport",
				extraction.NewEdge("internal/api/handler.go", "internal/transport/http.go", false, extraction.ImportKindPlain),
				extraction.NewEdge("internal/api/router.go", "internal/transport/http.go", false, extraction.ImportKindPlain),
			),
			// Every layer the clause named, in the order the user typed them, because any of them may be the
			// one they meant to write differently — and every file dependency, because a layer policy fails
			// per pair of layers and all of them have to be unpicked.
			message: `layer "api": may only depend on layers "domain", "db"; it depends on transport through ` +
				"internal/api/handler.go -> internal/transport/http.go, " +
				"internal/api/router.go -> internal/transport/http.go",
		},
		{
			name: "a sealed layer that depends on another at all",
			violation: layersassertion.NewDependencyViolation(
				layersassertion.NewClause("domain", nil, kernel.Should), "db",
				extraction.NewEdge("internal/domain/order.go", "internal/db/conn.go", false, extraction.ImportKindPlain),
			),
			// `may only depend on layers` with nothing named reads as `no layers`, which is the one reading of
			// the empty list that is still English.
			message: `layer "domain": may only depend on no layers; it depends on db through ` +
				"internal/domain/order.go -> internal/db/conn.go",
		},
		{
			// A violation built by hand may carry no dependency, and a report of it then names the pair of
			// layers alone rather than a file that is not there.
			name: "a layer dependency built by hand out of no files",
			violation: layersassertion.NewDependencyViolation(
				layersassertion.NewClause("db", []string{"api"}, kernel.ShouldNot), "api"),
			message: `layer "db": may not depend on layers "api"; it depends on api`,
		},
	} {
		t.Run(wanted.name, func(t *testing.T) {
			message := archtest.NewViolationFactory(nil).Message(wanted.violation)

			if message != wanted.message {
				t.Errorf("a %s violation reads\n\t%s\nwant\n\t%s", wanted.violation.Kind(), message, wanted.message)
			}
		})
	}
}

func TestEveryKindOfViolationTheLibraryDeclaresHasAPhrasingOfItsOwn(t *testing.T) {
	// Step 8 of AGENTS.md's "Adding a new rule" — teach the violation factory how to phrase it — held to
	// mechanically for the kinds that exist today: a family the factory has not been taught falls back to
	// its own String, which is a message but not a phrasing, and this is what notices.
	phrased := map[kernel.ViolationKind]kernel.Violation{
		kernel.KindEmptyTest:              kernel.NewEmptyTestViolation("files", folderMatcher(t, "internal/**")),
		filesassertion.KindFileCycle:      filesassertion.NewCycleViolation(cycle(t, "a.go", "b.go")),
		filesassertion.KindFileNaming:     filesassertion.NewNamingViolation("a.go", filenameMatcher(t, "*.go"), kernel.Should),
		filesassertion.KindFileDependency: filesassertion.NewDependencyViolation("a.go", nil, nil, kernel.Should),
		filesassertion.KindFileAdherence:  filesassertion.NewAdherenceViolation("a.go", "be short", kernel.Should),
		filesassertion.KindFileExternalDependency: filesassertion.NewExternalDependencyViolation(
			"a.go", nil, nil, kernel.Should),
		layersassertion.KindLayerDependency: layersassertion.NewDependencyViolation(
			layersassertion.NewClause("api", []string{"domain"}, kernel.Should), "db"),
	}

	for kind, violation := range phrased {
		if violation.Kind() != kind {
			t.Errorf("the fixture for %q reports kind %q, want them to agree", kind, violation.Kind())
		}
		message := archtest.NewViolationFactory(nil).Message(violation)
		if strings.Contains(message, "has not been taught") {
			t.Errorf("a %q violation reads %q, want a phrasing of its own", kind, message)
		}
		if strings.HasPrefix(message, string(kind)) {
			t.Errorf("a %q violation reads %q, want the offender named rather than the kind", kind, message)
		}
	}
}

func TestAViolationTheFactoryHasNotBeenTaughtStillReportsWhatItCan(t *testing.T) {
	// A rule family in a module written later, or a Checkable of a user's own: the report says what kind of
	// violation it is and lets the violation say the rest of it. Losing the wording is acceptable; losing
	// the information is not.
	message := archtest.NewViolationFactory(nil).Message(describedViolation{})

	if message != "custom-kind: the whole story, told by the violation itself" {
		t.Errorf("an unknown violation reads %q, want its kind and its own words", message)
	}
}

func TestAViolationThatCannotDescribeItselfSaysWhatIsMissing(t *testing.T) {
	// The only thing left to report is that the phrasing is outstanding — and naming what has to be taught
	// is the only reminder a report can give.
	message := archtest.NewViolationFactory(nil).Message(silentViolation{})

	if !strings.HasPrefix(message, "custom-kind: ") {
		t.Errorf("an unknown violation reads %q, want its kind first", message)
	}
	if !strings.Contains(message, "archtest.ViolationFactory") {
		t.Errorf("an unknown violation reads %q, want it to name what has to be taught", message)
	}
}

func TestAViolationWithNoKindAtAllIsStillReported(t *testing.T) {
	message := archtest.NewViolationFactory(nil).Message(namelessViolation{})

	if !strings.HasPrefix(message, "unknown: ") {
		t.Errorf("a violation of no kind reads %q, want it called unknown", message)
	}
}

func TestANilViolationIsReportedRatherThanCrashingTheHostTest(t *testing.T) {
	// A nil in a []Violation is a bug in whatever built the list. This layer describes somebody else's
	// failing test, and taking their test process down while doing it is the one outcome worse than a
	// vague message.
	message := archtest.NewViolationFactory(nil).Message(nil)

	if message != "(no violation)" {
		t.Errorf("a nil violation reads %q, want it said plainly", message)
	}
}

func TestMessagesPhrasesEveryViolationInTheOrderTheRuleFoundThem(t *testing.T) {
	violations := []kernel.Violation{
		filesassertion.NewNamingViolation("a.go", filenameMatcher(t, "*_test.go"), kernel.Should),
		filesassertion.NewNamingViolation("b.go", filenameMatcher(t, "*_test.go"), kernel.Should),
	}

	messages := archtest.NewViolationFactory(nil).Messages(violations)

	want := []string{
		`a.go: should, filename matches "*_test.go"; it does not`,
		`b.go: should, filename matches "*_test.go"; it does not`,
	}
	if !slices.Equal(messages, want) {
		t.Errorf("the messages are %q, want %q", messages, want)
	}
}

func TestMessagesOfNoViolationsIsNoMessages(t *testing.T) {
	// An empty result is the pass, and how a pass reads is the result factory's to say rather than a list
	// of one message claiming there was nothing to report.
	for _, violations := range [][]kernel.Violation{nil, {}} {
		if messages := archtest.NewViolationFactory(nil).Messages(violations); len(messages) != 0 {
			t.Errorf("the messages of %v are %q, want none", violations, messages)
		}
	}
}

func TestAColoredMessageIsThePlainOneWithTheRolesPainted(t *testing.T) {
	// Color is decoration and never content: a report read from a terminal and the same report read from a
	// CI log have to say the same thing, so stripping the escape sequences has to give the plain message
	// back exactly.
	violation := filesassertion.NewDependencyViolation(
		"internal/api/handler.go",
		[]matching.Filter{folderMatcher(t, "internal/db/**")},
		[]string{"internal/db/conn.go"},
		kernel.ShouldNot,
	)
	options := &archtest.MessageOptions{Palette: archtest.DefaultPalette()}

	colored := archtest.NewViolationFactory(options).Message(violation)
	plain := archtest.NewViolationFactory(nil).Message(violation)

	if colored == plain {
		t.Errorf("the colored message is %q, want it painted", colored)
	}
	if stripped := plainText(colored); stripped != plain {
		t.Errorf("the colored message strips to\n\t%s\nwant\n\t%s", stripped, plain)
	}
	// The roles the palette names, each painted in the color it was given: the file to open in cyan, the
	// rule it broke in yellow, what it actually does in red.
	for role, painted := range map[string]string{
		"subject":     "\x1b[36minternal/api/handler.go\x1b[0m",
		"requirement": "\x1b[33mshould not, depend on files, path without filename matches \"internal/db/**\"\x1b[0m",
		"finding":     "\x1b[31mit depends on internal/db/conn.go\x1b[0m",
	} {
		if !strings.Contains(colored, painted) {
			t.Errorf("the colored message does not paint the %s: %q", role, colored)
		}
	}
}

func TestTheHintOfAnEmptyTestIsPaintedApartFromTheRestOfIt(t *testing.T) {
	// The note that explains the library rather than the code under it is the quietest thing in the
	// report, because a reader who already knows it should be able to skip it.
	violation := kernel.NewEmptyTestViolation("files", folderMatcher(t, "internal/renamed/**"))
	options := &archtest.MessageOptions{Palette: archtest.DefaultPalette()}

	colored := archtest.NewViolationFactory(options).Message(violation)

	if !strings.Contains(colored, "\x1b[90m"+theEmptyTestHint+"\x1b[0m") {
		t.Errorf("the empty-test message is %q, want the hint painted in the hint color", colored)
	}
	if plainText(colored) != `no files matched: path without filename matches "internal/renamed/**"; `+theEmptyTestHint {
		t.Errorf("the empty-test message strips to %q, want the plain one", plainText(colored))
	}
}

// describedViolation is a violation of a family this layer has not been taught, of the kind every violation
// in the library is: one that renders itself for a log line.
type describedViolation struct{}

func (describedViolation) Kind() kernel.ViolationKind {
	return "custom-kind"
}

func (describedViolation) String() string {
	return "the whole story, told by the violation itself"
}

// silentViolation is the same, without even that: the least a Violation can be, which is what the interface
// actually requires.
type silentViolation struct{}

func (silentViolation) Kind() kernel.ViolationKind {
	return "custom-kind"
}

// namelessViolation is a violation whose kind was never declared — a zero ViolationKind, which is what a
// forgotten constant looks like.
type namelessViolation struct{}

func (namelessViolation) Kind() kernel.ViolationKind {
	return ""
}

// filenameMatcher and folderMatcher are the selectors a rule's scope is built from, compiled the one way
// the library compiles a pattern: through a RegexFactory, in the glob syntax a user types.
func filenameMatcher(t *testing.T, pattern string) matching.Filter {
	t.Helper()

	selector, err := matching.NewRegexFactory(nil).FilenameMatcher(pattern)
	if err != nil {
		t.Fatalf("compiling the filename pattern %q failed: %v", pattern, err)
	}
	return selector
}

func folderMatcher(t *testing.T, pattern string) matching.Filter {
	t.Helper()

	selector, err := matching.NewRegexFactory(nil).FolderMatcher(pattern)
	if err != nil {
		t.Fatalf("compiling the folder pattern %q failed: %v", pattern, err)
	}
	return selector
}

func pathMatcher(t *testing.T, pattern string) matching.Filter {
	t.Helper()

	selector, err := matching.NewRegexFactory(nil).PathMatcher(pattern)
	if err != nil {
		t.Fatalf("compiling the path pattern %q failed: %v", pattern, err)
	}
	return selector
}

// cycle is the circular chain of dependencies between these files, in the order they are given, closing
// from the last one back onto the first. It goes through the enumeration because Circuit has no exported
// constructor, on purpose: a cycle is something a projection found.
func cycle(t *testing.T, files ...string) cycles.Circuit {
	t.Helper()

	edges := make([]projection.ProjectedEdge, 0, len(files))
	for index, file := range files {
		next := files[(index+1)%len(files)]
		edges = append(edges, projection.NewProjectedEdge(file, next, extraction.NewEdge(file, next, false, extraction.ImportKindPlain)))
	}
	circuits, complete := cycles.ProjectCircuits(edges, nil)
	if !complete || len(circuits) != 1 {
		t.Fatalf("the fixture %v has %d cycles (complete: %t), want exactly one", files, len(circuits), complete)
	}
	return circuits[0]
}

// ansiEscape matches one ANSI escape sequence, which is everything Color.Paint ever emits.
var ansiEscape = regexp.MustCompile("\x1b\\[[0-9;]*m")

// plainText is a colored message with the color taken back out, which is how a test says "the same report,
// read from a CI log".
func plainText(message string) string {
	return ansiEscape.ReplaceAllString(message, "")
}
