package doctor

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFindExecutablesInPath(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	binName := "adb"
	if runtime.GOOS == "windows" {
		binName = "adb.exe"
	}

	bin1 := filepath.Join(dir1, binName)
	bin2 := filepath.Join(dir2, binName)

	if err := os.WriteFile(bin1, []byte("mock1"), 0755); err != nil {
		t.Fatalf("write bin1: %v", err)
	}
	if err := os.WriteFile(bin2, []byte("mock2"), 0755); err != nil {
		t.Fatalf("write bin2: %v", err)
	}

	pathEnv := dir1 + string(os.PathListSeparator) + dir2 + string(os.PathListSeparator) + dir1
	matches := FindExecutablesInPath("adb", pathEnv)

	if len(matches) != 2 {
		t.Fatalf("expected 2 deduplicated matches, got %d: %v", len(matches), matches)
	}
	if matches[0] != bin1 || matches[1] != bin2 {
		t.Errorf("unexpected matches order: %v", matches)
	}
}
