package extraction

import "testing"

func TestDescribeSourcesDescribesEveryDeclaredType(t *testing.T) {
	// A class here is a declared type, whatever it is declared as: the struct, the interface and the name
	// given to another type are three classes, and a grouped declaration is as many as it has specs.
	source := "package api\n\ntype Handler struct{}\n\ntype Reader interface{ Read() error }\n\n" +
		"type (\n\tID string\n\tHandle func() error\n)\n"

	classes := describeOne(t, source).Classes

	want := []struct {
		name        string
		isInterface bool
	}{
		{name: "Handler", isInterface: false},
		{name: "Reader", isInterface: true},
		{name: "ID", isInterface: false},
		{name: "Handle", isInterface: false},
	}
	if len(classes) != len(want) {
		t.Fatalf("described %d classes (%+v), want the %d the file declares", len(classes), classes, len(want))
	}
	for index, expected := range want {
		if classes[index].Name != expected.name {
			t.Errorf("class %d is %q, want %q in the order they were declared", index, classes[index].Name, expected.name)
		}
		if classes[index].Interface != expected.isInterface {
			t.Errorf("%s.Interface = %t, want %t", classes[index].Name, classes[index].Interface, expected.isInterface)
		}
		if classes[index].Path != "internal/api/handler.go" {
			t.Errorf("%s.Path = %q, want the file it was declared in", classes[index].Name, classes[index].Path)
		}
	}
}

func TestDescribeSourcesQualifiesAClassByTheFolderItWasDeclaredIn(t *testing.T) {
	// The identifier a class metric reports and `for classes matching` is matched against: the folder, a dot
	// and the name, so two packages declaring one name are two subjects. At the project root the name is the
	// whole of it, because `..Server` would be a stranger spelling of `Server`.
	//
	// The two paths beside it are the identifiers a measurement is reported by — FileInfo.Path is the subject of
	// every metric about a file, and ClassInfo.Path is the key a file is narrowed by — so both are the
	// normalised identifier and never the string the caller happened to spell: a host separator or a `.`
	// element in either would be the mixing the identifier convention exists to prevent.
	tests := []struct {
		identifier string
		want       string
		wantPath   string
	}{
		{identifier: "internal/api/handler.go", want: "internal/api.Handler", wantPath: "internal/api/handler.go"},
		{identifier: "main.go", want: "Handler", wantPath: "main.go"},
		{identifier: `internal\api\handler.go`, want: "internal/api.Handler", wantPath: "internal/api/handler.go"},
		{identifier: "./internal/api/handler.go", want: "internal/api.Handler", wantPath: "internal/api/handler.go"},
	}

	for _, test := range tests {
		t.Run(test.identifier, func(t *testing.T) {
			files, err := describeSources([]source{{identifier: test.identifier, text: "package api\n\ntype Handler struct{}\n"}})
			if err != nil {
				t.Fatalf("describeSources failed: %v", err)
			}

			if got := files[0].Classes[0].Identifier; got != test.want {
				t.Errorf("Identifier = %q, want %q", got, test.want)
			}
			if got := files[0].Path; got != test.wantPath {
				t.Errorf("FileInfo.Path = %q, want the normalised identifier %q", got, test.wantPath)
			}
			if got := files[0].Classes[0].Path; got != test.wantPath {
				t.Errorf("ClassInfo.Path = %q, want the normalised identifier %q", got, test.wantPath)
			}
		})
	}
}

func TestDescribeSourcesCountsTheFieldsOfAStruct(t *testing.T) {
	// One per name declared, and one for an entry that has none: what the embedded type brings with it is a
	// count of another declaration, which this package would have to resolve a type to know.
	tests := []struct {
		name   string
		source string
		want   int
	}{
		{name: "no fields", source: "package api\n\ntype Handler struct{}\n", want: 0},
		{name: "one field per line", source: "package api\n\ntype Handler struct {\n\tname string\n\tsize int\n}\n", want: 2},
		{name: "a grouped declaration is as many as it has names", source: "package api\n\ntype Handler struct {\n\tname, label string\n}\n", want: 2},
		{name: "an embedded field is one", source: "package api\n\ntype Handler struct {\n\tReader\n\tname string\n}\n", want: 2},
		{name: "a type that is not a struct has none", source: "package api\n\ntype ID string\n", want: 0},
		{name: "the fields of a generic type count too", source: "package api\n\ntype Stack[T any] struct {\n\titems []T\n}\n", want: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			classes := describeOne(t, test.source).Classes

			if classes[0].FieldCount != test.want {
				t.Errorf("FieldCount = %d, want %d for %q", classes[0].FieldCount, test.want, test.source)
			}
		})
	}
}

