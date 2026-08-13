package extraction

import (
	"path"
	"strings"
)

// Identifiers are the strings every user pattern is matched against, so they have to be normalised
// and stable. The convention for this library, and it is never mixed:
//
//   - separators are forward slashes, whatever the host operating system uses;
//   - the path is lexically clean — no `.` element, no `//`, no trailing slash;
//   - internal nodes are project-relative, rooted at the directory holding `go.mod`;
//   - external nodes are Go import paths, which are already slash-separated and relative to
//     nothing.
//
// Everything that mints an identifier goes through this file. NormalizeIdentifier preserves
// whether the string it was given was absolute, because silently rewriting an absolute path into a
// relative one would be the mixing this convention exists to prevent — use RelativeIdentifier to
// cross that boundary deliberately.

// NormalizeIdentifier puts an identifier into the canonical form described above. The empty
// string normalises to itself: it is the absence of an identifier, not the project root.
func NormalizeIdentifier(identifier string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(identifier, `\`, "/"))
	if trimmed == "" {
		return ""
	}
	// path.Clean drops `.` elements, collapses `//` and strips the trailing slash, while keeping
	// a leading `/` and any leading `..` exactly as they were.
	return path.Clean(trimmed)
}

// RelativeIdentifier turns an absolute path into the project-relative identifier used throughout
// the graph. It reports false when target lies outside root, which is the caller's signal that the
// path belongs to no project node rather than something to paper over with `../..`.
//
// It is pure lexical arithmetic on normalised identifiers: nothing here touches the filesystem, and
// the answer does not depend on the host operating system's separator.
func RelativeIdentifier(root, target string) (string, bool) {
	normalizedRoot := NormalizeIdentifier(root)
	normalizedTarget := NormalizeIdentifier(target)
	if normalizedRoot == "" || normalizedTarget == "" {
		return "", false
	}
	if normalizedTarget == normalizedRoot {
		return ".", true
	}
	if normalizedRoot == "." {
		// The target is already relative to the root, unless it is absolute or escapes upwards.
		if path.IsAbs(normalizedTarget) || escapesUpwards(normalizedTarget) {
			return "", false
		}
		return normalizedTarget, true
	}

	prefix := normalizedRoot
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	relative, inside := strings.CutPrefix(normalizedTarget, prefix)
	if !inside || relative == "" || escapesUpwards(relative) {
		return "", false
	}
	return relative, true
}

func escapesUpwards(identifier string) bool {
	return identifier == ".." || strings.HasPrefix(identifier, "../")
}
