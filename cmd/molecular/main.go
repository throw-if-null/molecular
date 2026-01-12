package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/version"
)

func main() {
	client := &http.Client{Timeout: 30 * time.Second}
	baseURL := fmt.Sprintf("http://%s:%d", api.DefaultHost, api.DefaultPort)
	os.Exit(run(os.Args[1:], client, baseURL, os.Stdout, os.Stderr))
}

// execLookPath is a variable to allow tests to stub out LookPath.
var execLookPath = func(name string) (string, error) { return exec.LookPath(name) }

func usage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage:")
	_, _ = fmt.Fprintln(w, "  molecular submit --task-id <id> --prompt <text>")
	_, _ = fmt.Fprintln(w, "  molecular status <task-id>")
	_, _ = fmt.Fprintln(w, "  molecular list [--limit N]")
	_, _ = fmt.Fprintln(w, "  molecular cancel <task-id>")
	_, _ = fmt.Fprintln(w, "  molecular logs <task-id> [--tail N]")
	_, _ = fmt.Fprintln(w, "  molecular cleanup <task-id>")
	_, _ = fmt.Fprintln(w, "  molecular version")
	_, _ = fmt.Fprintln(w, "  molecular doctor [--json]")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "doctor checks:")
	_, _ = fmt.Fprintln(w, "  - git in PATH (required)")
	_, _ = fmt.Fprintln(w, "  - gh in PATH (optional)")
	_, _ = fmt.Fprintln(w, "  - .molecular/config.toml exists")
	_, _ = fmt.Fprintln(w, "  - .molecular/lithium.sh and .molecular/chlorine.sh exist + executable")
	_, _ = fmt.Fprintln(w, "")
	_, _ = fmt.Fprintln(w, "exit codes:")
	_, _ = fmt.Fprintln(w, "  0: ok")
	_, _ = fmt.Fprintln(w, "  1: problems found")
	_, _ = fmt.Fprintln(w, "  2: usage error")
}

// run executes the CLI logic and returns an exit code.
func run(args []string, client *http.Client, baseURL string, out io.Writer, errOut io.Writer) int {
	if len(args) < 1 {
		usage(errOut)
		return 2
	}
	switch args[0] {
	case "submit":
		return submitWithClient(args[1:], client, baseURL, out, errOut)
	case "status":
		return statusWithClient(args[1:], client, baseURL, out, errOut)
	case "list":
		return listWithClient(args[1:], client, baseURL, out, errOut)
	case "cancel":
		return cancelWithClient(args[1:], client, baseURL, out, errOut)
	case "logs":
		return logsWithClient(args[1:], client, baseURL, out, errOut)
	case "cleanup":
		return cleanupWithClient(args[1:], client, baseURL, out, errOut)
	case "version":
		fmt.Fprintf(out, "molecular %s (%s)\n", version.Version, version.Commit)
		return 0
	case "doctor":
		return doctorWithIO(args[1:], out, errOut)
	default:
		usage(errOut)
		return 2
	}
}

// submitWithClient implements submit using provided http client/baseURL.
func submitWithClient(args []string, client *http.Client, baseURL string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("submit", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var taskID string
	var prompt string
	fs.StringVar(&taskID, "task-id", "", "task id")
	fs.StringVar(&prompt, "prompt", "", "task prompt")
	_ = fs.Parse(args)

	if taskID == "" || prompt == "" {
		fs.Usage()
		return 2
	}

	req := api.CreateTaskRequest{TaskID: taskID, Prompt: prompt}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&req); err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}

	resp, err := client.Post(baseURL+"/v1/tasks", "application/json", &buf)
	if err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}
	if resp.StatusCode >= 400 {
		fmt.Fprintln(errOut, fmt.Errorf("request failed: %s: %s", resp.Status, string(body)).Error())
		return 1
	}

	fmt.Fprintln(out, string(body))
	return 0
}

// statusWithClient implements status using provided http client/baseURL.
func statusWithClient(args []string, client *http.Client, baseURL string, out io.Writer, errOut io.Writer) int {
	if len(args) != 1 {
		usage(errOut)
		return 2
	}
	taskID := args[0]

	resp, err := client.Get(baseURL + "/v1/tasks/" + taskID)
	if err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}
	if resp.StatusCode >= 400 {
		fmt.Fprintln(errOut, fmt.Errorf("request failed: %s: %s", resp.Status, string(body)).Error())
		return 1
	}

	fmt.Fprintln(out, string(body))
	return 0
}

func listWithClient(args []string, client *http.Client, baseURL string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var limit int
	fs.IntVar(&limit, "limit", 0, "limit number of tasks")
	_ = fs.Parse(args)

	url := baseURL + "/v1/tasks"
	if limit > 0 {
		url = fmt.Sprintf("%s?limit=%d", url, limit)
	}

	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}
	if resp.StatusCode >= 400 {
		fmt.Fprintln(errOut, fmt.Errorf("request failed: %s: %s", resp.Status, string(body)).Error())
		return 1
	}
	fmt.Fprintln(out, string(body))
	return 0
}

