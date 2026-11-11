package oracle

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"lowlang/internal/fuzz/model"
)

const divideProgram = `fn div(a: i64, b: i64) -> i64 {
    a / b
}

fn main() {
    print(div(10, 2))
    print(div(1, 0))
}
`

func requireToolchain(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"as", "ld"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not installed", tool)
		}
	}
}

func TestCheckAcceptsMatchingOutcome(t *testing.T) {
	requireToolchain(t)
	runner := &Runner{Timeout: 10 * time.Second}
	expected := model.Outcome{Stdout: "5\n", Panic: "integer divide by zero"}
	if findings := runner.Check(context.Background(), divideProgram, expected); len(findings) != 0 {
		t.Errorf("unexpected findings: %+v", findings)
	}
}

func TestCheckReportsReferenceMismatch(t *testing.T) {
	requireToolchain(t)
	runner := &Runner{Timeout: 10 * time.Second}
	expected := model.Outcome{Stdout: "6\n", Panic: "integer divide by zero"}
	findings := runner.Check(context.Background(), divideProgram, expected)
	if len(findings) != 2 {
		t.Fatalf("got %d findings, want one per mode: %+v", len(findings), findings)
	}
	for _, f := range findings {
		if f.Class != ClassReference || !strings.Contains(f.Detail, `want "6"`) {
			t.Errorf("finding %+v", f)
		}
	}
}

func TestCheckReportsRejectedProgram(t *testing.T) {
	requireToolchain(t)
	runner := &Runner{Timeout: 10 * time.Second}
	findings := runner.Check(context.Background(), "fn main() {\n    print(missing)\n}\n", model.Outcome{})
	if len(findings) != 2 || findings[0].Class != ClassReject {
		t.Fatalf("findings %+v", findings)
	}
	if strings.Contains(findings[0].Detail, "lowfuzz-") {
		t.Errorf("detail leaks the work directory: %s", findings[0].Detail)
	}
}

func TestSignatureIgnoresNumbers(t *testing.T) {
	a := Finding{Class: ClassReject, Detail: "release: main.low:3:4: 300 overflows u8\nmore"}
	b := Finding{Class: ClassReject, Detail: "release: main.low:90:11: 7 overflows u8"}
	if a.Signature() != b.Signature() {
		t.Errorf("%q != %q", a.Signature(), b.Signature())
	}
	c := Finding{Class: ClassReject, Detail: "release: main.low:3:4: 300 overflows u16"}
	if a.Signature() == c.Signature() {
		t.Error("type names were collapsed")
	}
}
