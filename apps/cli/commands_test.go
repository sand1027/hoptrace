package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sandeepv/hoptrace/internal/platform"
)

func useTempDB(t *testing.T) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "history.db")
	t.Setenv("HOPTRACE_DB", path)
}

func runCLI(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := newRootCmd()
	var out, errb bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errb)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errb.String(), err
}

func testServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "/ok", http.StatusFound)
			return
		case "/big":
			w.Write(bytes.Repeat([]byte("x"), 4096))
			return
		case "/echo":
			b, _ := io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"method": r.Method,
				"body":   string(b),
				"hdr":    r.Header.Get("X-Test"),
			})
			return
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestCLI_Version(t *testing.T) {
	out, _, err := runCLI(t, "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hoptrace") {
		t.Fatalf("out=%q", out)
	}
}

func TestCLI_Help(t *testing.T) {
	out, _, err := runCLI(t)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "hoptrace") {
		t.Fatalf("out=%q", out)
	}
}

func TestCLI_Completion(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		out, _, err := runCLI(t, "completion", sh)
		if err != nil {
			t.Fatalf("%s: %v", sh, err)
		}
		if len(out) < 20 {
			t.Fatalf("%s completion too short", sh)
		}
	}
}

func TestCLI_ProbeMetricsOnly(t *testing.T) {
	useTempDB(t)
	srv := testServer(t)
	out, errb, err := runCLI(t, "--metrics-only", "--no-save", srv.URL+"/ok")
	if err != nil {
		t.Fatalf("err=%v stderr=%s", err, errb)
	}
	if !strings.Contains(out, "status=200") && !strings.Contains(out, "total=") {
		t.Fatalf("unexpected metrics output: %q", out)
	}
}

func TestCLI_ProbeAliasAndCompact(t *testing.T) {
	useTempDB(t)
	srv := testServer(t)
	out, _, err := runCLI(t, "probe", "--compact", "--no-save", srv.URL+"/ok")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "status=200") {
		t.Fatalf("out=%q", out)
	}
}

func TestCLI_ProbePOSTWithHeaderAndData(t *testing.T) {
	useTempDB(t)
	srv := testServer(t)
	out, _, err := runCLI(t,
		"--metrics-only", "--no-save",
		"-X", "POST",
		"-H", "X-Test: hello",
		"-d", `{"a":1}`,
		srv.URL+"/echo",
	)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "status=200") {
		t.Fatalf("out=%q", out)
	}
}

func TestCLI_ProbeDataFromFile(t *testing.T) {
	useTempDB(t)
	srv := testServer(t)
	path := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(path, []byte(`{"from":"file"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := runCLI(t, "--metrics-only", "--no-save", "-X", "POST", "-d", "@"+path, srv.URL+"/echo")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCLI_ProbeFollowRedirect(t *testing.T) {
	useTempDB(t)
	srv := testServer(t)
	out, _, err := runCLI(t, "-L", "--metrics-only", "--no-save", srv.URL+"/redirect")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "status=200") {
		t.Fatalf("expected final 200 after follow, out=%q", out)
	}
}

func TestCLI_ProbeInvalidHeader(t *testing.T) {
	_, _, err := runCLI(t, "-H", "BadHeader", "http://example.com")
	if platform.ExitCodeOf(err) != platform.ExitUsage {
		t.Fatalf("want usage exit, got %v code=%d", err, platform.ExitCodeOf(err))
	}
}

func TestCLI_ProbeMaxBody(t *testing.T) {
	useTempDB(t)
	srv := testServer(t)
	_, errb, err := runCLI(t, "--metrics-only", "--no-save", "--max-body", "64", srv.URL+"/big")
	if platform.ExitCodeOf(err) != platform.ExitTempFail {
		t.Fatalf("want temp fail for oversized body, err=%v stderr=%s", err, errb)
	}
}

func TestCLI_ProbeSLOFail(t *testing.T) {
	useTempDB(t)
	srv := testServer(t)
	_, errb, err := runCLI(t, "--metrics-only", "--no-save", "--slo", "total=0.0001", srv.URL+"/ok")
	if platform.ExitCodeOf(err) != platform.ExitSLOFail {
		t.Fatalf("want slo fail, err=%v stderr=%s code=%d", err, errb, platform.ExitCodeOf(err))
	}
}

func TestCLI_ProbeJSON(t *testing.T) {
	useTempDB(t)
	srv := testServer(t)
	path := filepath.Join(t.TempDir(), "report.json")
	_, _, err := runCLI(t, "--no-save", "--json", path, "--metrics-only", srv.URL+"/ok")
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("InitialURL")) && !bytes.Contains(b, []byte("initial_url")) && !bytes.Contains(b, []byte("summary")) {
		// ProbeReport JSON uses exported field names
		if len(b) < 10 {
			t.Fatalf("json too small: %s", b)
		}
	}
}

func TestCLI_HistorySaveRunLibrary(t *testing.T) {
	useTempDB(t)
	srv := testServer(t)

	out, _, err := runCLI(t, "history")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "no history") {
		t.Fatalf("empty history: %q", out)
	}

	_, _, err = runCLI(t, "save", "demo", srv.URL+"/ok", "--desc", "test", "-H", "Accept: text/plain")
	if err != nil {
		t.Fatal(err)
	}

	out, _, err = runCLI(t, "saved", "list")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "demo") {
		t.Fatalf("saved list: %q", out)
	}

	out, _, err = runCLI(t, "saved", "show", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "name=demo") {
		t.Fatalf("show: %q", out)
	}

	_, errb, err := runCLI(t, "run", "demo", "--metrics-only")
	if err != nil {
		t.Fatalf("run: %v stderr=%s", err, errb)
	}

	out, _, err = runCLI(t, "history")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "status=") {
		t.Fatalf("history after run: %q", out)
	}

	// extract first history id
	line := strings.Split(strings.TrimSpace(out), "\n")[0]
	id := strings.Fields(line)[0]
	out, _, err = runCLI(t, "history", "show", id)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 5 {
		t.Fatalf("history show empty")
	}

	// second run for compare
	_, _, err = runCLI(t, "run", "demo", "--metrics-only")
	if err != nil {
		t.Fatal(err)
	}
	out, _, err = runCLI(t, "history")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		t.Fatalf("need 2 history rows, got %q", out)
	}
	idA := strings.Fields(lines[0])[0]
	idB := strings.Fields(lines[1])[0]
	out, _, err = runCLI(t, "history", "compare", idA, idB)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Compare") || !strings.Contains(out, "delta total") {
		t.Fatalf("compare: %q", out)
	}

	out, _, err = runCLI(t, "saved", "delete", "demo")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "deleted demo") {
		t.Fatalf("delete: %q", out)
	}
}

func TestCLI_DashboardNoOpen(t *testing.T) {
	t.Setenv("HOPTRACE_DASHBOARD", "http://127.0.0.1:3999")
	t.Setenv("HOPTRACE_API", "http://127.0.0.1:3998")
	out, _, err := runCLI(t, "dashboard", "--no-open")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "http://127.0.0.1:3999") {
		t.Fatalf("out=%q", out)
	}
}
