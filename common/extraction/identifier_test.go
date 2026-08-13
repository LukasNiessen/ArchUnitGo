package extraction

import "testing"

func TestNormalizeIdentifier(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"already canonical", "internal/api/handler.go", "internal/api/handler.go"},
		{"windows separators", `internal\api\handler.go`, "internal/api/handler.go"},
		{"mixed separators", `internal\api/handler.go`, "internal/api/handler.go"},
		{"leading dot slash", "./internal/api", "internal/api"},
		{"duplicate separators", "internal//api///handler.go", "internal/api/handler.go"},
		{"trailing separator", "internal/api/", "internal/api"},
		{"interior dot element", "internal/./api", "internal/api"},
		{"parent element resolved", "internal/db/../api", "internal/api"},
		{"surrounding whitespace", "  internal/api  ", "internal/api"},
		{"absolute stays absolute", "/home/dev/repo/main.go", "/home/dev/repo/main.go"},
		{"absolute cleaned", `\home\dev\..\dev\repo\`, "/home/dev/repo"},
		{"go import path", "github.com/LukasNiessen/ArchUnitGo/common", "github.com/LukasNiessen/ArchUnitGo/common"},
		{"standard library import path", "fmt", "fmt"},
		{"project root", "./", "."},
		{"empty is the absence of an identifier", "", ""},
		{"whitespace only", "   ", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NormalizeIdentifier(test.in); got != test.want {
				t.Errorf("NormalizeIdentifier(%q) = %q, want %q", test.in, got, test.want)
			}
		})
	}
}

func TestNormalizeIdentifierIsIdempotent(t *testing.T) {
	inputs := []string{`internal\api\`, "./a//b/../c", "/tmp/x/./y", "fmt", ""}

	for _, in := range inputs {
		once := NormalizeIdentifier(in)
		if twice := NormalizeIdentifier(once); twice != once {
			t.Errorf("NormalizeIdentifier(%q) is not stable: %q then %q", in, once, twice)
		}
	}
}

func TestRelativeIdentifier(t *testing.T) {
	tests := []struct {
		name   string
		root   string
		target string
		want   string
		wantOk bool
	}{
		{"file under root", "/home/dev/repo", "/home/dev/repo/internal/api/handler.go", "internal/api/handler.go", true},
		{"directory under root", "/home/dev/repo", "/home/dev/repo/internal", "internal", true},
		{"root itself", "/home/dev/repo", "/home/dev/repo", ".", true},
		{"windows separators", `\home\dev\repo`, `\home\dev\repo\internal\api`, "internal/api", true},
		{"unclean input", "/home/dev/repo/", "/home/dev/repo/./internal//api", "internal/api", true},
		{"project root as root", ".", "internal/api/handler.go", "internal/api/handler.go", true},
		{"escaping a relative root", ".", "../other/main.go", "", false},
		{"sibling of root", "/home/dev/repo", "/home/dev/other/main.go", "", false},
		{"escaping an absolute root", "/home/dev/repo", "/home/dev/repo/../other", "", false},
		{"parent of root", "/home/dev/repo", "/home/dev", "", false},
		{"prefix but not a child", "/home/dev/repo", "/home/dev/repository/main.go", "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := RelativeIdentifier(test.root, test.target)
			if ok != test.wantOk {
				t.Fatalf("RelativeIdentifier(%q, %q) ok = %v, want %v", test.root, test.target, ok, test.wantOk)
			}
			if got != test.want {
				t.Errorf("RelativeIdentifier(%q, %q) = %q, want %q", test.root, test.target, got, test.want)
			}
		})
	}
}

func TestRelativeIdentifierRefusesToMixConventions(t *testing.T) {
	// An absolute target against a relative root cannot be made relative, and guessing would be
	// exactly the mixing the identifier convention forbids.
	if got, ok := RelativeIdentifier("repo", "/home/dev/repo/main.go"); ok {
		t.Errorf("RelativeIdentifier should have refused, got %q", got)
	}
}
