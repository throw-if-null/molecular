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
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "submit":
		submit(os.Args[2:])
	case "status":
		status(os.Args[2:])
	case "version":
		fmt.Printf("molecular %s (%s)\n", version.Version, version.Commit)
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	_, _ = fmt.Fprintln(os.Stderr, "usage:")
	_, _ = fmt.Fprintln(os.Stderr, "  molecular submit --task-id <id> --prompt <text>")
	_, _ = fmt.Fprintln(os.Stderr, "  molecular status <task-id>")
	_, _ = fmt.Fprintln(os.Stderr, "  molecular version")
}

func submit(args []string) {
	fs := flag.NewFlagSet("submit", flag.ExitOnError)
	var taskID string
	var prompt string
	fs.StringVar(&taskID, "task-id", "", "task id")
	fs.StringVar(&prompt, "prompt", "", "task prompt")
	_ = fs.Parse(args)

	if taskID == "" || prompt == "" {
		fs.Usage()
		os.Exit(2)
	}

	req := api.CreateTaskRequest{TaskID: taskID, Prompt: prompt}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(&req); err != nil {
		fatal(err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Post(fmt.Sprintf("http://%s:%d/v1/tasks", api.DefaultHost, api.DefaultPort), "application/json", &buf)
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fatal(err)
	}
	if resp.StatusCode >= 400 {
		fatal(fmt.Errorf("request failed: %s: %s", resp.Status, string(body)))
	}

	fmt.Println(string(body))
}

func status(args []string) {
	if len(args) != 1 {
		usage()
		os.Exit(2)
	}
	taskID := args[0]

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://%s:%d/v1/tasks/%s", api.DefaultHost, api.DefaultPort, taskID))
	if err != nil {
		fatal(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fatal(err)
	}
	if resp.StatusCode >= 400 {
		fatal(fmt.Errorf("request failed: %s: %s", resp.Status, string(body)))
	}

	fmt.Println(string(body))
}

func fatal(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err.Error())
	os.Exit(1)
}
