// Package shellintegration installs per-user file-manager launch entries.
package shellintegration

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Environment makes platform integration deterministic and testable.
type Environment struct {
	GOOS       string
	Executable string
	Home       string
	AppData    string
	Run        func(string, ...string) error
}

func (e Environment) runner(name string, args ...string) error {
	if e.Run != nil {
		return e.Run(name, args...)
	}
	return exec.Command(name, args...).Run()
}

// Install registers Ayame Diff for the current user and returns created paths.
func Install(e Environment) ([]string, error) {
	switch e.GOOS {
	case "windows":
		return installWindows(e)
	case "darwin":
		return installDarwin(e)
	default:
		return installLinux(e)
	}
}

// Uninstall removes current-user file-manager registration.
func Uninstall(e Environment) error {
	switch e.GOOS {
	case "windows":
		for _, key := range windowsKeys {
			if err := e.runner("reg.exe", "DELETE", key, "/f"); err != nil {
				// Missing keys are harmless; reg.exe reports them as nonzero.
				continue
			}
		}
		return removeIfExists(filepath.Join(appData(e), "Microsoft", "Windows", "SendTo", "Compare with Ayame Diff.cmd"))
	case "darwin":
		return os.RemoveAll(filepath.Join(e.Home, "Library", "Services", "Compare with Ayame Diff.workflow"))
	default:
		if err := removeIfExists(filepath.Join(e.Home, ".local", "share", "applications", "ayame-diff.desktop")); err != nil {
			return err
		}
		return removeIfExists(filepath.Join(e.Home, ".local", "share", "icons", "hicolor", "scalable", "apps", "ayame-diff.svg"))
	}
}

func installLinux(e Environment) ([]string, error) {
	desktop := filepath.Join(e.Home, ".local", "share", "applications", "ayame-diff.desktop")
	icon := filepath.Join(e.Home, ".local", "share", "icons", "hicolor", "scalable", "apps", "ayame-diff.svg")
	content := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Ayame Diff
Name[ja]=Ayame Diff
Comment=Compare files and folders
Exec=%s --gui %%F
Icon=ayame-diff
Terminal=false
Categories=Utility;Development;
MimeType=text/plain;text/csv;application/json;inode/directory;
Actions=OpenGUI;

[Desktop Action OpenGUI]
Name=Open Ayame Diff
Name[ja]=Ayame Diff を開く
Exec=%s gui
`, desktopQuote(e.Executable), desktopQuote(e.Executable))
	if err := writeFile(desktop, []byte(content), 0o755); err != nil {
		return nil, err
	}
	if err := writeFile(icon, []byte(faviconSVG), 0o644); err != nil {
		return nil, err
	}
	return []string{desktop, icon}, nil
}

func installDarwin(e Environment) ([]string, error) {
	root := filepath.Join(e.Home, "Library", "Services", "Compare with Ayame Diff.workflow", "Contents")
	infoPath, workflowPath := filepath.Join(root, "Info.plist"), filepath.Join(root, "document.wflow")
	info := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.hjosugi.ayame-diff.finder</string>
<key>CFBundleName</key><string>Compare with Ayame Diff</string>
<key>NSServices</key><array><dict>
<key>NSMenuItem</key><dict><key>default</key><string>Compare with Ayame Diff</string></dict>
<key>NSMessage</key><string>runWorkflowAsService</string>
<key>NSSendFileTypes</key><array><string>public.item</string></array>
</dict></array></dict></plist>
`
	script := "exec " + shellQuote(e.Executable) + " --gui \"$@\""
	workflow := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>actions</key><array><dict>
<key>action</key><dict><key>AMAccepts</key><dict><key>Container</key><string>List</string><key>Optional</key><true/><key>Types</key><array><string>com.apple.cocoa.path</string></array></dict>
<key>AMActionVersion</key><string>2.0.3</string><key>AMApplication</key><array><string>Automator</string></array>
<key>AMParameterProperties</key><dict/><key>AMProvides</key><dict><key>Container</key><string>List</string><key>Types</key><array><string>com.apple.cocoa.path</string></array></dict>
<key>ActionBundlePath</key><string>/System/Library/Automator/Run Shell Script.action</string>
<key>ActionName</key><string>Run Shell Script</string><key>ActionParameters</key><dict>
<key>COMMAND_STRING</key><string>%s</string><key>CheckedForUserDefaultShell</key><true/>
<key>inputMethod</key><integer>1</integer><key>shell</key><string>/bin/sh</string></dict>
</dict></dict></array><key>workflowMetaData</key><dict><key>serviceInputTypeIdentifier</key><string>com.apple.Automator.fileSystemObject</string><key>serviceProcessesInput</key><integer>0</integer><key>serviceApplicationBundleID</key><string>com.apple.finder</string></dict></dict></plist>
`, html.EscapeString(script))
	if err := writeFile(infoPath, []byte(info), 0o644); err != nil {
		return nil, err
	}
	if err := writeFile(workflowPath, []byte(workflow), 0o644); err != nil {
		return nil, err
	}
	return []string{filepath.Dir(root)}, nil
}

var windowsKeys = []string{
	`HKCU\Software\Classes\*\shell\AyameDiff`,
	`HKCU\Software\Classes\Directory\shell\AyameDiff`,
}

func installWindows(e Environment) ([]string, error) {
	command := fmt.Sprintf(`"%s" shell-select "%%1"`, strings.ReplaceAll(e.Executable, `"`, `\"`))
	for _, key := range windowsKeys {
		if err := e.runner("reg.exe", "ADD", key, "/ve", "/d", "Compare with Ayame Diff", "/f"); err != nil {
			return nil, err
		}
		if err := e.runner("reg.exe", "ADD", key+`\command`, "/ve", "/d", command, "/f"); err != nil {
			return nil, err
		}
	}
	sendTo := filepath.Join(appData(e), "Microsoft", "Windows", "SendTo", "Compare with Ayame Diff.cmd")
	content := fmt.Sprintf("@echo off\r\n\"%s\" --gui %%*\r\n", strings.ReplaceAll(e.Executable, `"`, `""`))
	if err := writeFile(sendTo, []byte(content), 0o644); err != nil {
		return nil, err
	}
	return []string{sendTo}, nil
}

func appData(e Environment) string {
	if e.AppData != "" {
		return e.AppData
	}
	return filepath.Join(e.Home, "AppData", "Roaming")
}

func writeFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, mode)
}

func removeIfExists(path string) error {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func desktopQuote(value string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(value, `\`, `\\`), `"`, `\"`) + `"`
}
func shellQuote(value string) string { return `'` + strings.ReplaceAll(value, `'`, `'"'"'`) + `'` }

const faviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><g fill="#7A5CC0"><ellipse cx="32" cy="18" rx="8" ry="14"/><ellipse cx="22" cy="29" rx="8" ry="15" transform="rotate(-35 22 29)"/><ellipse cx="42" cy="29" rx="8" ry="15" transform="rotate(35 42 29)"/><ellipse cx="24" cy="43" rx="8" ry="14" transform="rotate(27 24 43)"/><ellipse cx="40" cy="43" rx="8" ry="14" transform="rotate(-27 40 43)"/></g><path d="M26 28l6 4" stroke="#4c9b45" stroke-width="3"/><path d="M38 28l-6 4" stroke="#ca564f" stroke-width="3"/></svg>`
