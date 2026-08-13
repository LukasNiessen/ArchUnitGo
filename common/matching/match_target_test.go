package matching

import "testing"

func TestMatchTargetExtract(t *testing.T) {
	tests := []struct {
		name       string
		target     MatchTarget
		identifier string
		want       string
		wantOk     bool
	}{
		{"path is the whole identifier", TargetPath, "internal/api/handler.go", "internal/api/handler.go", true},
		{"path of a root file", TargetPath, "main.go", "main.go", true},
		{"filename is the last segment", TargetFilename, "internal/api/handler.go", "handler.go", true},
		{"filename of a root file", TargetFilename, "main.go", "main.go", true},
		{"filename of a folder", TargetFilename, "internal/api", "api", true},
		{"folder is everything else", TargetPathWithoutFilename, "internal/api/handler.go", "internal/api", true},
		{"folder of a root file is the root", TargetPathWithoutFilename, "main.go", ".", true},
		{"folder of an absolute identifier", TargetPathWithoutFilename, "/home/dev/repo/main.go", "/home/dev/repo", true},
		{"classname unqualified", TargetClassname, "Handler", "Handler", true},
		{"classname qualified by package", TargetClassname, "api.Handler", "Handler", true},
		{"classname qualified by import path", TargetClassname, "internal/api.Handler", "Handler", true},
		{"empty identifier has no target", TargetPath, "", "", false},
		{"undeclared target", MatchTarget(9), "internal/api/handler.go", "", false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := test.target.extract(test.identifier)
			if ok != test.wantOk {
				t.Fatalf("%v.extract(%q) ok = %v, want %v", test.target, test.identifier, ok, test.wantOk)
			}
			if got != test.want {
				t.Errorf("%v.extract(%q) = %q, want %q", test.target, test.identifier, got, test.want)
			}
		})
	}
}

func TestMatchTargetNames(t *testing.T) {
	tests := []struct {
		target MatchTarget
		want   string
	}{
		{TargetPath, "path"},
		{TargetFilename, "filename"},
		{TargetPathWithoutFilename, "path without filename"},
		{TargetClassname, "classname"},
		{MatchTarget(9), "unknown"},
	}

	for _, test := range tests {
		t.Run(test.want, func(t *testing.T) {
			if got := test.target.String(); got != test.want {
				t.Errorf("MatchTarget(%d).String() = %q, want %q", test.target, got, test.want)
			}
			if valid := test.target.Valid(); valid != (test.want != "unknown") {
				t.Errorf("MatchTarget(%d).Valid() = %v", test.target, valid)
			}
		})
	}
}

func TestTargetPathIsTheZeroValue(t *testing.T) {
	// Looking at the whole identifier is the least surprising default, so it is what an
	// uninitialised target means.
	var target MatchTarget

	if target != TargetPath {
		t.Errorf("the zero MatchTarget is %v, want %v", target, TargetPath)
	}
}
