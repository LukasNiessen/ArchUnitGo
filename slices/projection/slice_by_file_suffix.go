package projection

import (
	"path"
	"strings"

	kernel "github.com/LukasNiessen/ArchUnitGo/common/projection"
)

// SliceByFileSuffix slices a project by what its files are rather than by where they live: the slice of a
// file is the last word of its filename, so `internal/api/order_handler.go` and
// `internal/shop/user_handler.go` are both in the slice `handler`.
//
// It is this family's Go spelling of the same idea a class-name suffix carries elsewhere. Go has no
// classes and the convention that says what a file holds is its name, in snake_case with the kind last —
// `order_handler.go`, `memory_store.go`, `http_client.go` — so the words are split on `_` and the last of
// them is the name of the slice. The extension is not part of it, and a filename of one word is that
// word: `handler.go` is in the slice `handler` as well.
//
// Two things follow from the definition and are worth writing down:
//
//   - The slices it finds cut across the folders. That is the point — a rule written over this projection
//     asks whether the handlers of a project depend on its stores, wherever the two of them sit — and it
//     is why this is a projection of its own rather than a pattern one could have passed to
//     SliceByPattern.
//   - A test file is in the slice `test`, when CheckOptions.IncludeTestFiles put it in the graph at all.
//     `conn_test.go` ends in the word `test`, so it does, and nothing here special-cases it: which files
//     a rule is about is the check options' question, settled before any of this runs.
//
// A filename with no word to take — one that is all extension, or that ends in the separator itself — is
// in no slice, and an edge with such a file at either end is dropped, the same as for any other mapper
// here.
func SliceByFileSuffix() kernel.MapFunction {
	return sliceBy(fileSuffix)
}

// fileSuffix is the naming half of SliceByFileSuffix: the last `_`-separated word of an identifier's
// filename, without its extension.
//
// It reads the identifier lexically, with path rather than path/filepath, because an identifier is
// already in this library's one separator convention — a projection is pure and never touches the
// filesystem the name came from.
func fileSuffix(identifier string) (string, bool) {
	filename := path.Base(identifier)
	stem := strings.TrimSuffix(filename, path.Ext(filename))
	if separator := strings.LastIndex(stem, "_"); separator >= 0 {
		stem = stem[separator+1:]
	}
	if stem == "" {
		return "", false
	}
	return stem, true
}
