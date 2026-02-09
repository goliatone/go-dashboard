package admininterop

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestExternalModuleCompile(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to resolve current file path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", ".."))

	moduleDir := t.TempDir()
	goMod := "module example.com/dashboardinteroptest\n\n" +
		"go 1.24.0\n\n" +
		"require github.com/goliatone/go-dashboard v0.0.0\n\n" +
		"replace github.com/goliatone/go-dashboard => " + repoRoot + "\n"
	if err := os.WriteFile(filepath.Join(moduleDir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	testSource := `package externaltest

import (
	"context"
	"testing"

	dashboardactivity "github.com/goliatone/go-dashboard/pkg/activity"
	"github.com/goliatone/go-dashboard/pkg/activity/admininterop"
)

func TestInteropCompiles(t *testing.T) {
	typ, id, ok := dashboardactivity.ParseCompositeObject("user:abc")
	if !ok || typ != "user" || id != "abc" {
		t.Fatalf("unexpected parser output: %q %q %v", typ, id, ok)
	}

	sink := admininterop.NewSinkFunc(dashboardactivity.Hooks{}, dashboardactivity.Config{})
	if err := sink.Record(context.Background(), admininterop.Record{
		Actor:  "actor-1",
		Action: "update",
		Object: "user:abc",
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
}
`
	if err := os.WriteFile(filepath.Join(moduleDir, "interop_test.go"), []byte(testSource), 0o644); err != nil {
		t.Fatalf("write interop test: %v", err)
	}

	cmd := exec.Command("go", "test", "-mod=readonly", "./...")
	cmd.Dir = moduleDir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("external go test failed: %v\n%s", err, strings.TrimSpace(string(output)))
	}
}
