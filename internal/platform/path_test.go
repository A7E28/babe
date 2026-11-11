package platform

import (
	"reflect"
	"testing"
)

func TestCleanPathString(t *testing.T) {
	input := "C:\\bin;  C:\\Users\\test\\platform-tools  ; ;C:\\Windows\\system32;"
	expected := []string{
		"C:\\bin",
		"C:\\Users\\test\\platform-tools",
		"C:\\Windows\\system32",
	}

	actual := CleanPathString(input, ';')
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("got %v, want %v", actual, expected)
	}
}

func TestContainsPath(t *testing.T) {
	paths := []string{"/usr/bin", "/home/user/.local/bin", "C:\\Platform-Tools"}

	if !ContainsPath(paths, "c:\\platform-tools", true) {
		t.Errorf("expected case-insensitive match for Windows path")
	}

	if ContainsPath(paths, "/USR/BIN", false) {
		t.Errorf("did not expect case-sensitive match for Unix path")
	}

	if !ContainsPath(paths, "/usr/bin", false) {
		t.Errorf("expected exact match for Unix path")
	}
}

func TestFilterOutPath(t *testing.T) {
	paths := []string{"/bin", "/usr/bin", "/home/user/platform-tools"}
	filtered := FilterOutPath(paths, "/usr/bin", false)
	expected := []string{"/bin", "/home/user/platform-tools"}

	if !reflect.DeepEqual(filtered, expected) {
		t.Errorf("got %v, want %v", filtered, expected)
	}
}

func TestAppendPathSegment(t *testing.T) {
	tests := []struct {
		current string
		dir     string
		delim   rune
		want    string
	}{
		{"", `C:\babe\bin`, ';', `C:\babe\bin`},
		{`C:\Windows\System32`, `C:\babe\bin`, ';', `C:\Windows\System32;C:\babe\bin`},
		{`C:\Windows\System32;`, `C:\babe\bin`, ';', `C:\Windows\System32;C:\babe\bin`},
		{`C:\Windows\System32;;;  `, `C:\babe\bin`, ';', `C:\Windows\System32;C:\babe\bin`},
	}

	for _, tc := range tests {
		if got := AppendPathSegment(tc.current, tc.dir, tc.delim); got != tc.want {
			t.Errorf("AppendPathSegment(%q, %q) = %q, want %q", tc.current, tc.dir, got, tc.want)
		}
	}
}
