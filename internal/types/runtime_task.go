package types

import (
	"errors"
	"time"
)

var (
	// ErrInvalidRuntimeTaskCursor means the client supplied a cursor that was
	// malformed or belonged to another queue/state view.
	ErrInvalidRuntimeTaskCursor = errors.New("invalid runtime task cursor")
	// ErrExpiredRuntimeTaskCursor means every task anchor retained in the
	// cursor has left the selected live state. The caller should refresh from
	// the first page rather than falling back to offset pagination.
	ErrExpiredRuntimeTaskCursor = errors.New("expired runtime task cursor")
)

// RuntimeTaskState is the stable operator-facing task lifecycle. It mirrors
// the durable states exposed by asynq while keeping the HTTP API independent
// from the queue library's Go enum.
type RuntimeTaskState string

const (
	RuntimeTaskPending   RuntimeTaskState = "pending"
	RuntimeTaskActive    RuntimeTaskState = "active"
	RuntimeTaskScheduled RuntimeTaskState = "scheduled"
	RuntimeTaskRetry     RuntimeTaskState = "retry"
	RuntimeTaskArchived  RuntimeTaskState = "archived"
	RuntimeTaskCompleted RuntimeTaskState = "completed"
)

func (s RuntimeTaskState) Valid() bool {
	switch s {
	case RuntimeTaskPending, RuntimeTaskActive, RuntimeTaskScheduled,
		RuntimeTaskRetry, RuntimeTaskArchived, RuntimeTaskCompleted:
		return true
	default:
		return false
	}
}

// RuntimeTaskAction is returned by the backend for every task. The frontend
// renders only these actions instead of inferring safety from the state.
type RuntimeTaskAction string

const (
	RuntimeTaskActionCancel RuntimeTaskAction = "cancel"
	RuntimeTaskActionRunNow RuntimeTaskAction = "run_now"
	RuntimeTaskActionDelete RuntimeTaskAction = "delete"
)

// RuntimeTaskInfo is the safe SystemAdmin projection of one queue task.
// Raw payloads and results are deliberately excluded because they may contain
// document content, signed object URLs, or connector credentials.
type RuntimeTaskInfo struct {
	ID              string              `json:"id"`
	Queue           string              `json:"queue"`
	Type            string              `json:"type"`
	State           RuntimeTaskState    `json:"state"`
	AllowedActions  []RuntimeTaskAction `json:"allowed_actions"`
	LastError       string              `json:"last_error,omitempty"`
	LastFailedAt    *time.Time          `json:"last_failed_at,omitempty"`
	NextProcessAt   *time.Time          `json:"next_process_at,omitempty"`
	StartedAt       *time.Time          `json:"started_at,omitempty"`
	CompletedAt     *time.Time          `json:"completed_at,omitempty"`
	Deadline        *time.Time          `json:"deadline,omitempty"`
	EnqueuedAt      *time.Time          `json:"enqueued_at,omitempty"`
	Retried         int                 `json:"retried"`
	MaxRetry        int                 `json:"max_retry"`
	IsOrphaned      bool                `json:"is_orphaned,omitempty"`
	Worker          string              `json:"worker,omitempty"`
	TenantID        uint64              `json:"tenant_id,omitempty"`
	KnowledgeBaseID string              `json:"knowledge_base_id,omitempty"`
	KnowledgeID     string              `json:"knowledge_id,omitempty"`
	TaskID          string              `json:"task_id,omitempty"`
	SourceID        string              `json:"source_id,omitempty"`
	TargetID        string              `json:"target_id,omitempty"`
	SourceKBID      string              `json:"source_kb_id,omitempty"`
	TargetKBID      string              `json:"target_kb_id,omitempty"`
	DataSourceID    string              `json:"data_source_id,omitempty"`
	SyncLogID       string              `json:"sync_log_id,omitempty"`
	KnowledgeCount  int                 `json:"knowledge_count,omitempty"`
	// Name/model/failure decorations. Names are resolved from Postgres by
	// the SystemAdmin handler (best-effort: empty means the object is gone
	// or unresolvable, and clients fall back to the ID). FailureCode is the
	// inferred reason from the last error string.
	KnowledgeBaseName  string             `json:"knowledge_base_name,omitempty"`
	KnowledgeName      string             `json:"knowledge_name,omitempty"`
	KnowledgeFileName  string             `json:"knowledge_file_name,omitempty"`
	TenantName         string             `json:"tenant_name,omitempty"`
	SourceKBName       string             `json:"source_kb_name,omitempty"`
	TargetKBName       string             `json:"target_kb_name,omitempty"`
	DataSourceName     string             `json:"data_source_name,omitempty"`
	Models             []RuntimeTaskModel `json:"models,omitempty"`
	FailureCode        string             `json:"failure_code,omitempty"`
	FailureDisposition string             `json:"failure_disposition,omitempty"`
}

// RuntimeTaskModel names one model a task will call, read from its payload.
// Role is a stable wire value (see the RuntimeTaskModelRole* constants); the
// frontend localizes it. Name is resolved from Postgres and stays empty when
// the model row is gone.
type RuntimeTaskModel struct {
	Role string `json:"role"`
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// RuntimeTaskModelRole* are the stable role values of RuntimeTaskModel.
const (
	RuntimeTaskModelRoleDefault   = "model"
	RuntimeTaskModelRoleChat      = "chat"
	RuntimeTaskModelRoleEmbedding = "embedding"
	RuntimeTaskModelRoleSummary   = "summary"
	RuntimeTaskModelRoleASR       = "asr"
	RuntimeTaskModelRoleVLM       = "vlm"
)

// RuntimeRetryAllResult reports what a bulk retry of a queue's archived
// (finally-failed) tasks did. The three totals partition the archived set as
// it stood when the call counted it: Retried counts tasks whose RunTask
// succeeded; Failed counts tasks of a selected type that the call attempted
// and could not revive (RunTask returned an error, including for a task that
// vanished between listing and the retry); Skipped counts the snapshot
// members the call left untouched — their type was not selected, or the walk
// stopped before reaching them. Retried + Failed + Skipped therefore accounts
// for every task the count saw; tasks that newly archive mid-call can only
// raise Retried/Failed above that snapshot, and a selected type's untouched
// remainder is floored at zero, so Skipped can never go negative. ByType
// carries exactly one entry per selected task type — zero when no task of
// that type was revived — so "selected but all failed" reads differently
// from "not selected".
type RuntimeRetryAllResult struct {
	Retried int            `json:"retried"`
	Skipped int            `json:"skipped"`
	Failed  int            `json:"failed"`
	ByType  map[string]int `json:"by_type"`
}

// RuntimeTaskPage is one stable slice of the operator task list. NextCursor
// is opaque to HTTP clients and remains tied to the selected queue/state.
type RuntimeTaskPage struct {
	Tasks      []RuntimeTaskInfo
	NextCursor string
	HasMore    bool
}

func (t RuntimeTaskInfo) Allows(action RuntimeTaskAction) bool {
	for _, allowed := range t.AllowedActions {
		if allowed == action {
			return true
		}
	}
	return false
}
