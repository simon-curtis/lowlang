package hot

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type fixture struct {
	t       *testing.T
	path    string
	stdout  *syncBuffer
	stderr  *syncBuffer
	session *Session
}

func newFixture(t *testing.T, src string) *fixture {
	t.Helper()
	f := &fixture{t: t, path: filepath.Join(t.TempDir(), "app.low"), stdout: &syncBuffer{}, stderr: &syncBuffer{}}
	f.write(src)
	session, err := New(f.path, f.stdout, f.stderr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	f.session = session
	f.reload()
	return f
}

func writeFile(t *testing.T, path, src string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDynamicReload(t *testing.T) {
	f := newFixture(t, "extern \"c\" {}\n\n"+program("1"))
	f.waitFor(f.stdout, "0\n")
	f.write("extern \"c\" {}\n\n" + program("2"))
	f.reload()
	f.waitFor(f.stdout, "0\n2\ntrue\n")
	f.waitFor(f.stderr, "low: reloaded value in")
	f.noRestart()
}

func TestReloadCallingC(t *testing.T) {
	c := "extern \"c\" {\n    fn labs(n: i64) -> i64\n}\n\n"
	f := newFixture(t, c+program("1"))
	f.waitFor(f.stdout, "0\n")
	f.write(c + program("labs(-6)"))
	f.reload()
	f.waitFor(f.stdout, "0\n6\ntrue\n")
	f.waitFor(f.stderr, "low: reloaded value in")
	f.noRestart()
}

func TestLibraryChangeRestarts(t *testing.T) {
	f := newFixture(t, program("1"))
	f.waitFor(f.stdout, "0\n")
	f.write("extern \"m\" {}\n\n" + program("2"))
	f.reload()
	f.waitFor(f.stderr, "low: restarted: linked libraries changed")
	f.waitFor(f.stdout, "0\n0\n2\n")
}

func TestRunWatchesModules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "low.mod"), "")
	writeFile(t, filepath.Join(dir, "main.low"), "import lib\n\nfn main() {\n    print(0)\n    var n = lib.value()\n    for n == 1 {\n        n = lib.value()\n    }\n    print(n)\n}\n")
	library := filepath.Join(dir, "lib", "lib.low")
	writeFile(t, library, "pub fn value() -> i64 {\n    1\n}\n")
	f := &fixture{t: t, stdout: &syncBuffer{}, stderr: &syncBuffer{}}
	session, err := New(dir, f.stdout, f.stderr)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- session.Run(ctx) }()
	t.Cleanup(func() {
		cancel()
		<-done
		session.Close()
	})
	f.waitFor(f.stdout, "0\n")
	writeFile(t, library, "pub fn value() -> i64 {\n    4\n}\n")
	f.waitFor(f.stdout, "0\n4\n")
	f.waitFor(f.stderr, "low: reloaded lib.value in")
	f.noRestart()
}

func TestReloadOnEmbeddedChange(t *testing.T) {
	f := newFixture(t, "fn value() -> i64 {\n    embed(\"n.txt\").len\n}\n\nfn main() {\n    print(0)\n    var n = value()\n    for n == 1 {\n        n = value()\n    }\n    print(n)\n}\n")
	f.waitFor(f.stderr, "n.txt: no such file")
	data := filepath.Join(filepath.Dir(f.path), "n.txt")
	writeFile(t, data, "a")
	f.reload()
	f.waitFor(f.stdout, "0\n")
	writeFile(t, data, "abc")
	f.reload()
	f.waitFor(f.stdout, "0\n3\n")
	f.waitFor(f.stderr, "low: reloaded value in")
	f.noRestart()
}