func TestDescribeSourcesCountsTheMembersAnInterfaceLists(t *testing.T) {
	// An interface's methods are its own declaration, so no search is needed for them — and an embedded
	// interface is one member here for the reason an embedded field is one field.
	tests := []struct {
		name   string
		source string
		want   int
	}{
		{name: "the empty interface", source: "package api\n\ntype Any interface{}\n", want: 0},
		{name: "one per method spec", source: "package api\n\ntype Reader interface {\n\tRead() error\n\tClose() error\n}\n", want: 2},
		{name: "an embedded interface is one", source: "package api\n\ntype Reader interface {\n\tio.Closer\n\tRead() error\n}\n", want: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			classes := describeOne(t, test.source).Classes

			if classes[0].MethodCount != test.want {
				t.Errorf("MethodCount = %d, want %d for %q", classes[0].MethodCount, test.want, test.source)
			}
		})
	}
}

func TestDescribeSourcesCountsMethodsHoweverTheirReceiverIsSpelled(t *testing.T) {
	// A method belongs to its type, not to the way the receiver was written: a value receiver, a pointer
	// receiver and a generic one — with one type parameter or with several, which the parser spells as two
	// different expressions — are all methods of the type they name.
	source := "package api\n\ntype Handler struct{}\n\nfunc (h Handler) Handle() {}\n\n" +
		"func (h *Handler) Close() error { return nil }\n\ntype Stack[T any] struct{}\n\n" +
		"func (s *Stack[T]) Push(item T) {}\n\ntype Pair[K, V any] struct{}\n\n" +
		"func (p *Pair[K, V]) Get() {}\n"

	classes := describeOne(t, source).Classes

	if len(classes) != 3 {
		t.Fatalf("described %+v, want the three types the file declares", classes)
	}
	if classes[0].MethodCount != 2 {
		t.Errorf("Handler.MethodCount = %d, want the value and the pointer receiver counted as 2", classes[0].MethodCount)
	}
	if classes[1].MethodCount != 1 {
		t.Errorf("Stack.MethodCount = %d, want the method on its generic receiver counted as 1", classes[1].MethodCount)
	}
	if classes[2].MethodCount != 1 {
		t.Errorf("Pair.MethodCount = %d, want the method on its two-parameter receiver counted as 1", classes[2].MethodCount)
	}
}

func TestDescribeSourcesAttributesMethodsAcrossTheFilesOfOneFolder(t *testing.T) {
	// A method is declared beside its type rather than inside it, so how many methods a type has is a question
	// about a whole package — and the answer must not depend on which file of it the type was declared in.
	sources := []source{
		{identifier: "internal/api/handler.go", text: "package api\n\ntype Handler struct{}\n\nfunc (h Handler) Handle() {}\n"},
		{identifier: "internal/api/handler_admin.go", text: "package api\n\nfunc (h *Handler) Admin() {}\n"},
	}

	files, err := describeSources(sources)
	if err != nil {
		t.Fatalf("describeSources failed: %v", err)
	}

	if got := files[0].Classes[0].MethodCount; got != 2 {
		t.Errorf("Handler.MethodCount = %d, want the 2 methods its folder declares", got)
	}
	if len(files[1].Classes) != 0 {
		t.Errorf("the second file declares %+v, want no class of its own", files[1].Classes)
	}
}

func TestDescribeSourcesKeepsTheMethodsOfTwoFoldersApart(t *testing.T) {
	// Two packages may both declare a `Handler`, and they are two classes: the folder is half of what a
	// receiver is attributed by, which is also why the class identifier carries it.
	sources := []source{
		{identifier: "internal/api/handler.go", text: "package api\n\ntype Handler struct{}\n\nfunc (h Handler) Handle() {}\n"},
		{identifier: "internal/db/handler.go", text: "package db\n\ntype Handler struct{}\n\nfunc (h Handler) Read() {}\n\nfunc (h Handler) Write() {}\n"},
	}

	files, err := describeSources(sources)
	if err != nil {
		t.Fatalf("describeSources failed: %v", err)
	}

	if got := files[0].Classes[0]; got.Identifier != "internal/api.Handler" || got.MethodCount != 1 {
		t.Errorf("the api Handler is %+v, want internal/api.Handler with its own 1 method", got)
	}
	if got := files[1].Classes[0]; got.Identifier != "internal/db.Handler" || got.MethodCount != 2 {
		t.Errorf("the db Handler is %+v, want internal/db.Handler with its own 2 methods", got)
	}
}

func TestDescribeSourcesLeavesAMethodOfATypeItNeverSawUncounted(t *testing.T) {
	// A receiver naming a type none of the selected files declares is either a type the scope left out or one
	// this library has no class for, and either way there is nothing to count it against.
	files, err := describeSources([]source{{identifier: "internal/api/handler.go", text: "package api\n\nfunc (h Handler) Handle() {}\n"}})
	if err != nil {
		t.Fatalf("describeSources failed: %v", err)
	}

	if len(files[0].Classes) != 0 {
		t.Errorf("described %+v, want no class from a file that declares none", files[0].Classes)
	}
	if files[0].FunctionCount != 0 {
		t.Errorf("FunctionCount = %d, want a method left out of the file's functions all the same", files[0].FunctionCount)
	}
}
