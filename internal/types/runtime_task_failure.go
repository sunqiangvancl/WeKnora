package types

import "strings"

// Runtime task failure codes. This is a stable wire format: the SystemAdmin
// queue console maps each code to a localized "inferred reason" and to an
// operator hint (retry is worth trying / needs a manual fix). The inference
// is based only on the task's last error string, so it is deliberately
// coarse — the raw error text is always shown next to it.
const (
	FailureCodeModelRateLimited     = "MODEL_RATE_LIMITED"
	FailureCodeModelUpstreamError   = "MODEL_UPSTREAM_ERROR"
	FailureCodeModelAuthFailed      = "MODEL_AUTH_FAILED"
	FailureCodeModelNotFound        = "MODEL_NOT_FOUND"
	FailureCodeModelEmptyOutput     = "MODEL_EMPTY_OUTPUT"
	FailureCodeModelResponseInvalid = "MODEL_RESPONSE_INVALID"
	FailureCodeDocReaderUnavailable = "DOCREADER_UNAVAILABLE"
	FailureCodeDocReaderTimeout     = "DOCREADER_TIMEOUT"
	FailureCodeDocReaderParseFailed = "DOCREADER_PARSE_FAILED"
	FailureCodeSourceFileUnreadable = "SOURCE_FILE_UNREADABLE"
	FailureCodeTaskTimeout          = "TASK_TIMEOUT"
	FailureCodeTaskCanceled         = "TASK_CANCELED"
	FailureCodeTaskInterrupted      = "TASK_INTERRUPTED"
	FailureCodeVectorStoreFailed    = "VECTORSTORE_WRITE_FAILED"
	FailureCodeEmbeddingFailed      = "EMBEDDING_FAILED"
	FailureCodeResourceGone         = "RESOURCE_GONE"
	FailureCodeUnknown              = "UNKNOWN"
)

// Failure dispositions tell the operator what to do about an inferred
// reason. Unknown means the classifier could not decide; the UI then shows
// the reason without a recommendation.
const (
	FailureDispositionRetryable     = "retryable"
	FailureDispositionDeterministic = "deterministic"
	FailureDispositionUnknown       = "unknown"
)

var failureDispositionByCode = map[string]string{
	FailureCodeModelRateLimited:     FailureDispositionRetryable,
	FailureCodeModelUpstreamError:   FailureDispositionRetryable,
	FailureCodeModelAuthFailed:      FailureDispositionDeterministic,
	FailureCodeModelNotFound:        FailureDispositionDeterministic,
	FailureCodeModelEmptyOutput:     FailureDispositionUnknown,
	FailureCodeModelResponseInvalid: FailureDispositionUnknown,
	FailureCodeDocReaderUnavailable: FailureDispositionRetryable,
	FailureCodeDocReaderTimeout:     FailureDispositionRetryable,
	FailureCodeDocReaderParseFailed: FailureDispositionDeterministic,
	FailureCodeSourceFileUnreadable: FailureDispositionDeterministic,
	FailureCodeTaskTimeout:          FailureDispositionRetryable,
	FailureCodeTaskCanceled:         FailureDispositionRetryable,
	FailureCodeTaskInterrupted:      FailureDispositionRetryable,
	FailureCodeVectorStoreFailed:    FailureDispositionRetryable,
	FailureCodeEmbeddingFailed:      FailureDispositionRetryable,
	FailureCodeResourceGone:         FailureDispositionDeterministic,
	FailureCodeUnknown:              FailureDispositionUnknown,
}

// FailureDispositionForCode returns the disposition registered for a code.
// Unknown codes report FailureDispositionUnknown rather than an error: the
// mapping may legitimately lag a newer backend.
func FailureDispositionForCode(code string) string {
	if disposition, ok := failureDispositionByCode[code]; ok {
		return disposition
	}
	return FailureDispositionUnknown
}

