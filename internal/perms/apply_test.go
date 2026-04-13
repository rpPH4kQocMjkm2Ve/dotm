package perms

import (
	"bytes"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func dirSet(dirs ...string) func(string) bool {
	m := make(map[string]bool, len(dirs))
	for _, d := range dirs {
		m[d] = true
	}
	return func(path string) bool { return m[path] }
}

func skipIfNotRoot(t *testing.T) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("requires root")
	}
}

func fileMode(t *testing.T, path string) int {
	t.Helper()
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return int(st.Mode & 0o7777)
}

func fileStat(t *testing.T, path string) (uid, gid int) {
	t.Helper()
	var st syscall.Stat_t
	if err := syscall.Stat(path, &st); err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return int(st.Uid), int(st.Gid)
}

func getNonRootUser(t *testing.T) (string, int) {
	t.Helper()
	for _, name := range []string{"nobody", "daemon", "bin"} {
		u, err := user.Lookup(name)
		if err != nil {
			continue
		}
		uid, _ := strconv.Atoi(u.Uid)
		if uid != 0 {
			return name, uid
		}
	}
	t.Skip("no non-root user available")
	return "", 0
}

func getNonRootGroup(t *testing.T) (string, int) {
	t.Helper()
	for _, name := range []string{"nobody", "nogroup", "daemon", "bin"} {
		g, err := user.LookupGroup(name)
		if err != nil {
			continue
		}
		gid, _ := strconv.Atoi(g.Gid)
		if gid != 0 {
			return name, gid
		}
	}
	t.Skip("no non-root group available")
	return "", 0
}

func makeAction(path string, mode int, owner, group string) PermAction {
	var modeStr string
	if mode >= 0 {
		modeStr = strconv.FormatInt(int64(mode), 8)
		// Pad to 4 digits.
		for len(modeStr) < 4 {
			modeStr = "0" + modeStr
		}
	}
	return PermAction{
		Path:  path,
		Mode:  mode,
		Owner: owner,
		Group: group,
		Rule: &PermRule{
			Pattern: "**",
			DirOnly: false,
			Mode:    modeStr,
			Owner:   owner,
			Group:   group,
			LineNum: 1,
		},
	}
}

// ─── ComputeActions ──────────────────────────────────────────────────────────

