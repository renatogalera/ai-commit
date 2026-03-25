package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", dir)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestHookScript(t *testing.T) {
	t.Parallel()
	script := HookScript()

	if !strings.Contains(script, hookMarker) {
		t.Error("script should contain the hook marker")
	}
	if !strings.Contains(script, "--msg-only") {
		t.Error("script should use --msg-only flag")
	}
	if !strings.Contains(script, "COMMIT_MSG_FILE") {
		t.Error("script should reference COMMIT_MSG_FILE")
	}
	if !strings.Contains(script, "COMMIT_SOURCE") {
		t.Error("script should check COMMIT_SOURCE")
	}
	if !strings.HasPrefix(script, "#!/bin/sh") {
		t.Error("script should start with shebang")
	}
}

func TestInstallAndUninstall(t *testing.T) {
	dir := initTestRepo(t)
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Initially not installed
	installed, err := IsInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if installed {
		t.Error("expected not installed initially")
	}

	// Install
	if err := Install(false); err != nil {
		t.Fatal(err)
	}

	// Verify installed
	installed, err = IsInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if !installed {
		t.Error("expected installed after Install()")
	}

	// Verify file permissions
	path, _ := HookPath()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Error("hook should be executable")
	}

	// Install again (idempotent)
	if err := Install(false); err != nil {
		t.Error("reinstalling ai-commit hook should succeed")
	}

	// Uninstall
	if err := Uninstall(); err != nil {
		t.Fatal(err)
	}

	installed, err = IsInstalled()
	if err != nil {
		t.Fatal(err)
	}
	if installed {
		t.Error("expected not installed after Uninstall()")
	}
}

func TestInstallOverThirdPartyHook(t *testing.T) {
	dir := initTestRepo(t)
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create a third-party hook
	path, _ := HookPath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("#!/bin/sh\necho third-party\n"), 0o755)

	// Install without force should fail
	err := Install(false)
	if err == nil {
		t.Error("expected error when overwriting third-party hook without force")
	}

	// Third-party detection
	isTP, _ := ExistingHookIsThirdParty()
	if !isTP {
		t.Error("expected third-party detection")
	}

	// Install with force should succeed
	if err := Install(true); err != nil {
		t.Fatal(err)
	}

	installed, _ := IsInstalled()
	if !installed {
		t.Error("expected installed after forced install")
	}
}

func TestUninstallThirdPartyHookRefused(t *testing.T) {
	dir := initTestRepo(t)
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	// Create a third-party hook
	path, _ := HookPath()
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte("#!/bin/sh\necho other\n"), 0o755)

	err := Uninstall()
	if err == nil {
		t.Error("expected error when uninstalling third-party hook")
	}
}

func TestUninstallNoHook(t *testing.T) {
	dir := initTestRepo(t)
	origDir, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(origDir)

	err := Uninstall()
	if err == nil {
		t.Error("expected error when no hook exists")
	}
}