func cancelWithClient(args []string, client *http.Client, baseURL string, out io.Writer, errOut io.Writer) int {
	if len(args) != 1 {
		usage(errOut)
		return 2
	}
	taskID := args[0]
	req, _ := http.NewRequest("POST", baseURL+"/v1/tasks/"+taskID+"/cancel", nil)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Fprintln(errOut, fmt.Errorf("request failed: %s: %s", resp.Status, string(body)).Error())
		return 1
	}
	fmt.Fprintln(out, string(body))
	return 0
}

func logsWithClient(args []string, client *http.Client, baseURL string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var tail int
	fs.IntVar(&tail, "tail", 0, "tail last N lines")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		usage(errOut)
		return 2
	}
	taskID := fs.Arg(0)
	u := baseURL + "/v1/tasks/" + taskID + "/logs"
	if tail > 0 {
		u = u + fmt.Sprintf("?tail=%d", tail)
	}
	resp, err := client.Get(u)
	if err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}
	if resp.StatusCode == http.StatusNotFound {
		fmt.Fprintln(errOut, "logs not found")
		return 1
	}
	if resp.StatusCode >= 400 {
		fmt.Fprintln(errOut, fmt.Errorf("request failed: %s: %s", resp.Status, string(body)).Error())
		return 1
	}
	fmt.Fprintln(out, string(body))
	return 0
}

func cleanupWithClient(args []string, client *http.Client, baseURL string, out io.Writer, errOut io.Writer) int {
	if len(args) != 1 {
		usage(errOut)
		return 2
	}
	taskID := args[0]
	// try placeholder endpoint
	req, _ := http.NewRequest("POST", baseURL+"/v1/tasks/"+taskID+"/cleanup", nil)
	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(errOut, err.Error())
		return 1
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		fmt.Fprintln(errOut, "cleanup not implemented")
		return 2
	}
	if resp.StatusCode >= 400 {
		fmt.Fprintln(errOut, fmt.Errorf("request failed: %s: %s", resp.Status, string(body)).Error())
		return 1
	}
	fmt.Fprintln(out, string(body))
	return 0
}

// doctorWithIO implements the 'doctor' command. It checks for git/gh, the
// presence of .molecular/config.toml and hook scripts. Supports --json for
// machine-readable output.
func doctorWithIO(args []string, out io.Writer, errOut io.Writer) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(errOut)
	var jsonMode bool
	fs.BoolVar(&jsonMode, "json", false, "output compact JSON for scripting")
	_ = fs.Parse(args)

	type report struct {
		Git      bool     `json:"git"`
		GH       bool     `json:"gh"`
		Config   bool     `json:"config"`
		Hooks    []string `json:"hooks"`
		Problems []string `json:"problems"`
	}

	res := report{}

	// look up git in PATH
	if _, err := execLookPath("git"); err == nil {
		res.Git = true
	} else {
		res.Git = false
		res.Problems = append(res.Problems, "git not found in PATH")
	}
	if _, err := execLookPath("gh"); err == nil {
		res.GH = true
	}

	// check .molecular/config.toml
	cfgPath := filepath.Join(".molecular", "config.toml")
	if _, err := os.Stat(cfgPath); err == nil {
		res.Config = true
	} else {
		res.Config = false
		res.Problems = append(res.Problems, ".molecular/config.toml not found")
	}

	// check hooks
	hooks := []string{"lithium.sh", "chlorine.sh"}
	for _, h := range hooks {
		p := filepath.Join(".molecular", h)
		if fi, err := os.Stat(p); err != nil {
			res.Hooks = append(res.Hooks, fmt.Sprintf("%s: missing", h))
			res.Problems = append(res.Problems, fmt.Sprintf("%s missing", p))
		} else {
			// executable on unix
			if fi.Mode()&0111 != 0 {
				res.Hooks = append(res.Hooks, fmt.Sprintf("%s: ok (executable)", h))
			} else {
				res.Hooks = append(res.Hooks, fmt.Sprintf("%s: present (not executable)", h))
				res.Problems = append(res.Problems, fmt.Sprintf("%s not executable", p))
			}
		}
	}

	if jsonMode {
		b, _ := json.Marshal(res)
		fmt.Fprintln(out, string(b))
		if len(res.Problems) > 0 {
			return 1
		}
		return 0
	}

	// human-friendly output
	fmt.Fprintln(out, "molecular doctor report:")
	fmt.Fprintf(out, "  git: %v\n", res.Git)
	fmt.Fprintf(out, "  gh: %v\n", res.GH)
	fmt.Fprintf(out, "  config present: %v\n", res.Config)
	fmt.Fprintln(out, "  hooks:")
	for _, h := range res.Hooks {
		fmt.Fprintf(out, "    - %s\n", h)
	}
	if len(res.Problems) > 0 {
		fmt.Fprintln(out, "problems:")
		for _, p := range res.Problems {
			fmt.Fprintf(out, "  - %s\n", p)
		}
		return 1
	}
	fmt.Fprintln(out, "ok")
	return 0
}
