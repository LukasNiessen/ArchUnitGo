package projection

import (
	"slices"
	"strings"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
	metricsextraction "github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
)

// Component is one package of the project as a distance metric sees it: the folder it is, how many types
// it declares and how many of those are interfaces, and which other components it depends on and is
// depended on by.
//
// It is the third population a metrics rule can be measured over, beside the files and the classes, and
// the subject of every metric that is about a package rather than about one file — abstractness,
// instability, the two distances and the coupling factor — because all five of them are ratios between
// numbers no single file has. A component is a folder rather than a Go import path for the reason the
// whole library deals in identifiers: an identifier is what the user's own patterns were written against,
// and the folder is what `in folder "internal/**"` already selects.
//
// Every field is a fact about the package rather than a formula over it. Which ratio a rule is about is
// metrics/calculation's business, so abstractness and instability are not in here — that is the same split
// FileInfo makes, and it is what lets the arithmetic be tested against a Component literal with no
// project on disk.
type Component struct {
	// Label is the folder as an identifier — `internal/api`, and `.` for the project root. It is
	// metricsextraction.FileInfo.Directory of every file the component is made of, and the string a
	// measurement about a component names as its subject.
	Label string
	// Classes is how many types the component's selected files declare, structs, interfaces and names
	// given to other types alike. It is the denominator of abstractness, and 0 for a package of nothing
	// but functions.
	Classes int
	// Interfaces is how many of those declared types are interfaces. It is the numerator of abstractness:
	// Go has no abstract class, and an interface is the declaration that says what a package promises
	// without saying how it keeps it.
	Interfaces int
	// DependsOn are the labels of the components this one depends on, sorted and each of them once. Its
	// length is Ce, the efferent coupling, and it is empty for a component that depends on none of the
	// others.
	DependsOn []string
	// DependedOnBy are the labels of the components that depend on this one, sorted and each of them once.
	// Its length is Ca, the afferent coupling, and it is empty for a component nothing else reaches for.
	DependedOnBy []string
}

// SelectComponents returns the packages a metrics rule is measured over: one Component per folder holding
// a file the rule selected, sorted by label, with the counts read off those files and the coupling read
// off the graph.
//
// The two halves come from the two things a rule has in hand. How many types a package declares is in the
// files metrics/extraction read, summed per folder; which components depend on which is in the graph,
// projected through PerComponentEdge — a folder pair per dependency between two selected files. That is
// also why a component appears here with no coupling at all rather than not appearing: a package the
// scope selected is a package the rule is about, and a package nothing imports and that imports nothing is
// the most stable, most concrete component there is.
//
// Only the selected files count, on both halves. A component of a narrowed scope is the part of that
// package the rule selected — its numbers are about the files that were read, and its coupling about the
// dependencies between them — which is PerComponentEdge's trade and metrics/extraction's, stated once
// more here because it is the answer to "why is this package's instability 0 when it clearly imports
// something": the thing it imports was not selected.
//
// No files at all is no components and no error. Whether a rule that selected nothing is a failure is the
// empty-test guard's question, and a terminal asks it before anything is judged.
func SelectComponents(graph extraction.Graph, files []metricsextraction.FileInfo) []Component {
	counted := make(map[string]Component, len(files))
	selected := make([]string, 0, len(files))
	for _, file := range files {
		component := counted[file.Directory]
		component.Label = file.Directory
		component.Classes += len(file.Classes)
		component.Interfaces += interfacesIn(file)
		counted[file.Directory] = component
		selected = append(selected, file.Path)
	}

	// Both ends of a projected edge are the folder of a selected file, so both are components already
	// counted above, and the coupling is written down from both sides at once — Ce of the one end is Ca of
	// the other, and reading it off one projection is what stops the two from disagreeing.
	for _, edge := range kernel.ProjectEdges(graph, PerComponentEdge(selected)) {
		depending, dependedOn := counted[edge.SourceLabel()], counted[edge.TargetLabel()]
		depending.DependsOn = append(depending.DependsOn, edge.TargetLabel())
		dependedOn.DependedOnBy = append(dependedOn.DependedOnBy, edge.SourceLabel())
		counted[edge.SourceLabel()] = depending
		counted[edge.TargetLabel()] = dependedOn
	}

	components := make([]Component, 0, len(counted))
	for _, component := range counted {
		// Sorted here even though ProjectEdges already hands its edges over ordered, so that the order this
		// type promises is the order this function established: a neighbor list is sorted because it was
		// sorted, and not because of what a function two layers down happens to do today.
		slices.Sort(component.DependsOn)
		slices.Sort(component.DependedOnBy)
		components = append(components, component)
	}
	// The map above is what sums a folder's files, so the order has to be re-established: a report of the
	// numbers of a project must not come out shuffled between two runs.
	slices.SortFunc(components, func(left, right Component) int {
		return strings.Compare(left.Label, right.Label)
	})
	return components
}

// interfacesIn counts the interfaces among the types one file declares. It is the per-file half of
// Component.Interfaces, and it is a loop rather than a field because being an interface is a fact about
// each declared type.
func interfacesIn(file metricsextraction.FileInfo) int {
	count := 0
	for _, class := range file.Classes {
		if class.Interface {
			count++
		}
	}
	return count
}
