# Feature: Health Monitoring and Alerting Hooks (v2)

## Goal

Add health monitoring and alerting capabilities to Silicon so operators can monitor daemon health, detect stuck tasks, and receive notifications for critical failures.

## Current state

- No health metrics beyond basic `/healthz` endpoint
- No alerting for task failures or stuck workers
- No visibility into worker queue depth or throughput
- Difficult to monitor Silicon in production

## Requirements

### Health metrics

Expose metrics via `/healthz` endpoint (enhanced):

```json
{
  "status": "healthy",
  "version": "v1.2.0",
  "uptime_seconds": 3600,
  "repo_root": "/path/to/repo",
  "workers": {
    "lithium": {"status": "running", "last_poll": "2026-01-13T10:00:00Z"},
    "carbon": {"status": "running", "last_poll": "2026-01-13T10:00:01Z"},
    "helium": {"status": "running", "last_poll": "2026-01-13T10:00:02Z"},
    "chlorine": {"status": "running", "last_poll": "2026-01-13T10:00:03Z"}
  },
  "store": {
    "status": "healthy",
    "db_size_mb": 12.5,
    "tasks_total": 42,
    "tasks_running": 2,
    "tasks_failed": 3
  },
  "disk": {
    "worktrees_size_mb": 250.3,
    "artifacts_size_mb": 105.7,
    "available_mb": 15000
  }
}
```

### Alerting hooks

Configure webhook URLs to receive alerts for:
- Task failures (after exhausting retries)
- Task stuck (e.g., in same phase for >30 min)
- Worker crash/panic recovery
- Disk space low (<10% available)
- Database errors

### Webhook payload

```json
{
  "event": "task.failed",
  "timestamp": "2026-01-13T10:00:00Z",
  "severity": "warning",
  "task_id": "feat-123",
  "phase": "build",
  "message": "Task failed after exhausting carbon_budget (3 attempts)",
  "metadata": {
    "last_error": "pnpm test failed: 2 tests failed",
    "attempts": 3,
    "duration_seconds": 120
  }
}
```

### Configuration

```toml
[monitoring]
health_endpoint = true
health_detailed = true  # Include disk/store metrics

[monitoring.alerts]
enabled = true
webhook_url = "https://hooks.slack.com/services/..."
webhook_timeout_ms = 5000

# Event filters
on_task_failed = true
on_task_stuck = true
on_worker_panic = true
on_disk_low = true
on_db_error = true

# Thresholds
task_stuck_threshold_minutes = 30
disk_low_threshold_mb = 1000
```

## Detailed implementation steps

### 1. Enhance `/healthz` endpoint

**File:** `internal/silicon/server.go`

**Changes:**
- Add detailed health check logic
- Query store for task counts
- Check worker last-poll timestamps
- Check disk usage for `.molecular/` directories
- Return JSON with metrics

**Implementation:**
```go
func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
    health := Health{
        Status:  "healthy",
        Version: version.Version,
        Uptime:  time.Since(s.startTime).Seconds(),
        RepoRoot: s.repoRoot,
    }
    
    // Worker health
    health.Workers = s.getWorkerHealth()
    
    // Store health
    health.Store = s.getStoreHealth()
    
    // Disk health
    health.Disk = s.getDiskHealth()
    
    // Determine overall status
    if health.Disk.AvailableMB < s.config.DiskLowThresholdMB {
        health.Status = "degraded"
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(health)
}
```

### 2. Create monitoring package

**File:** `internal/monitoring/monitoring.go`

**API:**
```go
package monitoring

type Config struct {
    Enabled     bool
    WebhookURL  string
    Timeout     time.Duration
    EventFilters EventFilters
}

type EventFilters struct {
    OnTaskFailed   bool
    OnTaskStuck    bool
    OnWorkerPanic  bool
    OnDiskLow      bool
    OnDBError      bool
}

type Alert struct {
    Event     string
    Timestamp time.Time
    Severity  string
    TaskID    string
    Phase     string
    Message   string
    Metadata  map[string]interface{}
}

// SendAlert sends an alert to configured webhook
func SendAlert(cfg Config, alert Alert) error
```

