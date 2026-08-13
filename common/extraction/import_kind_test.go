package extraction

import (
	"slices"
	"testing"
)

func TestImportKindString(t *testing.T) {
	tests := []struct {
		kind ImportKind
		want string
	}{
		{ImportKindPlain, "plain"},
		{ImportKindAliased, "aliased"},
		{ImportKindBlank, "blank"},
		{ImportKindDot, "dot"},
		{ImportKind(99), "unknown"},
	}

	for _, test := range tests {
		if got := test.kind.String(); got != test.want {
			t.Errorf("ImportKind(%d).String() = %q, want %q", test.kind, got, test.want)
		}
	}
}

func TestImportKindValid(t *testing.T) {
	for _, kind := range []ImportKind{ImportKindPlain, ImportKindAliased, ImportKindBlank, ImportKindDot} {
		if !kind.Valid() {
			t.Errorf("%v should be a valid import kind", kind)
		}
	}
	if ImportKind(4).Valid() {
		t.Error("ImportKind(4) should not be valid")
	}
}

func TestImportKindSetHoldsWhatWasPutIn(t *testing.T) {
	set := NewImportKindSet(ImportKindPlain, ImportKindDot)

	if !set.Contains(ImportKindPlain) || !set.Contains(ImportKindDot) {
		t.Errorf("%v should contain plain and dot", set)
	}
	if set.Contains(ImportKindBlank) || set.Contains(ImportKindAliased) {
		t.Errorf("%v should not contain blank or aliased", set)
	}
	if set.Len() != 2 {
		t.Errorf("Len() = %d, want 2", set.Len())
	}
	if set.Empty() {
		t.Errorf("%v should not be empty", set)
	}
}

func TestEmptyImportKindSet(t *testing.T) {
	var set ImportKindSet

	if !set.Empty() {
		t.Error("the zero ImportKindSet should be empty")
	}
	if set.Len() != 0 {
		t.Errorf("Len() = %d, want 0", set.Len())
	}
	if set.Contains(ImportKindPlain) {
		t.Error("the zero ImportKindSet should contain nothing")
	}
	if got := set.String(); got != "[]" {
		t.Errorf("String() = %q, want %q", got, "[]")
	}
}

func TestImportKindSetIsASet(t *testing.T) {
	set := NewImportKindSet(ImportKindBlank, ImportKindBlank, ImportKindBlank)

	if set.Len() != 1 {
		t.Errorf("Len() = %d, want 1 — duplicates are one element", set.Len())
	}
}

func TestImportKindSetIgnoresUndeclaredKinds(t *testing.T) {
	set := NewImportKindSet(ImportKind(7), ImportKindPlain)

	if set.Len() != 1 || !set.Contains(ImportKindPlain) {
		t.Errorf("%v should hold plain and nothing else", set)
	}
	if set.Contains(ImportKind(7)) {
		t.Error("an undeclared kind should never be reported as contained")
	}
}

func TestImportKindSetWithDoesNotMutateTheReceiver(t *testing.T) {
	original := NewImportKindSet(ImportKindPlain)

	extended := original.With(ImportKindDot)

	if original.Contains(ImportKindDot) {
		t.Errorf("With mutated the receiver: %v", original)
	}
	if !extended.Contains(ImportKindPlain) || !extended.Contains(ImportKindDot) {
		t.Errorf("%v should hold plain and dot", extended)
	}
}

func TestImportKindSetUnion(t *testing.T) {
	left := NewImportKindSet(ImportKindPlain, ImportKindAliased)
	right := NewImportKindSet(ImportKindAliased, ImportKindBlank)

	union := left.Union(right)

	want := []ImportKind{ImportKindPlain, ImportKindAliased, ImportKindBlank}
	if got := union.Kinds(); !slices.Equal(got, want) {
		t.Errorf("Kinds() = %v, want %v", got, want)
	}
}

func TestImportKindSetKindsIsStableAndInDeclarationOrder(t *testing.T) {
	set := NewImportKindSet(ImportKindDot, ImportKindPlain, ImportKindBlank)

	want := []ImportKind{ImportKindPlain, ImportKindBlank, ImportKindDot}
	for range 5 {
		if got := set.Kinds(); !slices.Equal(got, want) {
			t.Fatalf("Kinds() = %v, want %v", got, want)
		}
	}
	if got := set.String(); got != "[plain, blank, dot]" {
		t.Errorf("String() = %q, want %q", got, "[plain, blank, dot]")
	}
}
