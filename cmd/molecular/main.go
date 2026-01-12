package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/throw-if-null/molecular/internal/api"
	"github.com/throw-if-null/molecular/internal/version"
)

func main() {
	client := &http.Client{Timeout: 30 * time.Second}
	baseURL := fmt.Sprintf("http://%s:%d", api.DefaultHost, api.DefaultPort)
	os.Exit(run(os.Args[1:], client, baseURL, os.Stdout, os.Stderr))
}

func usage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "usage:")
	_, _ = fmt.Fprintln(w, "  molecular submit --task-id <id> --prompt <text>")
	_, _ = fmt.Fprintln(w, "  molecular status <task-id>")
	_, _ = fmt.Fprintln(w, "  molecular list [--limit N]")
	_, _ = fmt.Fprintln(w, "  molecular cancel <task-id>")
	_, _ = fmt.Fprintln(w, "  molecular logs <task-id> [--tail N]")
	_, _ = fmt.Fprintln(w, "  molecular cleanup <task-id>")
	_, _ = fmt.Fprintln(w, "  molecular version")
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
	fs.IntVar(&tail, "tail", 0, "tail last N lines (not supported)")
	_ = fs.Parse(args)
	if fs.NArg() != 1 {
		usage(errOut)
		return 2
	}
	taskID := fs.Arg(0)
	if tail > 0 {
		fmt.Fprintln(errOut, "warning: --tail is not supported yet; ignoring")
	}
	resp, err := client.Get(baseURL + "/v1/tasks/" + taskID + "/logs")
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