func (f *fixture) write(src string) {
	f.t.Helper()
	if err := os.WriteFile(f.path, []byte(src), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) reload() {
	f.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := f.session.Reload(ctx); err != nil {
		f.t.Fatal(err)
	}
}

func (f *fixture) noRestart() {
	f.t.Helper()
	if strings.Contains(f.stderr.String(), "restarted") {
		f.t.Fatalf("program restarted instead of reloading:\n%s", f.stderr)
	}
}

func (f *fixture) waitFor(buf *syncBuffer, want string) {
	f.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(buf.String(), want) {
		if time.Now().After(deadline) {
			f.t.Fatalf("timed out waiting for %q\nstdout:\n%s\nstderr:\n%s", want, f.stdout, f.stderr)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

const spin = `fn value() -> i64 {
    VALUE
}

fn step(n: i64) -> i64 {
    n + value()
}

fn main() {
    print(0)
    var total = 0
    var n = value()
    for n == 1 {
        total = step(total)
        n = value()
    }
    print(n)
    print(total > 0)
}
`

func program(value string) string {
	return strings.ReplaceAll(spin, "VALUE", value)
}

func TestReloadKeepsState(t *testing.T) {
	f := newFixture(t, program("1"))
	f.waitFor(f.stdout, "0\n")
	f.write(program("2"))
	f.reload()
	f.waitFor(f.stdout, "0\n2\ntrue\n")
	f.waitFor(f.stderr, "low: reloaded value in")
	f.noRestart()
}

func TestReloadAddsFunction(t *testing.T) {
	f := newFixture(t, program("1"))
	f.waitFor(f.stdout, "0\n")
	f.write(strings.Replace(program("seven() - 4"), "fn main", "fn seven() -> i64 {\n    7\n}\n\nfn main", 1))
	f.reload()
	f.waitFor(f.stdout, "0\n3\ntrue\n")
	f.waitFor(f.stderr, "low: reloaded seven, value in")
	f.noRestart()
}

func TestCompileErrorKeepsProgramRunning(t *testing.T) {
	f := newFixture(t, program("1"))
	f.waitFor(f.stdout, "0\n")
	f.write(program("undefined_name"))
	f.reload()
	f.waitFor(f.stderr, "undefined: undefined_name")
	f.write(program("5"))
	f.reload()
	f.waitFor(f.stdout, "0\n5\ntrue\n")
	f.waitFor(f.stderr, "low: reloaded value in")
	f.noRestart()
}

const heap = `enum List {
    Cons(i64, *List)
    Nil
}

fn build(n: i64) -> List {
    if n == 0 {
        return List.Nil
    }
    Cons(n, new(build(n - 1)))
}

fn total(l: List) -> i64 {
    match l {
        Cons(v, rest) => v + total(*rest)
        Nil => 0
    }
}

fn done() -> bool {
    DONE
}

fn main() {
    let list = build(100)
    print(0)
    for !done() {
        let junk = new(build(10))
    }
    print(total(list))
}
`

func TestReloadKeepsHeap(t *testing.T) {
	f := newFixture(t, strings.ReplaceAll(heap, "DONE", "false"))
	f.waitFor(f.stdout, "0\n")
	scaled := strings.Replace(heap, "v + total(*rest)", "2 * v + total(*rest)", 1)
	f.write(strings.ReplaceAll(scaled, "DONE", "true"))
	f.reload()
	f.waitFor(f.stdout, "0\n10100\n")
	f.waitFor(f.stderr, "low: reloaded done, total in")
	f.noRestart()
}

const values = `fn value() -> i64 {
    VALUE
}

fn native(n: i64) -> i64 {
    n + VALUE
}

fn main() {
    print(0)
    let named: fn() -> i64 = value
    let inline = fn() -> i64 { VALUE }
    let callback: extern fn(i64) -> i64 = native
    var n = named() + inline() + callback(0)
    for n < 6 {
        n = named() + inline() + callback(0)
    }
    print(n)
}
`

func TestReloadReachesStoredFunctions(t *testing.T) {
	f := newFixture(t, strings.ReplaceAll(values, "VALUE", "1"))
	f.waitFor(f.stdout, "0\n")
	f.write(strings.ReplaceAll(values, "VALUE", "2"))
	f.reload()
	f.waitFor(f.stdout, "0\n6\n")
	f.waitFor(f.stderr, "low: reloaded ")
	for _, name := range []string{"main..c1", "native", "value"} {
		if !strings.Contains(f.stderr.String(), name) {
			t.Fatalf("%s was not reloaded:\n%s", name, f.stderr)
		}
	}
	f.noRestart()
}

func TestCaptureChangeRestarts(t *testing.T) {
	captures := strings.Replace(values, "fn() -> i64 { VALUE }", "fn() -> i64 { one }", 1)
	captures = strings.Replace(captures, "    let named", "    let one = 1\n    let named", 1)
	f := newFixture(t, strings.ReplaceAll(captures, "VALUE", "1"))
	f.waitFor(f.stdout, "0\n")
	f.write(strings.ReplaceAll(values, "VALUE", "2"))
	f.reload()
	f.waitFor(f.stderr, "low: restarted: ")
	f.waitFor(f.stdout, "0\n0\n6\n")
}

func TestLayoutChangeRestarts(t *testing.T) {
	f := newFixture(t, "struct P {\n    x: i64\n}\n\n"+program("1"))
	f.waitFor(f.stdout, "0\n")
	f.write("struct P {\n    x: i64\n    y: i64\n}\n\n" + program("2"))
	f.reload()
	f.waitFor(f.stderr, "low: restarted: type layout changed")
	f.waitFor(f.stdout, "0\n0\n2\n")
}

func TestSignatureChangeRestarts(t *testing.T) {
	f := newFixture(t, program("1"))
	f.waitFor(f.stdout, "0\n")
	f.write(strings.Replace(program("2"), "fn step(n: i64) -> i64 {\n    n + value()", "fn step(n: i64, m: i64) -> i64 {\n    n + m", 1))
	f.write(strings.Replace(readFile(t, f.path), "step(total)", "step(total, 1)", 1))
	f.reload()
	f.waitFor(f.stderr, "low: restarted: ")
	f.waitFor(f.stdout, "0\n0\n2\n")
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(src)
}