func TestComputeActionsEmptyRules(t *testing.T) {
	actions := ComputeActions(nil, []string{"/etc/foo"}, "/", func(_ string) bool { return false })
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

func TestComputeActionsEmptyManaged(t *testing.T) {
	rules, _ := ParseRules("etc/** 0644 root root\n")
	actions := ComputeActions(rules, nil, "/", func(_ string) bool { return false })
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

func TestComputeActionsSingleFileMatch(t *testing.T) {
	rules, _ := ParseRules("etc/** 0644 root root\n")
	actions := ComputeActions(rules, []string{"/etc/pacman.conf"}, "/",
		func(_ string) bool { return false })

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	a := actions[0]
	assertEqual(t, "path", a.Path, "/etc/pacman.conf")
	assertEqual(t, "mode", a.Mode, 0o644)
	assertEqual(t, "owner", a.Owner, "root")
	assertEqual(t, "group", a.Group, "root")
}

func TestComputeActionsDirRuleSkipsFiles(t *testing.T) {
	rules, _ := ParseRules("etc/**/ 0755 root root\n")
	actions := ComputeActions(rules, []string{"/etc/pacman.conf"}, "/",
		func(_ string) bool { return false })
	if len(actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(actions))
	}
}

func TestComputeActionsDirRuleMatchesDirs(t *testing.T) {
	rules, _ := ParseRules("etc/**/ 0755 root root\n")
	actions := ComputeActions(rules, []string{"/etc/security"}, "/",
		dirSet("/etc/security"))
	if len(actions) != 1 {
		t.Fatalf("expected 1, got %d", len(actions))
	}
	assertEqual(t, "mode", actions[0].Mode, 0o755)
}

func TestComputeActionsFileRuleSkipsDirs(t *testing.T) {
	rules, _ := ParseRules("etc/** 0644 root root\n")
	actions := ComputeActions(rules, []string{"/etc/security"}, "/",
		dirSet("/etc/security"))
	if len(actions) != 0 {
		t.Errorf("expected 0, got %d", len(actions))
	}
}

func TestComputeActionsLastMatchWins(t *testing.T) {
	rules, _ := ParseRules("etc/** 0644 root root\netc/security/** 0600 root root\n")
	actions := ComputeActions(rules,
		[]string{"/etc/security/faillock.conf", "/etc/pacman.conf"}, "/",
		func(_ string) bool { return false })

	byPath := make(map[string]PermAction)
	for _, a := range actions {
		byPath[a.Path] = a
	}
	assertEqual(t, "security mode", byPath["/etc/security/faillock.conf"].Mode, 0o600)
	assertEqual(t, "pacman mode", byPath["/etc/pacman.conf"].Mode, 0o644)
}

func TestComputeActionsSkipMode(t *testing.T) {
	rules, _ := ParseRules("etc/** - root root\n")
	actions := ComputeActions(rules, []string{"/etc/foo"}, "/",
		func(_ string) bool { return false })
	assertEqual(t, "mode", actions[0].Mode, -1)
	assertEqual(t, "owner", actions[0].Owner, "root")
}

func TestComputeActionsNonRootDest(t *testing.T) {
	rules, _ := ParseRules("etc/** 0644 root root\n")
	actions := ComputeActions(rules,
		[]string{"/custom/dest/etc/foo.conf"}, "/custom/dest",
		func(_ string) bool { return false })
	if len(actions) != 1 {
		t.Fatalf("expected 1, got %d", len(actions))
	}
	assertEqual(t, "path", actions[0].Path, "/custom/dest/etc/foo.conf")
}

func TestComputeActionsOutsideDestSkipped(t *testing.T) {
	rules, _ := ParseRules("etc/** 0644 root root\n")
	actions := ComputeActions(rules, []string{"/other/etc/foo"}, "/dest",
		func(_ string) bool { return false })
	if len(actions) != 0 {
		t.Errorf("expected 0, got %d", len(actions))
	}
}

func TestComputeActionsNoMatchingRule(t *testing.T) {
	rules, _ := ParseRules("etc/** 0644 root root\n")
	actions := ComputeActions(rules, []string{"/usr/lib/foo"}, "/",
		func(_ string) bool { return false })
	if len(actions) != 0 {
		t.Errorf("expected 0, got %d", len(actions))
	}
}

func TestComputeActionsMixedFilesAndDirs(t *testing.T) {
	rules, _ := ParseRules("etc/**/ 0755 root root\netc/** 0644 root root\n")
	dirs := dirSet("/etc", "/etc/security")
	managed := []string{"/etc", "/etc/security", "/etc/pacman.conf", "/etc/security/faillock.conf"}
	actions := ComputeActions(rules, managed, "/", dirs)

	byPath := make(map[string]PermAction)
	for _, a := range actions {
		byPath[a.Path] = a
	}
	assertEqual(t, "etc mode", byPath["/etc"].Mode, 0o755)
	assertEqual(t, "security mode", byPath["/etc/security"].Mode, 0o755)
	assertEqual(t, "pacman mode", byPath["/etc/pacman.conf"].Mode, 0o644)
	assertEqual(t, "faillock mode", byPath["/etc/security/faillock.conf"].Mode, 0o644)
}

func TestComputeActionsSpecificOverridesGlob(t *testing.T) {
	rules, _ := ParseRules("etc/** 0644 root root\netc/pacman.conf 0600 root root\n")
	actions := ComputeActions(rules,
		[]string{"/etc/pacman.conf", "/etc/other.conf"}, "/",
		func(_ string) bool { return false })

	byPath := make(map[string]PermAction)
	for _, a := range actions {
		byPath[a.Path] = a
	}
	assertEqual(t, "pacman mode", byPath["/etc/pacman.conf"].Mode, 0o600)
	assertEqual(t, "other mode", byPath["/etc/other.conf"].Mode, 0o644)
}

func TestComputeActionsDestDirSkipped(t *testing.T) {
	rules, _ := ParseRules("** 0644 root root\n")
	actions := ComputeActions(rules, []string{"/"}, "/",
		func(_ string) bool { return true })
	if len(actions) != 0 {
		t.Errorf("expected 0, got %d", len(actions))
	}
}

func TestComputeActionsDeeplyNested(t *testing.T) {
	rules, _ := ParseRules("etc/** 0644 root root\n")
	actions := ComputeActions(rules,
		[]string{"/etc/a/b/c/d/e/f/g.conf"}, "/",
		func(_ string) bool { return false })
	if len(actions) != 1 {
		t.Errorf("expected 1, got %d", len(actions))
	}
}

func TestComputeActionsSpecificGroup(t *testing.T) {
	rules, _ := ParseRules("etc/** 0644 root root\netc/polkit-1/rules.d/** 0750 root polkitd\n")
	actions := ComputeActions(rules,
		[]string{"/etc/polkit-1/rules.d/99-sing-box.rules"}, "/",
		func(_ string) bool { return false })
	assertEqual(t, "mode", actions[0].Mode, 0o750)
	assertEqual(t, "group", actions[0].Group, "polkitd")
}

// ─── ApplyActions — chmod (require root) ─────────────────────────────────────

func TestApplyChmodFile(t *testing.T) {
	skipIfNotRoot(t)
	f := filepath.Join(t.TempDir(), "test.conf")
	if err := os.WriteFile(f, []byte("content"), 0o777); err != nil {
		t.Fatal(err)
	}

	ok, errs := ApplyActions([]PermAction{makeAction(f, 0o644, "", "")}, false)
	if !ok {
		t.Fatalf("errors: %v", errs)
	}
	assertEqual(t, "mode", fileMode(t, f), 0o644)
}

func TestApplyChmodDirectory(t *testing.T) {
	skipIfNotRoot(t)
	d := filepath.Join(t.TempDir(), "subdir")
	if err := os.Mkdir(d, 0o777); err != nil {
		t.Fatal(err)
	}

	ok, errs := ApplyActions([]PermAction{makeAction(d, 0o755, "", "")}, false)
	if !ok {
		t.Fatalf("errors: %v", errs)
	}
	assertEqual(t, "mode", fileMode(t, d), 0o755)
}

func TestApplyChmodRestrictive(t *testing.T) {
	skipIfNotRoot(t)
	f := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(f, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	ok, _ := ApplyActions([]PermAction{makeAction(f, 0o600, "", "")}, false)
	if !ok {
		t.Fatal("expected success")
	}
	assertEqual(t, "mode", fileMode(t, f), 0o600)
}

func TestApplyChmodNonexistent(t *testing.T) {
	skipIfNotRoot(t)
	ok, errs := ApplyActions([]PermAction{
		makeAction(filepath.Join(t.TempDir(), "nonexistent"), 0o644, "", ""),
	}, false)
	if ok {
		t.Fatal("expected failure")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

func TestApplySkipModeNoChmod(t *testing.T) {
	skipIfNotRoot(t)
	f := filepath.Join(t.TempDir(), "test")
	if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f, 0o777); err != nil {
		t.Fatal(err)
	} // explicit chmod — WriteFile is subject to umask

	ok, _ := ApplyActions([]PermAction{makeAction(f, -1, "", "")}, false)
	if !ok {
		t.Fatal("expected success")
	}
	assertEqual(t, "mode unchanged", fileMode(t, f), 0o777)
}

// ─── ApplyActions — chown (require root) ─────────────────────────────────────

func TestApplyChownToRoot(t *testing.T) {
	skipIfNotRoot(t)
	f := filepath.Join(t.TempDir(), "test")
	if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	ok, errs := ApplyActions([]PermAction{makeAction(f, -1, "root", "root")}, false)
	if !ok {
		t.Fatalf("errors: %v", errs)
	}
	uid, gid := fileStat(t, f)
	assertEqual(t, "uid", uid, 0)
	assertEqual(t, "gid", gid, 0)
}

func TestApplyChownNonRootUser(t *testing.T) {
	skipIfNotRoot(t)
	name, expectedUID := getNonRootUser(t)
	f := filepath.Join(t.TempDir(), "test")
	if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	ok, errs := ApplyActions([]PermAction{makeAction(f, -1, name, "")}, false)
	if !ok {
		t.Fatalf("errors: %v", errs)
	}
	uid, _ := fileStat(t, f)
	assertEqual(t, "uid", uid, expectedUID)
}

func TestApplyChownGroupOnly(t *testing.T) {
	skipIfNotRoot(t)
	name, expectedGID := getNonRootGroup(t)
	f := filepath.Join(t.TempDir(), "test")
	if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	ok, errs := ApplyActions([]PermAction{makeAction(f, -1, "", name)}, false)
	if !ok {
		t.Fatalf("errors: %v", errs)
	}
	_, gid := fileStat(t, f)
	assertEqual(t, "gid", gid, expectedGID)
}

func TestApplyChownNonexistentUser(t *testing.T) {
	skipIfNotRoot(t)
	f := filepath.Join(t.TempDir(), "test")
	if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	ok, errs := ApplyActions([]PermAction{
		makeAction(f, -1, "nonexistent_user_xyz_12345", ""),
	}, false)
	if ok {
		t.Fatal("expected failure")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

// ─── Combined + dry run ──────────────────────────────────────────────────────

func TestApplyChmodAndChown(t *testing.T) {
	skipIfNotRoot(t)
	f := filepath.Join(t.TempDir(), "test")
	if err := os.WriteFile(f, []byte(""), 0o777); err != nil {
		t.Fatal(err)
	}

	ok, errs := ApplyActions([]PermAction{makeAction(f, 0o600, "root", "root")}, false)
	if !ok {
		t.Fatalf("errors: %v", errs)
	}
	assertEqual(t, "mode", fileMode(t, f), 0o600)
	uid, gid := fileStat(t, f)
	assertEqual(t, "uid", uid, 0)
	assertEqual(t, "gid", gid, 0)
}

func TestApplyPartialFailure(t *testing.T) {
	skipIfNotRoot(t)
	tmp := t.TempDir()
	good := filepath.Join(tmp, "good")
	if err := os.WriteFile(good, []byte(""), 0o777); err != nil {
		t.Fatal(err)
	}

	actions := []PermAction{
		makeAction(filepath.Join(tmp, "nonexistent"), 0o644, "", ""),
		makeAction(good, 0o600, "", ""),
	}

	ok, errs := ApplyActions(actions, false)
	if ok {
		t.Fatal("expected failure")
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	// Second action still applied.
	assertEqual(t, "good mode", fileMode(t, good), 0o600)
}

func TestApplyDryRunNoChanges(t *testing.T) {
	skipIfNotRoot(t)
	f := filepath.Join(t.TempDir(), "test")
	if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(f, 0o777); err != nil {
		t.Fatal(err)
	} // explicit chmod — WriteFile is subject to umask

	ok, _ := ApplyActions([]PermAction{makeAction(f, 0o644, "root", "root")}, true)
	if !ok {
		t.Fatal("expected success")
	}
	// File unchanged.
	assertEqual(t, "mode unchanged", fileMode(t, f), 0o777)
}

func TestApplyDryRunNoRoot(t *testing.T) {
	// Dry run does not require root.
	actions := []PermAction{
		{Path: "/some/file.conf", Mode: 0o644, Owner: "root", Group: "root"},
		{Path: "/some/dir/", Mode: 0o755, Owner: "-", Group: "-"},
	}
	ok, errs := ApplyActions(actions, true)
	if !ok {
		t.Fatalf("dry run should succeed: %v", errs)
	}
}

func TestApplyActionsEmptyActions(t *testing.T) {
	ok, errs := ApplyActions(nil, false)
	if !ok {
		t.Fatalf("expected success for empty actions: %v", errs)
	}
}

// ─── Full pipeline ───────────────────────────────────────────────────────────

func TestFullPipeline(t *testing.T) {
	skipIfNotRoot(t)
	root := t.TempDir()

	// Build tree.
	dirs := []string{
		"etc", "etc/security", "etc/polkit-1", "etc/polkit-1/rules.d",
		"etc/systemd", "etc/systemd/network", "efi", "efi/loader", "root",
	}
	files := []string{
		"etc/pacman.conf", "etc/mkinitcpio.conf", "etc/security/faillock.conf",
		"etc/polkit-1/rules.d/99-sing-box.rules", "etc/systemd/network/10-wire.network",
		"etc/systemd/coredump.conf", "efi/loader/loader.conf", "root/.zshrc",
	}

	var managed []string
	for _, d := range dirs {
		p := filepath.Join(root, d)
		if err := os.MkdirAll(p, 0o777); err != nil {
			t.Fatal(err)
		}
		managed = append(managed, p)
	}
	for _, f := range files {
		p := filepath.Join(root, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o777); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(""), 0o777); err != nil {
			t.Fatal(err)
		}
		managed = append(managed, p)
	}

	content := `# Directories
efi/**/                   0755  root  root
etc/**/                   0755  root  root
etc/security/             0700  root  root
root/**/                  0700  root  root

# Files — base
efi/**                    0644  root  root
etc/**                    0644  root  root
root/**                   0600  root  root

# Executables
efi/loader/loader.conf    0755  root  root

# Sensitive
etc/security/**           0600  root  root
etc/polkit-1/rules.d/**   0640  root  root
`
	rules, err := ParseRules(content)
	if err != nil {
		t.Fatal(err)
	}
	actions := ComputeActions(rules, managed, root, nil)
	ok, errs := ApplyActions(actions, false)
	if !ok {
		t.Fatalf("apply errors: %v", errs)
	}

	mode := func(rel string) int { return fileMode(t, filepath.Join(root, rel)) }

	// Directories.
	assertEqual(t, "etc", mode("etc"), 0o755)
	assertEqual(t, "etc/security", mode("etc/security"), 0o700)
	assertEqual(t, "etc/polkit-1", mode("etc/polkit-1"), 0o755)
	assertEqual(t, "etc/polkit-1/rules.d", mode("etc/polkit-1/rules.d"), 0o755)
	assertEqual(t, "root", mode("root"), 0o700)
	assertEqual(t, "efi", mode("efi"), 0o755)

	// Regular files.
	assertEqual(t, "pacman", mode("etc/pacman.conf"), 0o644)
	assertEqual(t, "mkinitcpio", mode("etc/mkinitcpio.conf"), 0o644)
	assertEqual(t, "coredump", mode("etc/systemd/coredump.conf"), 0o644)

	// Sensitive.
	assertEqual(t, "faillock", mode("etc/security/faillock.conf"), 0o600)

	// Polkit.
	assertEqual(t, "polkit", mode("etc/polkit-1/rules.d/99-sing-box.rules"), 0o640)

	// Executable.
	assertEqual(t, "loader", mode("efi/loader/loader.conf"), 0o755)

	// Root home.
	assertEqual(t, "zshrc", mode("root/.zshrc"), 0o600)
}

func TestFullPipelineIdempotent(t *testing.T) {
	skipIfNotRoot(t)
	root := t.TempDir()

	for _, d := range []string{"etc", "etc/security"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	for _, f := range []string{"etc/pacman.conf", "etc/security/faillock.conf"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte(""), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	managed := []string{
		filepath.Join(root, "etc"),
		filepath.Join(root, "etc/security"),
		filepath.Join(root, "etc/pacman.conf"),
		filepath.Join(root, "etc/security/faillock.conf"),
	}

	rules, _ := ParseRules("etc/**/ 0755 root root\netc/** 0644 root root\n")

	// First apply.
	actions := ComputeActions(rules, managed, root, nil)
	ApplyActions(actions, false)

	state1 := make(map[string]int)
	for _, p := range managed {
		state1[p] = fileMode(t, p)
	}

	// Second apply.
	actions = ComputeActions(rules, managed, root, nil)
	ApplyActions(actions, false)

	for _, p := range managed {
		assertEqual(t, "idempotent "+p, fileMode(t, p), state1[p])
	}
}

// ─── PAM safety regression ──────────────────────────────────────────────────

const othersRead = 0o004

func TestPamRestrictiveModeBlocksPam(t *testing.T) {
	rules, _ := ParseRules("etc/** 0644 root root\netc/security/** 0600 root root\n")
	actions := ComputeActions(rules,
		[]string{"/etc/security/faillock.conf"}, "/",
		func(_ string) bool { return false })

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	if actions[0].Mode&othersRead != 0 {
		t.Error("expected no others-read bit on 0600")
	}
}

func TestPamWorldReadableAllowsPam(t *testing.T) {
	rules, _ := ParseRules("etc/** 0644 root root\netc/security/** 0644 root root\n")
	actions := ComputeActions(rules,
		[]string{"/etc/security/faillock.conf"}, "/",
		func(_ string) bool { return false })

	if actions[0].Mode&othersRead == 0 {
		t.Error("expected others-read bit on 0644")
	}
}

func TestPamLastMatchWinsOverridesSafe(t *testing.T) {
	rules, _ := ParseRules(
		"etc/** 0644 root root\netc/**/ 0755 root root\n" +
			"etc/security/** 0600 root root\netc/pacman.conf 0644 root root\n")
	actions := ComputeActions(rules,
		[]string{"/etc/security/faillock.conf", "/etc/pacman.conf"}, "/",
		func(_ string) bool { return false })

	byPath := make(map[string]PermAction)
	for _, a := range actions {
		byPath[a.Path] = a
	}
	if byPath["/etc/pacman.conf"].Mode&othersRead == 0 {
		t.Error("pacman.conf should be world-readable")
	}
	if byPath["/etc/security/faillock.conf"].Mode&othersRead != 0 {
		t.Error("faillock.conf with 0600 should NOT be world-readable")
	}
}

// ─── isDir ──────────────────────────────────────────────────────────────────

func TestIsDirWithRealFilesystem(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if !isDir(dir) {
		t.Error("expected TempDir to be a directory")
	}
	if isDir(filePath) {
		t.Error("expected file to not be a directory")
	}
	if isDir(filepath.Join(dir, "nonexistent")) {
		t.Error("expected nonexistent path to not be a directory")
	}
}

func TestComputeActionsWithRealIsDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	rules, _ := ParseRules("** 0644 - -")
	managed := []string{
		filepath.Join(dir, "file.txt"),
	}

	// Use nil isDirFunc to trigger real isDir.
	actions := ComputeActions(rules, managed, dir, nil)

	// Should have action for the file.
	if len(actions) != 1 {
		t.Errorf("expected 1 action, got %d", len(actions))
	}
}

// ─── ApplyActions — dry run output verification ─────────────────────────────

func TestApplyDryRunOutput(t *testing.T) {
	actions := []PermAction{
		{Path: "/some/file.conf", Mode: 0o644, Owner: "root", Group: "root"},
		{Path: "/some/dir/", Mode: -1, Owner: "root", Group: "-"},
		{Path: "/some/other.conf", Mode: -1, Owner: "-", Group: "root"},
	}

	// Capture output.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	ok, errs := ApplyActions(actions, true)
	if err := w.Close(); err != nil {
		t.Fatalf("w.Close: %v", err)
	}

	if !ok {
		t.Fatalf("dry run should succeed: %v", errs)
	}

	// Read captured output.
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatalf("buf.ReadFrom: %v", err)
	}
	output := buf.String()

	// Verify output contains expected values.
	if !strings.Contains(output, "0644") {
		t.Error("expected mode 0644 in dry run output")
	}
	if !strings.Contains(output, "root:root") {
		t.Error("expected root:root in dry run output")
	}
	if !strings.Contains(output, "-:root") {
		t.Error("expected -:root in dry run output")
	}
	if !strings.Contains(output, "root:-") {
		t.Error("expected root:- in dry run output")
	}
}

// ─── ApplyActions — chown with both owner and group ─────────────────────────

func TestApplyChownBothOwnerAndGroup(t *testing.T) {
	skipIfNotRoot(t)
	f := filepath.Join(t.TempDir(), "test")
	if err := os.WriteFile(f, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	ok, errs := ApplyActions([]PermAction{makeAction(f, -1, "root", "root")}, false)
	if !ok {
		t.Fatalf("errors: %v", errs)
	}
	uid, gid := fileStat(t, f)
	assertEqual(t, "uid", uid, 0)
	assertEqual(t, "gid", gid, 0)
}

// ─── ApplyActions — mode parsing edge cases ─────────────────────────────────

func TestComputeActionsInvalidModeParsing(t *testing.T) {
	// Rule with invalid mode (not octal) — should still compute actions
	// but Mode should be -1 after parsing fails.
	rules := []PermRule{
		{
			Pattern: "**",
			DirOnly: false,
			Mode:    "9999", // invalid octal but ParseRules would reject this
			Owner:   "root",
			Group:   "root",
		},
	}

	// Manually create rule with invalid mode to test ComputeActions parsing.
	rules[0].Mode = "invalid"
	actions := ComputeActions(rules, []string{"/some/file"}, "/",
		func(_ string) bool { return false })

	if len(actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(actions))
	}
	// Mode should be -1 since "invalid" can't be parsed as octal.
	if actions[0].Mode != -1 {
		t.Errorf("expected mode -1 for invalid mode string, got %d", actions[0].Mode)
	}
}
