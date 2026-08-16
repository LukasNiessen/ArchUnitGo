package projection_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
	"github.com/LukasNiessen/ArchUnitGo/slices/projection"
)

func TestSliceByRegexIsTheGlobSpellingInTheSubstrate(t *testing.T) {
	// `internal/([a-z]+)/.*` is `internal/(**)/**` written as a regular expression, and the same slicing has to
	// come out of both: the syntax a user chose is not part of what a rule means.
	mapper, err := projection.SliceByRegex(`internal/([a-z]+)/.*`)
	if err != nil {
		t.Fatalf("SliceByRegex: %v", err)
	}

	projected := kernel.ProjectEdges(fixtureGraph(), mapper)

	want := []string{"api -> db", "db -> api"}
	if dependencies := edgeStrings(projected); !slices.Equal(dependencies, want) {
		t.Errorf("the dependencies between the slices are %v, want %v", dependencies, want)
	}
}

func TestSliceByRegexLeavesGreedinessToTheCaller(t *testing.T) {
	// The expression is taken as written, which is the point of having the substrate available: the glob
	// spelling picks the short capture for you, and a caller who wants the long one asks for it here.
	graph := extraction.NewGraph(
		extraction.SelfEdge("internal/api/rest/handler.go"),
		extraction.SelfEdge("internal/db/sql/conn.go"),
		extraction.NewEdge("internal/api/rest/handler.go", "internal/db/sql/conn.go", false, extraction.ImportKindPlain),
	)

	tests := []struct {
		name       string
		expression string
		want       []string
	}{
		{"a lazy capture names the folder", `internal/(.*?)/.*`, []string{"api -> db"}},
		{"a greedy capture names the path to the file", `internal/(.*)/.*`, []string{"api/rest -> db/sql"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapper, err := projection.SliceByRegex(test.expression)
			if err != nil {
				t.Fatalf("SliceByRegex(%q): %v", test.expression, err)
			}

			projected := kernel.ProjectEdges(graph, mapper)

			if dependencies := edgeStrings(projected); !slices.Equal(dependencies, test.want) {
				t.Errorf("%q sliced the graph into %v, want %v", test.expression, dependencies, test.want)
			}
		})
	}
}

func TestSliceByRegexWantsExactlyOneCapture(t *testing.T) {
	tests := []struct {
		name       string
		expression string
		wantErr    error
	}{
		{"no capture", `internal/[a-z]+/.*`, matching.ErrOneCapture},
		{"a non-capturing group does not count", `internal/(?:[a-z]+)/.*`, matching.ErrOneCapture},
		{"two captures", `internal/([a-z]+)/([a-z]+)\.go`, matching.ErrOneCapture},
		{"an expression that does not compile", `internal/([a-z]+/.*`, matching.ErrInvalidPattern},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mapper, err := projection.SliceByRegex(test.expression)

			if !errors.Is(err, test.wantErr) {
				t.Errorf("SliceByRegex(%q) error = %v, want %v", test.expression, err, test.wantErr)
			}
			if mapper != nil {
				t.Errorf("SliceByRegex(%q) returned a mapper beside the error, want none", test.expression)
			}
		})
	}
}
