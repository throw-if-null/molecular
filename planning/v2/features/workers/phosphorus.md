# Feature: Phosphorus Reporter Worker (v2)

## Goal

Implement Phosphorus, a reporter worker that bundles task artifacts into a shareable HTML report with logs, diffs, test results, and metadata for easy sharing and archival.

## Current state

- Artifacts scattered across `.molecular/runs/<task>/attempts/` directories
- No unified view of task execution
- Difficult to share task results with team members
- No permanent archival format

## Requirements

### Worker role

**Phosphorus** runs after Chlorine (or after failed/cancelled tasks) to:
- Collect all attempt artifacts (logs, diffs, result.json files)
- Generate HTML report with navigation and styling
- Bundle assets into single directory or archive
- Persist report to `.molecular/reports/<task_id>/`
- Optionally upload report to configured location

### Report contents

1. **Task summary**
   - Task ID, prompt, status, phase
   - Timeline: created → started → updated → finished
   - Total duration, attempt count

2. **Attempt timeline**
   - Visual timeline showing all attempts
   - Role, status, duration, error summary
   - Expand/collapse sections for each attempt

3. **Logs**
   - Syntax-highlighted logs for each attempt
   - Collapsible sections
   - Search/filter functionality

4. **Diffs**
   - For Carbon/Chlorine attempts, show git diff
   - Syntax-highlighted code diffs
   - Side-by-side or unified view

5. **Test results**
   - Parse common test output formats (Go, pytest, Jest, etc.)
   - Show passed/failed tests
   - Link failures to relevant log sections

6. **Metadata**
   - Environment info (Go version, OS, etc.)
   - Configuration snapshot
   - Worker commands used

### Output format

**Option A: Single HTML file** (self-contained)
- Embeds CSS, JS, and log data inline
- Easy to share via email or file transfer
- Larger file size

**Option B: Directory with assets**
- `report.html` + `assets/` directory
- Logs as separate files (linked from HTML)
- More modular, easier to inspect raw files

**Recommended:** Option B with optional archive to `.tar.gz` or `.zip` for sharing.

### Trigger

- Phosphorus runs automatically after Chlorine succeeds
- Phosphorus runs on task failure/cancellation if configured
- Manual trigger: `molecular report <task-id>`

### Configuration

```toml
[workers]
phosphorus_enabled = true
phosphorus_on_failure = false  # Generate report even for failed tasks

[phosphorus]
output_format = "directory"  # or "archive"
archive_format = "tar.gz"    # or "zip"
upload_enabled = false
upload_url = ""              # Optional: upload report to URL
```

## Detailed implementation steps

### 1. Create Phosphorus worker

**File:** `internal/silicon/phosphorus_worker.go`

**Responsibilities:**
- Poll for tasks in phase `report` (new phase after `finish`)
- Create attempt for Phosphorus role
- Collect artifacts from all attempts
- Generate HTML report
- Write report to `.molecular/reports/<task_id>/`
- Mark task as `completed` (or `reported` status)

**Worker signature:**
```go
func StartPhosphorusWorker(ctx context.Context, s Store, repoRoot string, interval time.Duration) context.CancelFunc
```

### 2. Add `report` phase to task lifecycle

**File:** `internal/store/store.go`

**Changes:**
- Add `report` phase to allowed phases
- Transition: `finish` → `report` (after Chlorine)
- Transition: `failed`/`cancelled` → `report` (if configured)

### 3. Create report generator

**File:** `internal/phosphorus/generator.go`

**API:**
```go
package phosphorus

type Config struct {
    TaskID      string
    RepoRoot    string
    OutputDir   string
    Format      string // "directory" or "archive"
}

type Report struct {
    Task     Task
    Attempts []Attempt
    Logs     map[string]string  // attemptID → log content
    Diffs    map[string]string  // attemptID → diff content
}

// Generate creates an HTML report from task artifacts
func Generate(cfg Config) (reportPath string, err error)
```

