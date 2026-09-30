package logging

import (
	"fmt"
	"maps"
	"slices"
	"strconv"

	"github.com/LukasNiessen/ArchUnitGo/common/extraction"
)

// LogGraph records every discovered file and dependency at debug level, including
// external targets and import kinds. It does not re-extract or change the graph.
func (l *Logger) LogGraph(graph extraction.Graph) {
	if !l.enabled(LevelDebug) {
		return
	}
	for _, edge := range graph {
		if edge.IsSelfEdge() {
			l.record(LevelDebug, "inspect", "discovered file: "+strconv.Quote(edge.Source))
			continue
		}
		l.record(LevelDebug, "inspect", fmt.Sprintf("dependency: %q -> %q (external=%t, kinds=%s)",
			edge.Source, edge.Target, edge.External, edge.ImportKinds))
	}
}

// LogSelection records the members of one selection at debug level. It preserves
// the caller's order and quotes identifiers so control characters cannot add lines.
func (l *Logger) LogSelection(step string, identifiers []string) {
	if !l.enabled(LevelDebug) {
		return
	}
	for _, identifier := range identifiers {
		l.record(LevelDebug, "inspect", step+": "+strconv.Quote(identifier))
	}
}

// LogMembership records each layer or slice and its selected files at debug level.
// Labels are sorted without mutating the supplied membership map.
func (l *Logger) LogMembership(kind string, membership map[string][]string) {
	if !l.enabled(LevelDebug) {
		return
	}
	for _, label := range slices.Sorted(maps.Keys(membership)) {
		l.LogSelection(kind+" "+strconv.Quote(label), membership[label])
	}
}
