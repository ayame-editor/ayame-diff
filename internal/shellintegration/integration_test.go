package shellintegration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallAndUninstallLinux(t *testing.T) {
	home := t.TempDir()
	e := Environment{GOOS: "linux", Executable: "/opt/Ayame Diff/ayame-diff", Home: home}
	paths, err := Install(e)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("paths=%v", paths)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil || !strings.Contains(string(data), `Exec="/opt/Ayame Diff/ayame-diff" --gui %F`) || !strings.Contains(string(data), "inode/directory") {
		t.Fatalf("desktop=%q err=%v", data, err)
	}
	if err := Uninstall(e); err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("still exists: %s", path)
		}
	}
}

func TestInstallDarwinWorkflow(t *testing.T) {
	home := t.TempDir()
	paths, err := Install(Environment{GOOS: "darwin", Executable: "/Applications/Ayame Diff.app/ayame-diff", Home: home})
	if err != nil || len(paths) != 1 {
		t.Fatalf("paths=%v err=%v", paths, err)
	}
	data, err := os.ReadFile(filepath.Join(paths[0], "Contents", "document.wflow"))
	if err != nil || !strings.Contains(string(data), "Run Shell Script.action") || !strings.Contains(string(data), "--gui") {
		t.Fatalf("workflow=%q err=%v", data, err)
	}
}

func TestInstallWindowsRegistryAndSendTo(t *testing.T) {
	home, appData := t.TempDir(), t.TempDir()
	var calls [][]string
	run := func(name string, args ...string) error {
		calls = append(calls, append([]string{name}, args...))
		return nil
	}
	paths, err := Install(Environment{GOOS: "windows", Executable: `C:\Program Files\Ayame\ayame-diff.exe`, Home: home, AppData: appData, Run: run})
	if err != nil || len(calls) != 4 || len(paths) != 1 {
		t.Fatalf("paths=%v calls=%v err=%v", paths, calls, err)
	}
	data, err := os.ReadFile(paths[0])
	if err != nil || !strings.Contains(string(data), "--gui %*") {
		t.Fatalf("SendTo=%q err=%v", data, err)
	}
}