**Implementation:**
- Load task from store
- Load all attempts from store
- Read logs from `.molecular/runs/<task>/attempts/<id>/log.txt`
- Generate diffs using `git diff` for Carbon/Chlorine attempts
- Render HTML using Go `html/template`
- Write to output directory
- Optionally create archive

### 4. Create HTML template

**File:** `internal/phosphorus/templates/report.html`

**Features:**
- Responsive design (works on mobile/desktop)
- Collapsible sections for attempts
- Syntax highlighting for logs (use Prism.js or similar, embedded)
- Search/filter functionality
- Timeline visualization (CSS timeline)

**Example structure:**
```html
<!DOCTYPE html>
<html>
<head>
    <title>Report: {{.Task.TaskID}}</title>
    <style>
        /* Embedded CSS */
    </style>
</head>
<body>
    <header>
        <h1>Task Report: {{.Task.TaskID}}</h1>
        <p>Status: {{.Task.Status}}</p>
    </header>
    
    <section id="summary">
        <h2>Summary</h2>
        <!-- Task summary -->
    </section>
    
    <section id="timeline">
        <h2>Attempt Timeline</h2>
        <!-- Visual timeline -->
    </section>
    
    <section id="attempts">
        <h2>Attempts</h2>
        {{range .Attempts}}
        <details>
            <summary>{{.Role}} ({{.Status}})</summary>
            <pre>{{index $.Logs .AttemptID}}</pre>
        </details>
        {{end}}
    </section>
    
    <script>
        /* Embedded JS for search/filter */
    </script>
</body>
</html>
```

### 5. Add CLI command

**File:** `cmd/molecular/main.go`

**New command:**
```bash
molecular report <task-id> [--output=path] [--format=directory|archive]
```

**Implementation:**
- Call API endpoint: `POST /v1/tasks/{task_id}/report`
- API handler creates Phosphorus attempt and generates report
- Return report path to user

### 6. Add API endpoint

**Endpoint:** `POST /v1/tasks/{task_id}/report`

**Response:**
```json
{
  "task_id": "feat-123",
  "report_path": ".molecular/reports/feat-123/report.html",
  "status": "generated"
}
```

### 7. Optional: Upload to remote

**File:** `internal/phosphorus/upload.go`

**Implementation:**
- If `upload_enabled=true`, upload report to configured URL
- Support HTTP POST with multipart/form-data
- Support signed URL upload (S3, GCS, etc.)
- Store upload URL in task metadata

### 8. Tests

**Unit tests:**
- Report generation with sample task data
- HTML template rendering
- Archive creation (.tar.gz, .zip)

**Integration tests:**
- Submit task, complete full workflow, generate report
- Verify report contains all attempts
- Verify logs and diffs included

### 9. Documentation

**File:** `README.md`

**Add section:**
```markdown
## Reports

Phosphorus automatically generates shareable HTML reports after tasks complete.

Reports are saved to `.molecular/reports/<task-id>/`.

Manually generate a report:

\```bash
molecular report feat-123
\```
```

## Acceptance criteria

- [ ] Phosphorus worker implemented
- [ ] Report generation creates HTML with logs and diffs
- [ ] Reports include attempt timeline
- [ ] `molecular report <task-id>` command works
- [ ] Configuration supports enabling/disabling Phosphorus
- [ ] Reports stored in `.molecular/reports/`
- [ ] Optional archive format (tar.gz/zip)
- [ ] Documentation updated

## Example report structure

```
.molecular/reports/feat-123/
├── report.html          # Main report file
├── assets/
│   ├── logs/
│   │   ├── lithium-abc123.txt
│   │   ├── carbon-def456.txt
│   │   └── ...
│   ├── diffs/
│   │   ├── carbon-def456.diff
│   │   └── ...
│   └── styles/
│       └── report.css   # (if not inlined)
└── metadata.json        # Task metadata snapshot
```

Or as archive:
```
.molecular/reports/feat-123.tar.gz
```

## Follow-up work (post-v2)

- PDF export via headless browser
- Compare reports across tasks
- Report templates (customize HTML/CSS)
- Metrics dashboard (aggregate stats across all tasks)
- Email report delivery
- Slack/Teams integration for report sharing
