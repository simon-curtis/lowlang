package editors

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"testing"
)

const grammarSource = "editors/tree-sitter-low/src"

func gitInRepository(args ...string) error {
	return exec.Command("git", append([]string{"-C", ".."}, args...)...).Run()
}

func TestZedRevHoldsTheCurrentGrammar(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
	if err := gitInRepository("rev-parse", "--git-dir"); err != nil {
		t.Skip("not a git checkout")
	}
	manifest, err := os.ReadFile("zed/extension.toml")
	if err != nil {
		t.Fatal(err)
	}
	found := regexp.MustCompile(`(?m)^rev = "([0-9a-f]+)"$`).FindSubmatch(manifest)
	if found == nil {
		t.Fatal("zed/extension.toml has no rev")
	}
	rev := string(found[1])
	if err := gitInRepository("cat-file", "-e", rev+"^{commit}"); err != nil {
		t.Skipf("commit %s is not in this checkout", rev)
	}
	err = gitInRepository("diff", "--quiet", rev, "HEAD", "--", grammarSource)
	var exit *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		t.Errorf("%s differs between rev %s in zed/extension.toml and HEAD; point rev at a commit that holds the current grammar", grammarSource, rev)
	default:
		t.Fatal(err)
	}
}
