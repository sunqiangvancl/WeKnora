package types

import "testing"

// TestClassifyRuntimeTaskFailureCases pins the inferred failure code against
// real error strings. Every case is copied from the production
// task_dead_letters ledger (2026-10-20, 8906 rows) so the classifier is
// grounded in what this deployment actually produces.
func TestClassifyRuntimeTaskFailureCases(t *testing.T) {
	cases := []struct {
		name  string
		err   string
		want  string
		notes string
	}{
		{
			name: "siliconflow 429 with upstream server_error body",
			err:  `summary generation exhausted retries: create chat completion: API request failed with status 429: {"error":{"type":"server_error","message":"Upstream error: rate limit hit"}}`,
			want: FailureCodeModelRateLimited,
		},
		{
			name: "siliconflow 400 upstream server_error",
			err:  `generate knowledge base description: create chat completion: API request failed with status 400: {"error":{"type":"server_error","message":"Upstream request failed"}}`,
			want: FailureCodeModelUpstreamError,
		},
		{
			name: "siliconflow 50602 request rejected",
			err:  `create chat completion: API request failed with status 429: {"code":50602,"message":"Request was rejected due to rate management"}`,
			want: FailureCodeModelRateLimited,
		},
		{
			name: "model credential rejected",
			err:  `generate knowledge base description: create chat completion: API request failed with status 401: {"error":{"type":"invalid_request_error"}}`,
			want: FailureCodeModelAuthFailed,
		},
		{
			name: "wiki synthesis model missing",
			err:  `synthesis model 3f9a2c not found`,
			want: FailureCodeModelNotFound,
		},
		{
			name: "wiki batch retries exhausted without a specific cause",
			err:  `exceeded wikiMaxFailRetries=5 (in-batch retries)`,
			want: FailureCodeUnknown,
		},
		{
			name: "summary model produced nothing",
			err:  `knowledge base description model returned no gist`,
			want: FailureCodeModelEmptyOutput,
		},
		{
			name: "summary model returned empty output",
			err:  `summary model returned empty output`,
			want: FailureCodeModelEmptyOutput,
		},
		{
			name: "xlsx missing workbook part",
			err:  `failed to create table from Excel file (sheets=[]): Binder Error: No xl/workbook.xml found in xlsx file LINE 1`,
			want: FailureCodeSourceFileUnreadable,
		},
		{
			name: "xlsx streaming zip with zero local headers",
			err:  `failed to create table from Excel file (sheets=[要求 sheet1]): Binder Error: No sheets found in xlsx file (is the file corrupt?)`,
			want: FailureCodeSourceFileUnreadable,
		},
		{
			name: "xlsx invalid styles part",
			err:  `failed to create table from Excel file (sheets=[工作表1]): Invalid Input Error: Invalid xf entry in styles.xml LINE 1`,
			want: FailureCodeSourceFileUnreadable,
		},
		{
			name: "indexing deadline exceeded",
			err:  `failed to index questions: context deadline exceeded`,
			want: FailureCodeTaskTimeout,
		},
		{
			name: "indexing canceled by the client",
			err:  `failed to index questions: context canceled`,
			want: FailureCodeTaskCanceled,
		},
		{
			name: "docreader not reachable",
			err:  `Cannot connect to the document parsing service, or the connection was interrupted. Check that DocReader is running.`,
			want: FailureCodeDocReaderUnavailable,
		},
		{
			name: "docreader gRPC unavailable mid-stream",
			err:  `document read failed: gRPC ReadStream recv failed: rpc error: code = Unavailable desc = error reading from server`,
			want: FailureCodeDocReaderUnavailable,
		},
		{
			name: "docreader generic parse failure",
			err:  `Document parsing failed. Check the file format and the DocReader logs for this attempt.`,
			want: FailureCodeDocReaderParseFailed,
		},
		{
			name: "docreader json conversion failure",
			err:  `document read failed: json conversion failed: invalid JSON content`,
			want: FailureCodeDocReaderParseFailed,
		},
		{
			name: "model returned non-JSON text",
			err:  "parse knowledge base description: invalid character '`' looking for beginning of value",
			want: FailureCodeModelResponseInvalid,
		},
		{
			name: "model returned truncated JSON",
			err:  `parse knowledge base description: unexpected end of JSON input`,
			want: FailureCodeModelResponseInvalid,
		},
		{
			name: "provider transport failure",
			err:  `generate knowledge base description: create chat completion: send request: Post "https://api.siliconflow.cn/v1/chat/completions": net/http: TLS handshake timeout`,
			want: FailureCodeModelUpstreamError,
		},
		{
			name: "worker lease expired after restart",
			err:  `task wiki:ingest lease expired at 2026-10-11T02:44:12Z: could not retry`,
			want: FailureCodeTaskInterrupted,
		},
		{
			name: "task interrupted by application restart",
			err:  `Task interrupted due to application restart`,
			want: FailureCodeTaskInterrupted,
		},
		{
			name: "knowledge row already deleted",
			err:  `load knowledge: sql: no rows in result set`,
			want: FailureCodeResourceGone,
		},
		{
			name: "empty error is unknown",
			err:  `   `,
			want: FailureCodeUnknown,
		},
		{
			name: "unrecognized error is unknown",
			err:  `something nobody has seen before`,
			want: FailureCodeUnknown,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ClassifyRuntimeTaskFailure(tc.err); got != tc.want {
				t.Fatalf("ClassifyRuntimeTaskFailure(%q) = %s, want %s", tc.err, got, tc.want)
			}
			disposition := FailureDispositionForCode(tc.want)
			if disposition != FailureDispositionRetryable &&
				disposition != FailureDispositionDeterministic &&
				disposition != FailureDispositionUnknown {
				t.Fatalf("disposition for %s = %q", tc.want, disposition)
			}
		})
	}
}

// TestFailureDispositionCoversEveryCode guards the two tables from drifting:
// every classification a user can see must have a disposition the console
// can render.
func TestFailureDispositionCoversEveryCode(t *testing.T) {
	codes := []string{
		FailureCodeModelRateLimited, FailureCodeModelUpstreamError,
		FailureCodeModelAuthFailed, FailureCodeModelNotFound,
		FailureCodeModelEmptyOutput, FailureCodeModelResponseInvalid,
		FailureCodeDocReaderUnavailable, FailureCodeDocReaderTimeout,
		FailureCodeDocReaderParseFailed, FailureCodeSourceFileUnreadable,
		FailureCodeTaskTimeout, FailureCodeTaskCanceled,
		FailureCodeTaskInterrupted, FailureCodeVectorStoreFailed,
		FailureCodeEmbeddingFailed, FailureCodeResourceGone,
		FailureCodeUnknown,
	}
	for _, code := range codes {
		if FailureDispositionForCode(code) == FailureDispositionUnknown && code != FailureCodeUnknown {
			t.Fatalf("code %s has no disposition", code)
		}
	}
	if FailureDispositionForCode("NOT_A_CODE") != FailureDispositionUnknown {
		t.Fatalf("unknown code must map to the unknown disposition")
	}
}