**Implementation:**
- HTTP POST to webhook URL with JSON payload
- Timeout handling
- Retry logic (3 attempts with exponential backoff)
- Log failures to stderr (don't block worker operations)

### 3. Instrument workers with alerts

**File:** `internal/silicon/worker.go`

**Changes:**
- Detect stuck tasks (same phase for >threshold time)
- Send alert on task failure after budget exhaustion
- Send alert on panic recovery

**Example (Carbon worker):**
```go
func StartCarbonWorker(...) {
    defer func() {
        if r := recover(); r != nil {
            alert := monitoring.Alert{
                Event:     "worker.panic",
                Severity:  "critical",
                Message:   fmt.Sprintf("Carbon worker panicked: %v", r),
                Metadata:  map[string]interface{}{"stack": string(debug.Stack())},
            }
            monitoring.SendAlert(monitoringConfig, alert)
            panic(r) // Re-panic after alerting
        }
    }()
    
    for {
        // ... worker loop ...
        
        // Check for stuck tasks
        if time.Since(task.UpdatedAt) > stuckThreshold {
            alert := monitoring.Alert{
                Event:    "task.stuck",
                Severity: "warning",
                TaskID:   task.TaskID,
                Phase:    task.Phase,
                Message:  fmt.Sprintf("Task stuck in %s phase for %s", task.Phase, time.Since(task.UpdatedAt)),
            }
            monitoring.SendAlert(monitoringConfig, alert)
        }
        
        // On failure after budget exhaustion
        if task.CarbonRetries >= carbonBudget {
            alert := monitoring.Alert{
                Event:    "task.failed",
                Severity: "warning",
                TaskID:   task.TaskID,
                Phase:    "build",
                Message:  "Task failed after exhausting carbon_budget",
                Metadata: map[string]interface{}{"attempts": task.CarbonRetries},
            }
            monitoring.SendAlert(monitoringConfig, alert)
        }
    }
}
```

### 4. Add disk monitoring

**File:** `internal/monitoring/disk.go`

**Implementation:**
- Periodically check disk usage for `.molecular/` directories
- Alert if available space < threshold
- Use `syscall.Statfs` (Unix) or equivalent (Windows)

**Example:**
```go
func CheckDiskUsage(repoRoot string, threshold int64) (DiskUsage, error) {
    var stat syscall.Statfs_t
    if err := syscall.Statfs(repoRoot, &stat); err != nil {
        return DiskUsage{}, err
    }
    
    availableMB := int64(stat.Bavail * uint64(stat.Bsize)) / (1024 * 1024)
    
    return DiskUsage{
        AvailableMB: availableMB,
        LowDisk:     availableMB < threshold,
    }, nil
}
```

### 5. Add database health monitoring

**File:** `internal/store/health.go`

**Implementation:**
- Check DB connection health (ping)
- Report DB size
- Report task count by status
- Alert on connection errors

**Example:**
```go
func (s *Store) Health() (StoreHealth, error) {
    if err := s.db.Ping(); err != nil {
        return StoreHealth{Status: "unhealthy"}, err
    }
    
    var dbSizeMB float64
    s.db.QueryRow("SELECT page_count * page_size / 1024.0 / 1024.0 FROM pragma_page_count, pragma_page_size").Scan(&dbSizeMB)
    
    var tasksTotal, tasksRunning, tasksFailed int
    s.db.QueryRow("SELECT COUNT(*) FROM tasks").Scan(&tasksTotal)
    s.db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status='running'").Scan(&tasksRunning)
    s.db.QueryRow("SELECT COUNT(*) FROM tasks WHERE status='failed'").Scan(&tasksFailed)
    
    return StoreHealth{
        Status:       "healthy",
        DBSizeMB:     dbSizeMB,
        TasksTotal:   tasksTotal,
        TasksRunning: tasksRunning,
        TasksFailed:  tasksFailed,
    }, nil
}
```

### 6. Update config schema

**File:** `internal/config/config.go`

**Add monitoring section:**
```go
type Config struct {
    Silicon    SiliconConfig    `toml:"silicon"`
    Retry      RetryConfig      `toml:"retry"`
    Workers    WorkersConfig    `toml:"workers"`
    Hooks      HooksConfig      `toml:"hooks"`
    OTel       OTelConfig       `toml:"otel"`
    Monitoring MonitoringConfig `toml:"monitoring"`
}

type MonitoringConfig struct {
    HealthEndpoint bool        `toml:"health_endpoint"`
    HealthDetailed bool        `toml:"health_detailed"`
    Alerts         AlertConfig `toml:"alerts"`
}

type AlertConfig struct {
    Enabled                     bool   `toml:"enabled"`
    WebhookURL                  string `toml:"webhook_url"`
    WebhookTimeoutMS            int    `toml:"webhook_timeout_ms"`
    OnTaskFailed                bool   `toml:"on_task_failed"`
    OnTaskStuck                 bool   `toml:"on_task_stuck"`
    OnWorkerPanic               bool   `toml:"on_worker_panic"`
    OnDiskLow                   bool   `toml:"on_disk_low"`
    OnDBError                   bool   `toml:"on_db_error"`
    TaskStuckThresholdMinutes   int    `toml:"task_stuck_threshold_minutes"`
    DiskLowThresholdMB          int    `toml:"disk_low_threshold_mb"`
}
```

**Defaults:**
```go
Monitoring: MonitoringConfig{
    HealthEndpoint: true,
    HealthDetailed: true,
    Alerts: AlertConfig{
        Enabled:                   false, // Opt-in
        WebhookTimeoutMS:          5000,
        OnTaskFailed:              true,
        OnTaskStuck:               true,
        OnWorkerPanic:             true,
        OnDiskLow:                 true,
        OnDBError:                 true,
        TaskStuckThresholdMinutes: 30,
        DiskLowThresholdMB:        1000,
    },
}
```

### 7. CLI health command

**File:** `cmd/molecular/main.go`

**New command:**
```bash
molecular health [--json]
```

**Implementation:**
- Call `GET /healthz`
- Print human-readable or JSON output

**Example output:**
```
Silicon Health Check

Status: healthy
Uptime: 1h 15m 30s
Version: v1.2.0

Workers:
  lithium  ✓ running (last poll: 2s ago)
  carbon   ✓ running (last poll: 1s ago)
  helium   ✓ running (last poll: 3s ago)
  chlorine ✓ running (last poll: 2s ago)

Store:
  Status: healthy
  Database size: 12.5 MB
  Tasks: 42 total, 2 running, 3 failed

Disk:
  Worktrees: 250.3 MB
  Artifacts: 105.7 MB
  Available: 15.0 GB
```

### 8. Tests

**Unit tests:**
- Webhook alert sending (success, timeout, retry)
- Disk usage calculation
- Store health check
- Stuck task detection

**Integration tests:**
- Submit task, trigger failure alert
- Verify webhook called with correct payload
- Health endpoint returns expected metrics

### 9. Documentation

**File:** `README.md`

**Add monitoring section:**
```markdown
## Monitoring

Silicon provides health metrics and alerting hooks.

### Health check

\```bash
curl http://127.0.0.1:8711/healthz
# or
molecular health
\```

### Alerts

Configure webhook alerts in `.molecular/config.toml`:

\```toml
[monitoring.alerts]
enabled = true
webhook_url = "https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
\```

Silicon sends alerts for:
- Task failures
- Stuck tasks (>30 min in same phase)
- Worker panics
- Low disk space
- Database errors
```

## Acceptance criteria

- [ ] Enhanced `/healthz` endpoint with detailed metrics
- [ ] Worker health tracking (last poll timestamps)
- [ ] Store health metrics (DB size, task counts)
- [ ] Disk usage monitoring
- [ ] Webhook alerting for configured events
- [ ] Configuration for alert filtering and thresholds
- [ ] `molecular health` CLI command
- [ ] Stuck task detection
- [ ] Panic recovery alerts
- [ ] Documentation updated

## Example Slack alert

When integrated with Slack webhook:

> **[Molecular Alert]** Task Failed
> 
> **Task ID:** feat-123  
> **Phase:** build  
> **Message:** Task failed after exhausting carbon_budget (3 attempts)  
> **Last Error:** pnpm test failed: 2 tests failed  
> **Duration:** 2m 0s  
> **Timestamp:** 2026-01-13 10:00:00 UTC

## Follow-up work (post-v2)

- Prometheus metrics exporter
- Grafana dashboard templates
- Custom alert templates (Slack, Teams, PagerDuty)
- Alert aggregation (batch multiple alerts)
- Alert rate limiting (prevent spam)
- Historical health metrics (time-series)
- Alerting for specific task IDs or patterns
