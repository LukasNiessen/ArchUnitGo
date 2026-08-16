package projection_test

import (
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	"github.com/LukasNiessen/ArchUnitGo/graph/projection"
)

func TestSnapshotOptionsWithDefaultsTurnsANilQueryIntoTheDefaultReport(t *testing.T) {
	// The options-bag contract: a nil bag, the zero bag and an explicitly empty one all describe the same
	// report — the project's own files, one node per file, nothing collapsed, no title.
	var absent *projection.SnapshotOptions

	resolved := absent.WithDefaults()

	if resolved.IncludeExternalDependencies || resolved.IncludeSelfDependencies {
		t.Errorf("the default query is %+v, want the project's own code and no self dependency", resolved)
	}
	if resolved.CollapseToFolderDepth != 0 || resolved.Title != "" {
		t.Errorf("the default query is %+v, want nothing collapsed and no title", resolved)
	}
	if len(resolved.Focus)+len(resolved.ReachableFrom)+len(resolved.DependentsOf)+len(resolved.CollapseGroups) != 0 {
		t.Errorf("the default query is %+v, want no node filtered out of the report", resolved)
	}
}

func TestSnapshotOptionsWithDefaultsCopiesTheSlicesItResolves(t *testing.T) {
	// A struct copy shares a slice's backing array, so without this a builder appending a second `focus on` to
	// its resolved query would reach into the query a stored half-built report shares.
	query := projection.SnapshotOptions{
		Focus:          make([]projection.Focus, 1, 4),
		ReachableFrom:  make([]matching.Filter, 1, 4),
		DependentsOf:   make([]matching.Filter, 1, 4),
		CollapseGroups: make([]projection.CollapseGroup, 1, 4),
	}
	query.Focus[0] = projection.Focus{Selector: pathMatcher(t, "internal/api/**")}
	query.CollapseGroups[0] = projection.CollapseGroup{Label: "api", Selector: pathMatcher(t, "internal/api/**")}
	// The spare capacity is what makes a shared array visible: two queries resolved from this one and appended
	// to would both land in index 1, and the second would write the first's element out of it.
	db, cmd := pathMatcher(t, "internal/db/**"), pathMatcher(t, "cmd/**")

	first, second := query.WithDefaults(), query.WithDefaults()
	first.Focus = append(first.Focus, projection.Focus{Selector: db})
	first.ReachableFrom = append(first.ReachableFrom, db)
	first.DependentsOf = append(first.DependentsOf, db)
	first.CollapseGroups = append(first.CollapseGroups, projection.CollapseGroup{Label: "db", Selector: db})
	second.Focus = append(second.Focus, projection.Focus{Selector: cmd})
	second.ReachableFrom = append(second.ReachableFrom, cmd)
	second.DependentsOf = append(second.DependentsOf, cmd)
	second.CollapseGroups = append(second.CollapseGroups, projection.CollapseGroup{Label: "cmd", Selector: cmd})

	// Each resolved query still sees what it appended itself, and not what the other one did.
	if got := first.Focus[1].Selector.String(); got != db.String() {
		t.Errorf("the first query's second focus is on %v, want the %v it was given", got, db)
	}
	if got := first.ReachableFrom[1].String(); got != db.String() {
		t.Errorf("the first query reaches from %v, want the %v it was given", got, db)
	}
	if got := first.DependentsOf[1].String(); got != db.String() {
		t.Errorf("the first query depends on %v, want the %v it was given", got, db)
	}
	if got := first.CollapseGroups[1].Label; got != "db" {
		t.Errorf("the first query's second group is %q, want the %q it was given", got, "db")
	}
	if got := second.Focus[1].Selector.String(); got != cmd.String() {
		t.Errorf("the second query's second focus is on %v, want the %v it was given", got, cmd)
	}
	// And the query the two were resolved from is the one it was described as.
	if len(query.Focus) != 1 || len(query.ReachableFrom) != 1 || len(query.DependentsOf) != 1 || len(query.CollapseGroups) != 1 {
		t.Errorf("appending to a resolved query changed the original to %+v", query)
	}
	if query.Focus[0].String() != first.Focus[0].String() {
		t.Errorf("the resolved query's first focus is %v, want the original's %v", first.Focus[0], query.Focus[0])
	}
}

func TestFocusStringSaysWhatItIsCenteredOnAndHowFarOut(t *testing.T) {
	// How a builder prints itself, and a hop is a hop rather than `hops` in the singular.
	focus := projection.Focus{Selector: pathMatcher(t, "internal/api/**"), Depth: 2}

	want := `path matches "internal/api/**" within 2 hops`
	if got := focus.String(); got != want {
		t.Errorf("the focus renders as %q, want %q", got, want)
	}
	one := projection.Focus{Selector: pathMatcher(t, "main.go"), Depth: 1}
	if got, want := one.String(), `path matches "main.go" within 1 hop`; got != want {
		t.Errorf("the focus renders as %q, want %q", got, want)
	}
}

func TestFocusStringRendersANegativeDepthAsNoHopsAtAll(t *testing.T) {
	// What the traversal is given, so that a report of a mistyped chain reads as what it will actually draw.
	focus := projection.Focus{Selector: pathMatcher(t, "main.go"), Depth: -2}

	want := `path matches "main.go" within 0 hops`
	if got := focus.String(); got != want {
		t.Errorf("the focus renders as %q, want %q", got, want)
	}
}

func TestCollapseGroupStringNamesTheGroupAndThePatternThatFillsIt(t *testing.T) {
	group := projection.CollapseGroup{Label: "third party", Selector: pathMatcher(t, "github.com/**")}

	want := `"third party" by path matches "github.com/**"`
	if got := group.String(); got != want {
		t.Errorf("the group renders as %q, want %q", got, want)
	}
}
