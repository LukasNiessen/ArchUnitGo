package extraction

import (
	"slices"
	"testing"
)

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

func TestDescribeSourcesNamesTheFieldsAStructDeclares(t *testing.T) {
	// A cohesion metric needs the fields named and not only counted, because a method reaches one by name. An
	// embedded field is named by the type it embeds, without the package qualifier, the pointer star or the type
	// arguments — that is what a method selects on the receiver.
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{name: "one per line", source: "package api\n\ntype Handler struct {\n\tname string\n\tsize int\n}\n", want: []string{"name", "size"}},
		{name: "a grouped declaration is as many as it has names", source: "package api\n\ntype Handler struct {\n\tname, label string\n}\n", want: []string{"name", "label"}},
		{name: "an embedded field is its type", source: "package api\n\ntype Handler struct {\n\tReader\n\tname string\n}\n", want: []string{"Reader", "name"}},
		{name: "an embedded pointer to a qualified type", source: "package api\n\ntype Handler struct {\n\t*io.Reader\n}\n", want: []string{"Reader"}},
		{name: "an embedded generic type", source: "package api\n\ntype Handler struct {\n\tStack[int]\n}\n", want: []string{"Stack"}},
		{name: "a type that is not a struct declares none", source: "package api\n\ntype ID string\n", want: nil},
		{name: "an interface declares none", source: "package api\n\ntype Reader interface {\n\tRead() error\n}\n", want: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			class := describeOne(t, test.source).Classes[0]

			var names []string
			for _, field := range class.Fields {
				names = append(names, field.Name)
			}
			if !slices.Equal(names, test.want) {
				t.Errorf("Fields = %v, want %v in the order they were declared", names, test.want)
			}
			if class.FieldCount != len(class.Fields) {
				t.Errorf("FieldCount = %d, want the %d fields it named", class.FieldCount, len(class.Fields))
			}
		})
	}
}

func TestDescribeSourcesRecordsWhichFieldsEachMethodReaches(t *testing.T) {
	// The relation every cohesion metric is a formula over, in both directions: each method keeps the fields it
	// selects on its receiver, and each field keeps the methods that select it. A field nobody reaches is still a
	// field, which is what makes it count against the class's cohesion.
	source := "package api\n\ntype Handler struct {\n\treader string\n\twriter string\n\tunused int\n}\n\n" +
		"func (h Handler) Read() string { return h.reader }\n\n" +
		"func (h *Handler) Copy() { h.writer = h.reader }\n\n" +
		"func (h Handler) String() string { return \"handler\" }\n"

	class := describeOne(t, source).Classes[0]

	wantMethods := []MethodInfo{
		{Name: "Read", AccessedFields: []string{"reader"}},
		{Name: "Copy", AccessedFields: []string{"writer", "reader"}},
		{Name: "String", AccessedFields: nil},
	}
	assertMethods(t, class, wantMethods)
	assertAccessedBy(t, class, map[string][]string{
		"reader": {"Read", "Copy"},
		"writer": {"Copy"},
		"unused": nil,
	})
	if class.MethodCount != 3 {
		t.Errorf("MethodCount = %d, want the 3 methods the relation holds", class.MethodCount)
	}
}

func TestDescribeSourcesRecordsOnlyWhatAMethodSelectsOnItsOwnReceiver(t *testing.T) {
	// A selection at any depth reaches the field it starts at, and it reaches it once however often the body
	// spells it. A selection on anything else is a field of another value, and a name that is not a field of this
	// class — a method of it, a field promoted from an embedded type — is not an access this package can
	// attribute: naming it would need a resolved type.
	source := "package api\n\ntype Handler struct {\n\treader Reader\n\tname string\n}\n\n" +
		"func (h Handler) Read() {\n\th.reader.Fill()\n\th.reader.Drain()\n\th.Log()\n" +
		"\tother := Handler{}\n\t_ = other.name\n\t_ = h.Promoted\n}\n\nfunc (h Handler) Log() {}\n"

	class := describeOne(t, source).Classes[0]

	assertMethods(t, class, []MethodInfo{
		{Name: "Read", AccessedFields: []string{"reader"}},
		{Name: "Log", AccessedFields: nil},
	})
	assertAccessedBy(t, class, map[string][]string{"reader": {"Read"}, "name": nil})
}

func TestDescribeSourcesRecordsAnAccessFromAnotherFileOfTheFolder(t *testing.T) {
	// A method is declared beside its type rather than inside it, so which fields it reaches is a question about
	// the whole folder — the second pass exists because the fields are only known once the type's own file has
	// been read.
	sources := []source{
		{identifier: "internal/api/handler.go", text: "package api\n\ntype Handler struct {\n\treader string\n}\n"},
		{identifier: "internal/api/handler_read.go", text: "package api\n\nfunc (h *Handler) Read() string { return h.reader }\n"},
	}

	files, err := describeSources(sources)
	if err != nil {
		t.Fatalf("describeSources failed: %v", err)
	}

	assertMethods(t, files[0].Classes[0], []MethodInfo{{Name: "Read", AccessedFields: []string{"reader"}}})
	assertAccessedBy(t, files[0].Classes[0], map[string][]string{"reader": {"Read"}})
}

