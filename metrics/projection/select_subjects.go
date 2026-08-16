package projection

import (
	"slices"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
	"github.com/LukasNiessen/ArchUnitGo/common/matching"
	metricsextraction "github.com/LukasNiessen/ArchUnitGo/metrics/extraction"
)

// Subjects are the things one metrics rule is measured over: the files, for a metric about a file, the
// classes, for a metric about a class, and the components, for a metric about a package.
//
// All three populations are carried because a rule's scope is written before its metric is chosen —
// `metrics, in folder "internal/**", for classes matching "*Service"` narrows all of them at once — and a
// metric reads the one it is about. That is what keeps the choice out of every caller: nothing between the
// scope and the number has to branch on what kind of metric was picked.
type Subjects struct {
	// Files are the files a metric about a file is measured over, in the order metrics/extraction read
	// them, which is the sorted order SelectFiles selected them in.
	Files []metricsextraction.FileInfo
	// Classes are the classes a metric about a class is measured over: the ones the class selectors
	// accepted, in the order their files arrived and, inside one file, the order they were declared in.
	Classes []metricsextraction.ClassInfo
	// Components are the packages a metric about a package is measured over: one per folder holding a
	// measured file, sorted by label, as SelectComponents built them.
	Components []Component
}

// SelectSubjects returns what a metrics rule measures, given the graph its scope was resolved against, the
// files that scope selected and read, and the selectors it wrote about classes.
//
// The class selectors are matched against the class identifier — `internal/api.Handler` — which is what
// `for classes matching` means: matching.TargetClassname strips the qualification off first, so the
// pattern is about the bare name while a report still says which package the class was in. They are
// combined with AND, like every scope verb, and no class selector at all is every class these files
// declare.
//
// The files narrow with them, and that is the one decision this function makes. A rule that named classes
// and then asked for a metric about a file is measured over the files declaring one of those classes,
// rather than over every file the folder selectors kept: a selector the user typed has to change what the
// rule is about, and quietly measuring files that hold none of the named classes would be the alternative.
// With no class selectors, every file that was read is measured.
//
// The components follow the files rather than being selected again, which is the same decision read once
// more: a component is the folder of the files that are measured, so a narrowed scope has narrowed
// components and their coupling is the coupling between the files that were kept. SelectComponents is where
// that is spelled out, and the graph is what it needs — the counts are in the files, the dependencies are
// not.
//
// The result holds copies of the slices it was given, because a projection that has been handed to a
// report must not change when the caller reuses its own.
func SelectSubjects(graph extraction.Graph, files []metricsextraction.FileInfo, classSelectors ...matching.Filter) Subjects {
	classes := make([]metricsextraction.ClassInfo, 0, len(files))
	for _, file := range files {
		for _, class := range file.Classes {
			if matchesEvery(class.Identifier, classSelectors) {
				classes = append(classes, class)
			}
		}
	}
	if len(classSelectors) == 0 {
		measured := slices.Clone(files)
		return Subjects{Files: measured, Classes: classes, Components: SelectComponents(graph, measured)}
	}

	declaring := make(map[string]struct{}, len(classes))
	for _, class := range classes {
		declaring[class.Path] = struct{}{}
	}
	measured := make([]metricsextraction.FileInfo, 0, len(declaring))
	for _, file := range files {
		if _, declares := declaring[file.Path]; declares {
			measured = append(measured, file)
		}
	}
	return Subjects{Files: measured, Classes: classes, Components: SelectComponents(graph, measured)}
}
