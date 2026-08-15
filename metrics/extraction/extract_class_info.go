package extraction

import (
	"go/ast"
	"go/token"
)

// ClassInfo is one type a file declares, and the two counts a rule can be written about it: how many
// fields it has and how many methods.
//
// Go has no classes. The vocabulary is the family's — matching.TargetClassname says the same thing about
// the scope verb `for classes matching` — and in this library a class is a declared type: a struct, an
// interface, or a name given to another type. That is the population `for classes matching` selects and
// the population `count, classes` counts, so the word means one thing everywhere.
//
// Two of the family's class-level metrics are deliberately absent rather than faked, because Go has
// nothing for them to be about: there is no inheritance, so depth of inheritance and number of children
// have no answer here, and a metric returning 0 for every type in every project would be a rule that
// passes forever while looking like it holds.
type ClassInfo struct {
	// Name is the declared name on its own — `Handler`. It is what a reader of a report recognizes, and
	// what a method's receiver names.
	Name string
	// Identifier is the name qualified by the folder the type was declared in — `internal/api.Handler`,
	// and `Handler` for a type declared at the project root.
	//
	// It is the string a class metric reports as its subject and the one `for classes matching` is
	// matched against: matching.TargetClassname strips exactly this qualification off before comparing,
	// so a pattern is written about the bare name while the report still says which package the type was
	// in. Two packages declaring `Handler` are two subjects, which is the whole reason the qualification
	// is carried at all.
	Identifier string
	// Path is the identifier of the file the type was declared in — `internal/api/handler.go`. A class
	// metric reports the class, and this is how a caller gets from it to the file to open.
	Path string
	// Interface says whether the declared type is an interface. It is what `count, interfaces` counts,
	// and the reason a metric about interfaces needs no second population: an interface is a class, so
	// `classes` counts it too.
	Interface bool
	// FieldCount is how many fields the type declares, and 0 for a type that is not a struct.
	//
	// An embedded field counts as one, because that is what the declaration adds here; how many fields
	// the embedded type brings with it is a question about another declaration. A grouped `a, b int` is
	// two.
	FieldCount int
	// MethodCount is how many methods the type has: for an interface the members it lists, and for every
	// other type the functions declared with it as their receiver, pointer and value receivers alike.
	//
	// A method is declared beside its type rather than inside it, so this is counted across the files of
	// the type's own folder — which are the files the rule selected, as ExtractFileInfo describes. An
	// interface's members are its own declaration and need no such search.
	MethodCount int
}

// extractClassInfo describes the types this file declares, in the order they were declared. A grouped
// `type ( A ...; B ... )` declaration is as many classes as it has specs, because a rule about a type
// should not care how the declarations were bracketed.
func extractClassInfo(identifier, directory string, file *ast.File) []ClassInfo {
	var classes []ClassInfo
	for _, declaration := range file.Decls {
		declared, isDeclaration := declaration.(*ast.GenDecl)
		if !isDeclaration || declared.Tok != token.TYPE {
			continue
		}
		for _, spec := range declared.Specs {
			if declaredType, isType := spec.(*ast.TypeSpec); isType {
				classes = append(classes, newClassInfo(identifier, directory, declaredType))
			}
		}
	}
	return classes
}

// newClassInfo describes one declared type: its names, and whichever of the two counts its declaration
// answers on its own. A type that is neither a struct nor an interface — `type ID string`, a function
// type, an alias — has no fields and no members of its own, and gets its methods from countReceivers like
// a struct does.
func newClassInfo(identifier, directory string, declared *ast.TypeSpec) ClassInfo {
	class := ClassInfo{
		Name:       declared.Name.Name,
		Identifier: qualify(directory, declared.Name.Name),
		Path:       identifier,
	}
	switch underlying := declared.Type.(type) {
	case *ast.StructType:
		class.FieldCount = countFields(underlying.Fields)
	case *ast.InterfaceType:
		class.Interface = true
		class.MethodCount = countFields(underlying.Methods)
	}
	return class
}

// qualify spells the identifier of a declared type: the folder it was declared in, a dot, and the name —
// `internal/api.Handler`. That is the shape matching.TargetClassname strips a bare name out of.
//
// A type declared at the project root is its bare name. The root's folder identifier is `.`, and
// `..Server` would be a stranger spelling of `Server` than the name itself — while still matching every
// pattern the name does, because the qualification is stripped before matching either way.
func qualify(directory, name string) string {
	if directory == "" || directory == "." {
		return name
	}
	return directory + "." + name
}

// countFields counts the members a struct's or an interface's field list declares: one per name, and one
// for an entry that has no name.
//
// The nameless entry is the embedded one — a field, an interface, or an element of a type constraint —
// and counting it as one is the same reading in both directions: the declaration adds one member here,
// and what the embedded type brings with it is a count of another declaration, which this package would
// have to resolve a type to know.
func countFields(fields *ast.FieldList) int {
	if fields == nil {
		return 0
	}
	count := 0
	for _, field := range fields.List {
		count += max(len(field.Names), 1)
	}
	return count
}

// receiver is a type methods can be declared on: the folder the declarations live in, and the name of the
// type. The folder is half of the key because a method travels with its package rather than its file, and
// two packages may both declare a `Handler`.
type receiver struct {
	directory string
	name      string
}

// countReceivers adds this file's method declarations to a tally per receiver type. A declaration whose
// receiver names no type — which the parser accepts and the compiler does not — is left out rather than
// counted against a nameless class.
func countReceivers(methods map[receiver]int, directory string, file *ast.File) {
	for _, declaration := range file.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction || function.Recv == nil {
			continue
		}
		if name := receiverTypeName(function.Recv); name != "" {
			methods[receiver{directory: directory, name: name}]++
		}
	}
}

// receiverTypeName is the name of the type a method is declared on, with the pointer star and the type
// parameters of a generic receiver stripped: `Stack` of both `func (s Stack) ...` and
// `func (s *Stack[T]) ...`. A method belongs to the type, not to the way the receiver was spelled.
func receiverTypeName(receivers *ast.FieldList) string {
	if receivers == nil || len(receivers.List) == 0 {
		return ""
	}
	return baseTypeName(receivers.List[0].Type)
}

// baseTypeName unwraps a receiver expression down to the declared name it names, and is empty for
// anything else.
func baseTypeName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.StarExpr:
		return baseTypeName(typed.X)
	case *ast.IndexExpr:
		return baseTypeName(typed.X)
	case *ast.IndexListExpr:
		return baseTypeName(typed.X)
	case *ast.Ident:
		return typed.Name
	default:
		return ""
	}
}

// attributeMethods gives every class the methods its folder declared on it, on top of the members its own
// declaration listed. It is the second pass of describeSources, and it mutates the classes in place
// because they are the slices this package built moments ago.
//
// A tallied receiver naming a type none of these files declares adds nothing: it is either a type in a
// file the scope left out, or one this library has no class for.
func attributeMethods(files []FileInfo, methods map[receiver]int) {
	for fileIndex := range files {
		for classIndex := range files[fileIndex].Classes {
			class := &files[fileIndex].Classes[classIndex]
			class.MethodCount += methods[receiver{directory: files[fileIndex].Directory, name: class.Name}]
		}
	}
}