// failureNeedles maps one code to the lowercase substrings that identify it.
// Order matters: the first matching entry wins, so specific patterns must be
// listed before generic ones (e.g. "no sheets found" before "not found").
var failureNeedles = []struct {
	code    string
	needles []string
}{
	{FailureCodeSourceFileUnreadable, []string{
		"no sheets found", "no xl/workbook.xml", "is the file corrupt",
		"invalid xf entry", "not found in xlsx", "not a valid zip file",
		"invalid zip", "unsupported file type", "file type not supported",
		"empty file", "file is empty",
	}},
	{FailureCodeModelRateLimited, []string{
		"50602", "rate limit", "ratelimit", "rate_limit", "too many requests",
		"qpm", "qps", "throttl", "调用频率", "频率超限", "请求过于频繁",
		"slow down", "status 429",
	}},
	{FailureCodeModelAuthFailed, []string{
		"status 401", "status 403", "invalid api key", "api key not",
		"unauthorized", "authentication failed", "invalid token",
	}},
	{FailureCodeModelNotFound, []string{
		"model not found", "model_not_found", "synthesis_model_not_found",
		"missing_synthesis_model", "no summary model", "no embedding model",
		"no chat model", "no synthesis model", "model is not configured",
	}},
	{FailureCodeDocReaderTimeout, []string{
		"docreader call timeout", "docreader timeout",
	}},
	{FailureCodeDocReaderUnavailable, []string{
		"document parsing service", "docreader service not connected",
		"code = unavailable",
	}},
	{FailureCodeDocReaderParseFailed, []string{
		"document read failed", "docreader parse", "parse document failed",
		"document parsing failed",
	}},
	// Provider-transport needles sit above the generic timeout needles so a
	// "send request: … TLS handshake timeout" stays a model error instead of
	// being read as a task-scheduler timeout. The docreader checks above win
	// for the same reason.
	{FailureCodeModelUpstreamError, []string{
		"api request failed with status 5", "server_error", "upstream",
		"server error", "send request", "read response", "bad gateway",
		"service unavailable", "connection refused", "connection reset",
		"broken pipe", "no such host", "tls handshake", "unexpected eof",
	}},
	{FailureCodeTaskInterrupted, []string{
		"lease expired", "application restart", "interrupted",
	}},
	{FailureCodeTaskCanceled, []string{"context canceled", "context cancelled"}},
	{FailureCodeTaskTimeout, []string{
		"deadline exceeded", "i/o timeout", "timed out", "timeout",
	}},
	{FailureCodeModelEmptyOutput, []string{
		"returned no gist", "returned empty output", "empty output",
		"empty response",
	}},
	{FailureCodeModelResponseInvalid, []string{
		"invalid character", "unexpected end of json", "invalid json",
	}},
	{FailureCodeVectorStoreFailed, []string{
		"vectorstore", "vector store", "milvus", "qdrant", "weaviate", "upsert",
	}},
	{FailureCodeEmbeddingFailed, []string{"embedding"}},
	{FailureCodeResourceGone, []string{
		"no rows in result set", "record not found", "knowledge not found",
		"knowledge base not found", "kb not found", "not found",
		"does not exist", "already deleted", "kb_deleted",
	}},
}

// exhaustedRetriesMarker is the wrapper the parsers add when a task's retries
// are used up ("summary generation exhausted retries: create chat
// completion: ..."). The interesting reason is the wrapped text.
const exhaustedRetriesMarker = "exhausted retries: "

// ClassifyRuntimeTaskFailure infers a failure code from a task's last error
// string. It returns FailureCodeUnknown for an empty or unrecognized error.
func ClassifyRuntimeTaskFailure(lastError string) string {
	if strings.TrimSpace(lastError) == "" {
		return FailureCodeUnknown
	}
	message := strings.ToLower(lastError)
	if index := strings.LastIndex(message, exhaustedRetriesMarker); index >= 0 {
		message = message[index+len(exhaustedRetriesMarker):]
	}
	// "<synthesis|summary|chat|embedding> model <id> not found" spells the
	// two halves too far apart for a single needle; the pairing is a model
	// configuration problem, not a deleted resource, so it is resolved
	// before the generic "not found" needle below.
	if strings.Contains(message, "model") && strings.Contains(message, "not found") {
		return FailureCodeModelNotFound
	}
	for _, entry := range failureNeedles {
		for _, needle := range entry.needles {
			if strings.Contains(message, needle) {
				return entry.code
			}
		}
	}
	return FailureCodeUnknown
}