func TestDescribeSourcesRecordsNoAccessForAMethodThatCannotNameItsReceiver(t *testing.T) {
	// What the receiver is called is how an access to a field is recognized, so a method that does not name it,
	// and one that names it `_`, reaches nothing — while still being one of the class's methods, because the
	// declaration is still a method of the type.
	// The blank receiver selects on `_` in its body, which is what a parser accepts and a type checker would not:
	// this package parses rather than type-checks, and without the selection the fixture would pass just as well
	// if `_` were read as a receiver name.
	source := "package api\n\ntype Handler struct {\n\tname string\n}\n\n" +
		"func (Handler) Anonymous() {}\n\nfunc (_ Handler) Blank() { _ = _.name }\n\n" +
		"func (h Handler) Named() string { return h.name }\n"

	class := describeOne(t, source).Classes[0]

	assertMethods(t, class, []MethodInfo{
		{Name: "Anonymous", AccessedFields: nil},
		{Name: "Blank", AccessedFields: nil},
		{Name: "Named", AccessedFields: []string{"name"}},
	})
	assertAccessedBy(t, class, map[string][]string{"name": {"Named"}})
}

func TestDescribeSourcesLeavesAnInterfacesMembersOutOfTheRelation(t *testing.T) {
	// An interface's members are counted as its methods and are not part of the relation: a member has no body,
	// so there is no field for it to reach and no cohesion to read off it.
	class := describeOne(t, "package api\n\ntype Reader interface {\n\tRead() error\n\tClose() error\n}\n").Classes[0]

	if class.MethodCount != 2 {
		t.Errorf("MethodCount = %d, want the 2 members it lists", class.MethodCount)
	}
	if len(class.Methods) != 0 || len(class.Fields) != 0 {
		t.Errorf("the relation of an interface is %+v over %+v, want neither", class.Methods, class.Fields)
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

func TestDescribeSourcesWritesTheTwoDirectionsOfTheRelationDownOnce(t *testing.T) {
	// The methods a field is reached by and the fields a method reaches are one fact recorded twice, because the
	// cohesion family asks it both ways. They are built together here so that they cannot disagree, and this is
	// the test that holds them to it.
	source := "package api\n\ntype Handler struct {\n\treader string\n\twriter string\n\tbuffer []byte\n}\n\n" +
		"func (h Handler) Read() string { return h.reader }\n\n" +
		"func (h *Handler) Copy() { h.writer = h.reader; h.buffer = nil }\n\n" +
		"func (h *Handler) Flush() { h.buffer = nil }\n"

	class := describeOne(t, source).Classes[0]

	for _, method := range class.Methods {
		for _, name := range method.AccessedFields {
			index := slices.IndexFunc(class.Fields, func(field FieldInfo) bool { return field.Name == name })
			if index < 0 {
				t.Fatalf("%s reaches %q, which is not a field of the class", method.Name, name)
			}
			if !slices.Contains(class.Fields[index].AccessedBy, method.Name) {
				t.Errorf("%s reaches %q, but %q is accessed by %v", method.Name, name, name, class.Fields[index].AccessedBy)
			}
		}
	}
	for _, field := range class.Fields {
		for _, name := range field.AccessedBy {
			index := slices.IndexFunc(class.Methods, func(method MethodInfo) bool { return method.Name == name })
			if index < 0 {
				t.Fatalf("%q is accessed by %q, which is not a method of the class", field.Name, name)
			}
			if !slices.Contains(class.Methods[index].AccessedFields, field.Name) {
				t.Errorf("%q is accessed by %s, but %s reaches %v", field.Name, name, name, class.Methods[index].AccessedFields)
			}
		}
	}
}

// assertMethods checks the methods of a class and the fields each of them reaches, in the order they were
// declared. It compares the whole relation rather than one method at a time, because a method missing from it and
// a method reaching one field too many are the same mistake seen from two sides.
func assertMethods(t *testing.T, class ClassInfo, want []MethodInfo) {
	t.Helper()

	if len(class.Methods) != len(want) {
		t.Fatalf("%s has the methods %+v, want %+v", class.Name, class.Methods, want)
	}
	for index, expected := range want {
		got := class.Methods[index]
		if got.Name != expected.Name {
			t.Errorf("method %d is %q, want %q in the order they were declared", index, got.Name, expected.Name)
		}
		if !slices.Equal(got.AccessedFields, expected.AccessedFields) {
			t.Errorf("%s reaches %v, want %v", got.Name, got.AccessedFields, expected.AccessedFields)
		}
	}
}

// assertAccessedBy checks which methods each field of a class is reached by. The table names every field, so a
// field the extractor invented or dropped fails it as loudly as a wrong access does.
func assertAccessedBy(t *testing.T, class ClassInfo, want map[string][]string) {
	t.Helper()

	if len(class.Fields) != len(want) {
		t.Fatalf("%s has the fields %+v, want the %d the table names", class.Name, class.Fields, len(want))
	}
	for _, field := range class.Fields {
		expected, named := want[field.Name]
		if !named {
			t.Errorf("%s has a field %q the table does not name", class.Name, field.Name)
			continue
		}
		if !slices.Equal(field.AccessedBy, expected) {
			t.Errorf("%q is accessed by %v, want %v", field.Name, field.AccessedBy, expected)
		}
	}
}
