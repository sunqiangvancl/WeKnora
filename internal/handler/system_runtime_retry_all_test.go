package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newRetryAllRouter wires the two bulk-retry handlers onto a bare gin engine
// with the same :queue path shape as the production admin routes.
func newRetryAllRouter(retryAll, facets gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	r.POST("/q/:queue/archived/actions/retry_all", retryAll)
	r.GET("/q/:queue/archived/facets", facets)
	return r
}

// TestRetryAllArchivedRuntimeTasksRejectsUnknownQueue — CI filter: Runtime.
func TestRetryAllArchivedRuntimeTasksRejectsUnknownQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &SystemHandler{}
	r := newRetryAllRouter(h.RetryAllArchivedRuntimeTasks, h.ListArchivedRuntimeTaskFacets)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/q/nope/archived/actions/retry_all", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid queue")
}

// TestRetryAllArchivedRuntimeTasksRejectsTypeFromAnotherQueue — CI filter: Runtime.
func TestRetryAllArchivedRuntimeTasksRejectsTypeFromAnotherQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &SystemHandler{}
	r := newRetryAllRouter(h.RetryAllArchivedRuntimeTasks, h.ListArchivedRuntimeTaskFacets)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/q/summary/archived/actions/retry_all",
		strings.NewReader(`{"task_types":["document:process"]}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "task_types")
}

// TestRetryAllArchivedRuntimeTasksDelegatesToInspector — CI filter: Runtime.
func TestRetryAllArchivedRuntimeTasksDelegatesToInspector(t *testing.T) {
	gin.SetMode(gin.TestMode)
	insp := &runtimeTaskTestInspector{retryAllResult: types.RuntimeRetryAllResult{
		Retried: 391, Failed: 2, Skipped: 55, ByType: map[string]int{"summary:generation": 391},
	}}
	h := &SystemHandler{taskInspector: insp}
	r := newRetryAllRouter(h.RetryAllArchivedRuntimeTasks, h.ListArchivedRuntimeTaskFacets)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/q/summary/archived/actions/retry_all",
		strings.NewReader(`{"task_types":["summary:generation"]}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, types.QueueSummary, insp.retryAllQueue)
	assert.Equal(t, []string{types.TypeSummaryGeneration}, insp.retryAllTypes)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(391), body["retried"])
	assert.Equal(t, float64(2), body["failed"])
	assert.Equal(t, float64(55), body["skipped"])
	require.IsType(t, map[string]interface{}{}, body["by_type"])
	assert.Equal(t, float64(391), body["by_type"].(map[string]interface{})["summary:generation"])
}

// TestRetryAllArchivedRuntimeTasksRejectsMalformedBody — CI filter: Runtime.
// Pins the one 400 the retry-all handler adds beyond the purge mapping: a body
// that is present but not JSON must be refused, not silently treated as "no
// filter".
func TestRetryAllArchivedRuntimeTasksRejectsMalformedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	insp := &runtimeTaskTestInspector{retryAllResult: types.RuntimeRetryAllResult{Retried: 1}}
	h := &SystemHandler{taskInspector: insp}
	r := newRetryAllRouter(h.RetryAllArchivedRuntimeTasks, h.ListArchivedRuntimeTaskFacets)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/q/summary/archived/actions/retry_all",
		strings.NewReader(`{"task_types": [`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid request body")
	// Refused before the inspector: nothing may have been revived.
	assert.Equal(t, "", insp.retryAllQueue)
}

// TestRetryAllArchivedRuntimeTasksWithoutBodyRetriesWholeQueue — CI filter:
// Runtime. Pins the "whole queue" default: a body-less POST must succeed and
// forward an empty (nil) task-type list, which the inspector reads as "every
// type in this queue". Without this test that contract is unpinned and a later
// "require a body" change would silently turn retry-all into a no-op.
func TestRetryAllArchivedRuntimeTasksWithoutBodyRetriesWholeQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	insp := &runtimeTaskTestInspector{retryAllResult: types.RuntimeRetryAllResult{
		Retried: 446, ByType: map[string]int{"summary:generation": 446},
	}}
	h := &SystemHandler{taskInspector: insp}
	r := newRetryAllRouter(h.RetryAllArchivedRuntimeTasks, h.ListArchivedRuntimeTaskFacets)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/q/summary/archived/actions/retry_all", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, types.QueueSummary, insp.retryAllQueue)
	assert.Nil(t, insp.retryAllTypes, "no body must forward an empty type list = whole queue")
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, float64(446), body["retried"])
}

// TestListArchivedRuntimeTaskFacetsDelegatesToInspector — CI filter: Runtime.
func TestListArchivedRuntimeTaskFacetsDelegatesToInspector(t *testing.T) {
	gin.SetMode(gin.TestMode)
	insp := &runtimeTaskTestInspector{facets: map[string]int{"summary:generation": 446}}
	h := &SystemHandler{taskInspector: insp}
	r := newRetryAllRouter(h.RetryAllArchivedRuntimeTasks, h.ListArchivedRuntimeTaskFacets)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/q/summary/archived/facets", nil))

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, types.QueueSummary, insp.facetsQueue)
	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.IsType(t, map[string]interface{}{}, body["by_type"])
	assert.Equal(t, float64(446), body["by_type"].(map[string]interface{})["summary:generation"])
}

// TestListArchivedRuntimeTaskFacetsRejectsUnknownQueue — CI filter: Runtime.
func TestListArchivedRuntimeTaskFacetsRejectsUnknownQueue(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &SystemHandler{}
	r := newRetryAllRouter(h.RetryAllArchivedRuntimeTasks, h.ListArchivedRuntimeTaskFacets)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/q/nope/archived/facets", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid queue")
}
