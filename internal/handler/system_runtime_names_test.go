package handler

import (
	"context"
	"fmt"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type fakeRuntimeKBNamer struct {
	bases map[string]*types.KnowledgeBase
}

func (f fakeRuntimeKBNamer) GetKnowledgeBasesByIDsOnly(_ context.Context, _ []string) ([]*types.KnowledgeBase, error) {
	rows := make([]*types.KnowledgeBase, 0, len(f.bases))
	for _, base := range f.bases {
		rows = append(rows, base)
	}
	return rows, nil
}

type fakeRuntimeKnowledgeNamer struct {
	rows map[string]*types.Knowledge
}

func (f fakeRuntimeKnowledgeNamer) GetKnowledgeByIDOnly(_ context.Context, id string) (*types.Knowledge, error) {
	return f.rows[id], nil
}

// fakeRuntimeTenantNamer embeds the wide interface so only the one method
// the console needs has to be implemented.
type fakeRuntimeTenantNamer struct {
	interfaces.TenantService
	tenants map[uint64]*types.Tenant
}

func (f fakeRuntimeTenantNamer) GetTenantsByIDs(_ context.Context, _ []uint64) (map[uint64]*types.Tenant, error) {
	return f.tenants, nil
}

type fakeRuntimeModelNamer struct {
	byTenant map[uint64]map[string]*types.Model
	// calls records the workspace each lookup ran under, pinning that the
	// console scopes model reads to the task's workspace.
	calls []runtimeTaskModelKey
}

func (f *fakeRuntimeModelNamer) GetModelByID(ctx context.Context, id string) (*types.Model, error) {
	tenantID, _ := types.TenantIDFromContext(ctx)
	f.calls = append(f.calls, runtimeTaskModelKey{tenantID: tenantID, modelID: id})
	models := f.byTenant[tenantID]
	if models == nil {
		return nil, nil
	}
	return models[id], nil
}

type fakeRuntimeDataSourceNamer struct {
	sources map[string]*types.DataSource
}

func (f fakeRuntimeDataSourceNamer) GetDataSource(_ context.Context, id string) (*types.DataSource, error) {
	return f.sources[id], nil
}

func TestEnrichRuntimeTaskNamesResolvesEveryIDField(t *testing.T) {
	modelNamer := &fakeRuntimeModelNamer{
		byTenant: map[uint64]map[string]*types.Model{
			10000: {
				"model-1": {ID: "model-1", Name: "BAAI/bge-m3"},
				"model-2": {ID: "model-2", Name: "deepseek-v4.1-flash", DisplayName: "DeepSeek Flash"},
			},
		},
	}
	h := &SystemHandler{
		kbSvc: fakeRuntimeKBNamer{bases: map[string]*types.KnowledgeBase{
			"kb-1": {ID: "kb-1", Name: "盘古SRM产品1.69"},
			"kb-2": {ID: "kb-2", Name: "大北农-饲料库"},
		}},
		knowledgeNameSvc: fakeRuntimeKnowledgeNamer{rows: map[string]*types.Knowledge{
			"doc-1": {ID: "doc-1", Title: ""},
			"doc-2": {ID: "doc-2", Title: "采购需求说明.pdf"},
		}},
		tenantSvc: fakeRuntimeTenantNamer{tenants: map[uint64]*types.Tenant{
			10000: {ID: 10000, Name: "服务商"},
		}},
		modelSvc: modelNamer,
		dataSourceSvc: fakeRuntimeDataSourceNamer{sources: map[string]*types.DataSource{
			"ds-1": {ID: "ds-1", Name: "飞书知识库同步"},
		}},
	}
	tasks := []types.RuntimeTaskInfo{
		{
			ID: "task-1", Type: types.TypeSummaryGeneration, TenantID: 10000,
			KnowledgeBaseID: "kb-1", KnowledgeID: "doc-1", DataSourceID: "ds-1",
			KnowledgeFileName: "采购需求说明.pdf",
			Models:            []types.RuntimeTaskModel{{Role: types.RuntimeTaskModelRoleSummary, ID: "model-2"}},
		},
		{
			ID: "task-2", Type: types.TypeKnowledgeMove, TenantID: 10000,
			SourceKBID: "kb-1", TargetKBID: "kb-2",
			Models: []types.RuntimeTaskModel{{Role: types.RuntimeTaskModelRoleEmbedding, ID: "model-1"}},
		},
		{
			ID: "task-3", Type: types.TypeKBClone,
			SourceID: "kb-1", TargetID: "kb-2",
		},
	}
	h.enrichRuntimeTaskNames(context.Background(), tasks)

	if tasks[0].KnowledgeBaseName != "盘古SRM产品1.69" {
		t.Fatalf("knowledge base name = %q", tasks[0].KnowledgeBaseName)
	}
	if tasks[0].KnowledgeName != "" {
		t.Fatalf("empty title must stay empty in the backend, got %q", tasks[0].KnowledgeName)
	}
	if tasks[0].TenantName != "服务商" {
		t.Fatalf("workspace name = %q", tasks[0].TenantName)
	}
	if tasks[0].DataSourceName != "飞书知识库同步" {
		t.Fatalf("data source name = %q", tasks[0].DataSourceName)
	}
	if len(tasks[0].Models) != 1 || tasks[0].Models[0].Name != "DeepSeek Flash" {
		t.Fatalf("model display name = %+v", tasks[0].Models)
	}
	if tasks[1].SourceKBName != "盘古SRM产品1.69" || tasks[1].TargetKBName != "大北农-饲料库" {
		t.Fatalf("move kb names = %q -> %q", tasks[1].SourceKBName, tasks[1].TargetKBName)
	}
	if tasks[1].Models[0].Name != "BAAI/bge-m3" {
		t.Fatalf("embedding model name = %+v", tasks[1].Models)
	}
	// kb:clone carries the two knowledge base IDs in the generic
	// source/target slots.
	if tasks[2].SourceKBName != "盘古SRM产品1.69" || tasks[2].TargetKBName != "大北农-饲料库" {
		t.Fatalf("clone kb names = %q -> %q", tasks[2].SourceKBName, tasks[2].TargetKBName)
	}
	// Every model lookup must run under the task's own workspace.
	for _, call := range modelNamer.calls {
		if call.tenantID != 10000 {
			t.Fatalf("model lookup ran under workspace %d", call.tenantID)
		}
	}
}

// TestEnrichRuntimeTaskNamesCoversAFullPage guards the lookup cap: a page at
// the API ceiling (100 tasks) must name every document. A cap below the page
// size silently leaves trailing rows with IDs only — which is exactly what
// the first deployment of this feature did.
func TestEnrichRuntimeTaskNamesCoversAFullPage(t *testing.T) {
	rows := make(map[string]*types.Knowledge, runtimeTaskNameLookupLimit)
	tasks := make([]types.RuntimeTaskInfo, 0, runtimeTaskNameLookupLimit)
	for i := 0; i < runtimeTaskNameLookupLimit; i++ {
		id := fmt.Sprintf("doc-%03d", i)
		rows[id] = &types.Knowledge{ID: id, Title: "文档 " + id}
		tasks = append(tasks, types.RuntimeTaskInfo{ID: id, TenantID: 1, KnowledgeID: id})
	}
	h := &SystemHandler{knowledgeNameSvc: fakeRuntimeKnowledgeNamer{rows: rows}}
	h.enrichRuntimeTaskNames(context.Background(), tasks)
	for _, task := range tasks {
		if task.KnowledgeName == "" {
			t.Fatalf("task %s lost its name on a full page", task.ID)
		}
	}
}

func TestEnrichRuntimeTaskNamesFallsBackToEmptyWhenUnwired(t *testing.T) {
	h := &SystemHandler{}
	tasks := []types.RuntimeTaskInfo{{
		ID: "task-1", TenantID: 1, KnowledgeBaseID: "kb-1", KnowledgeID: "doc-1",
		Models: []types.RuntimeTaskModel{{Role: types.RuntimeTaskModelRoleChat, ID: "m-1"}},
	}}
	h.enrichRuntimeTaskNames(context.Background(), tasks)
	if tasks[0].KnowledgeBaseName != "" || tasks[0].KnowledgeName != "" ||
		tasks[0].TenantName != "" || tasks[0].Models[0].Name != "" {
		t.Fatalf("unwired handler must leave names empty: %+v", tasks[0])
	}
}
