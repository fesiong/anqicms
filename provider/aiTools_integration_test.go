package provider

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"testing"
)

func initTestSite(t *testing.T) *Website {
	dbSite, err := GetDBWebsiteInfo(1)
	if err != nil {
		t.Fatal(err)
	}
	dbSite.Status = 1
	InitWebsite(dbSite)
	w := GetWebsite(1)
	return w
}

// testServiceWithSite creates an AiChatService with a real database for integration testing.
func testServiceWithSite(t *testing.T, w *Website) *AiChatService {
	t.Helper()
	svc := &AiChatService{
		mu:       sync.RWMutex{},
		sessions: make(map[string]*ChatSession),
		Logger:   slog.Default(),
		mcpSrv:   nil,
		db:       w.DB,
		site:     w,
	}
	svc.Tools, svc.Handlers = svc.getEinoTools()
	return svc
}

func TestIntegration_ArchiveCRUD(t *testing.T) {
	w := initTestSite(t)
	svc := testServiceWithSite(t, w)
	ctx := context.Background()

	result, err := svc.Handlers["category_list"](ctx, `{}`)
	if err != nil {
		t.Fatalf("category_list failed: %v", err)
	}
	t.Logf("category_list result: %s", result)

	// todo
	catID := 12

	// 1. Create archive
	result, err = svc.Handlers["archive_create"](ctx, `{"title":"Integration Test Article","content":"This is test content for integration testing.","category_id":`+strconv.Itoa(int(catID))+`}`)
	if err != nil {
		t.Fatalf("archive_create failed: %v", err)
	}
	t.Logf("archive_create result: %s", result)

	// 2. List archives
	result, err = svc.Handlers["archive_list"](ctx, `{}`)
	if err != nil {
		t.Fatalf("archive_list failed: %v", err)
	}
	t.Logf("archive_list result: %s", result)

	// 3. Get archive detail (archive_id=1)
	result, err = svc.Handlers["archive_get"](ctx, `{"id":1}`)
	if err != nil {
		t.Fatalf("archive_get failed: %v", err)
	}
	t.Logf("archive_get result: %s", result)

	// 4. Publish archive
	result, err = svc.Handlers["archive_publish"](ctx, `{"id":1,"status":1}`)
	if err != nil {
		t.Fatalf("archive_publish failed: %v", err)
	}
	t.Logf("archive_publish result: %s", result)

	// Update Tag
	result, err = svc.Handlers["archive_tag_update"](ctx, `{"ids":[19,20],"tags": ["Golang", "Web"]}`)
	if err != nil {
		t.Fatalf("archive_publish failed: %v", err)
	}
	t.Logf("archive_publish result: %s", result)

	// 5. Delete archive
	result, err = svc.Handlers["archive_delete"](ctx, `{"id":1}`)
	if err != nil {
		t.Fatalf("archive_delete failed: %v", err)
	}
	t.Logf("archive_delete result: %s", result)
}

func TestIntegration_CategoryCRUD(t *testing.T) {
	w := initTestSite(t)
	svc := testServiceWithSite(t, w)
	ctx := context.Background()

	// 1. Create category
	result, err := svc.Handlers["category_create"](ctx, `{"title":"Technology","description":"Tech news and articles"}`)
	if err != nil {
		t.Fatalf("category_create failed: %v", err)
	}
	t.Logf("category_create result: %s", result)

	// 2. List categories
	result, err = svc.Handlers["category_list"](ctx, `{}`)
	if err != nil {
		t.Fatalf("category_list failed: %v", err)
	}
	t.Logf("category_list result: %s", result)

	// 3. Get category detail
	result, err = svc.Handlers["category_get"](ctx, `{"id":19}`)
	if err != nil {
		t.Fatalf("category_get failed: %v", err)
	}
	t.Logf("category_get result: %s", result)

	// 4. Delete category
	result, err = svc.Handlers["category_delete"](ctx, `{"id":19}`)
	if err != nil {
		t.Fatalf("category_delete failed: %v", err)
	}
	t.Logf("category_delete result: %s", result)
}

func TestIntegration_ModuleCRUD(t *testing.T) {
	w := initTestSite(t)
	svc := testServiceWithSite(t, w)
	ctx := context.Background()

	// 1. Create module
	result, err := svc.Handlers["module_create"](ctx, `{"title":"Technology","table_name":"tech"}`)
	if err != nil {
		t.Fatalf("module_create failed: %v", err)
	}
	t.Logf("module_create result: %s", result)

	// 2. List modules
	result, err = svc.Handlers["module_list"](ctx, `{}`)
	if err != nil {
		t.Fatalf("module_list failed: %v", err)
	}
	t.Logf("module_list result: %s", result)

	// 3. Get module detail
	result, err = svc.Handlers["module_get"](ctx, `{"id":4}`)
	if err != nil {
		t.Fatalf("module_get failed: %v", err)
	}
	t.Logf("module_get result: %s", result)

	// 4. Delete module
	result, err = svc.Handlers["module_delete"](ctx, `{"id":4}`)
	if err != nil {
		t.Fatalf("module_delete failed: %v", err)
	}
	t.Logf("module_delete result: %s", result)
}

func TestIntegration_TagCRUD(t *testing.T) {
	w := initTestSite(t)
	svc := testServiceWithSite(t, w)
	ctx := context.Background()

	// 1. Create tag
	result, err := svc.Handlers["tag_create"](ctx, `{"title":"golang","description":"Go programming language"}`)
	if err != nil {
		t.Fatalf("tag_create failed: %v", err)
	}
	t.Logf("tag_create result: %s", result)

	// 2. List tags
	result, err = svc.Handlers["tag_list"](ctx, `{}`)
	if err != nil {
		t.Fatalf("tag_list failed: %v", err)
	}
	t.Logf("tag_list result: %s", result)

	// 3. Get tag detail
	result, err = svc.Handlers["tag_get"](ctx, `{"id":6}`)
	if err != nil {
		t.Fatalf("tag_get failed: %v", err)
	}
	t.Logf("tag_get result: %s", result)

	// 4. Delete tag
	result, err = svc.Handlers["tag_delete"](ctx, `{"id":6}`)
	if err != nil {
		t.Fatalf("tag_delete failed: %v", err)
	}
	t.Logf("tag_delete result: %s", result)
}
