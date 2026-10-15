package procwatch

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

type fake struct {
	names []string
	err   error
}

func (f fake) Names() ([]string, error) { return f.names, f.err }

func TestIsQuaver(t *testing.T) {
	for name, want := range map[string]bool{
		"Quaver.exe": true, "quaver.EXE": true, "Quaver": true, "quaver": true,
		"": false, "Quaver.exe.bak": false, "QuaverStreamCompanion.exe": false, "qsc.exe": false, " Quaver": false,
	} {
		if got := IsQuaver(name); got != want {
			t.Errorf("IsQuaver(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestRunning(t *testing.T) {
	boom := errors.New("boom")
	cases := []struct {
		name  string
		l     Lister
		want  bool
		isErr bool
	}{
		{"found", fake{names: []string{"explorer.exe", "quaver.exe"}}, true, false},
		{"absent", fake{names: []string{"explorer.exe"}}, false, false},
		{"empty list", fake{}, false, false},
		{"error", fake{err: boom}, false, true},
		{"error with names", fake{names: []string{"Quaver.exe"}, err: boom}, false, true},
	}
	for _, c := range cases {
		got, err := Running(c.l)
		if got != c.want || (err != nil) != c.isErr {
			t.Errorf("%s: got %v, %v", c.name, got, err)
		}
	}
}

func proc(t *testing.T, root, pid, comm, cmdline string) {
	t.Helper()
	dir := filepath.Join(root, pid)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if comm != "" {
		if err := os.WriteFile(filepath.Join(dir, "comm"), []byte(comm+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "cmdline"), []byte(cmdline), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProcFSNativeAndProton(t *testing.T) {
	root := t.TempDir()
	proc(t, root, "100", "bash", "bash\x00-c\x00echo Quaver.exe")
	proc(t, root, "200", "Quaver", "/opt/quaver/Quaver")
	if err := os.MkdirAll(filepath.Join(root, "self"), 0o755); err != nil {
		t.Fatal(err)
	}
	names, err := (ProcFS{Root: root}).Names()
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(names)
	if !slices.Equal(names, []string{"Quaver", "bash"}) {
		t.Fatalf("names = %v", names)
	}
	if ok, _ := Running(ProcFS{Root: root}); !ok {
		t.Fatal("native Quaver not found")
	}

	proton := t.TempDir()
	bs := string(rune(92))
	proc(t, proton, "300", "wine64-preloader", "Z:"+bs+"games"+bs+"Quaver"+bs+"Quaver.exe\x00")
	proc(t, proton, "301", "wineserver", "wineserver\x00-p")
	proc(t, proton, "302", "bash", "bash\x00/tmp/Quaver.exe")
	if ok, err := Running(ProcFS{Root: proton}); err != nil || !ok {
		t.Fatalf("proton Quaver.exe: %v, %v", ok, err)
	}
	names, _ = (ProcFS{Root: proton}).Names()
	slices.Sort(names)
	if !slices.Equal(names, []string{"Quaver.exe", "bash", "wineserver"}) {
		t.Fatalf("names = %v", names)
	}
}

func TestProcFSIgnoresCmdlineOfOtherPrograms(t *testing.T) {
	root := t.TempDir()
	proc(t, root, "1", "bash", "bash\x00Quaver.exe")
	proc(t, root, "2", "wineserver", "wineserver\x00-p")
	if ok, err := Running(ProcFS{Root: root}); err != nil || ok {
		t.Fatalf("got %v, %v", ok, err)
	}
}

func TestProcFSSkipsVanishedProcesses(t *testing.T) {
	root := t.TempDir()
	proc(t, root, "10", "", "")
	proc(t, root, "11", "Quaver.exe", "")
	if ok, err := Running(ProcFS{Root: root}); err != nil || !ok {
		t.Fatalf("got %v, %v", ok, err)
	}
}

func TestProcFSErrorsWhenNothingIsVisible(t *testing.T) {
	if _, err := (ProcFS{Root: t.TempDir()}).Names(); err == nil {
		t.Fatal("empty tree must be an error, never an absence")
	}
	if _, err := (ProcFS{Root: filepath.Join(t.TempDir(), "missing")}).Names(); err == nil {
		t.Fatal("missing root must be an error")
	}
}

func TestProcFSBoundsCmdline(t *testing.T) {
	root := t.TempDir()
	big := make([]byte, 1<<20)
	copy(big[len(big)-12:], "Quaver.exe\x00")
	proc(t, root, "5", "wine64", string(big))
	proc(t, root, "6", "wine64", "Quaver.exe")
	names, err := (ProcFS{Root: root}).Names()
	slices.Sort(names)
	if err != nil || !slices.Equal(names, []string{"Quaver.exe", "wine64"}) {
		t.Fatalf("%v %v", names, err)
	}
}

func stat(t *testing.T, root, pid, comm, state string) {
	t.Helper()
	line := pid + " (" + comm + ") " + state + " 1 1 1 0 -1 4194560\n"
	if err := os.WriteFile(filepath.Join(root, pid, "stat"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestProcFSSkipsZombies(t *testing.T) {
	root := t.TempDir()
	proc(t, root, "1", "Quaver", "")
	stat(t, root, "1", "Quaver", "Z")
	proc(t, root, "2", "bash", "")
	stat(t, root, "2", "bash", "S")
	if ok, err := Running(ProcFS{Root: root}); err != nil || ok {
		t.Fatalf("a zombie must not count as running: %v, %v", ok, err)
	}
	proc(t, root, "3", "Quaver", "")
	stat(t, root, "3", "Qua) Z ver", "S")
	if ok, err := Running(ProcFS{Root: root}); err != nil || !ok {
		t.Fatalf("a live Quaver was missed: %v, %v", ok, err)
	}
}

func TestProcFSErrorsWhenNoNameCanBeRead(t *testing.T) {
	root := t.TempDir()
	proc(t, root, "1", "", "")
	proc(t, root, "2", "", "")
	if names, err := (ProcFS{Root: root}).Names(); err == nil {
		t.Fatalf("names = %v: unreadable names must be an error, never an absence", names)
	}
	proc(t, root, "3", "bash", "")
	stat(t, root, "3", "bash", "Z")
	if _, err := (ProcFS{Root: root}).Names(); err == nil {
		t.Fatal("only zombies and unreadable processes: must be an error")
	}
}
