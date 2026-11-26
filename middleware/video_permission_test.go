package middleware

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kataras/iris/v12"
	"kandaoni.com/anqicms/config"
)

func TestVideoRoutesRequireContentMenuPermission(t *testing.T) {
	middlewares := videoPartyMiddlewares(t)
	hasStrict := false
	for _, middleware := range middlewares {
		if middleware == "middleware.AdminPermissionRequired" {
			hasStrict = true
		}
		if middleware == "middleware.AdminPermission" {
			t.Fatalf("video party must not rely on permissive AdminPermission: %v", middlewares)
		}
	}
	if !hasStrict {
		t.Fatalf("video party must use AdminPermissionRequired: %v", middlewares)
	}

	installMiddlewares := videoRouteMiddlewares(t, "/ffmpeg/install")
	hasSuperOnly := false
	for _, middleware := range installMiddlewares {
		if middleware == "middleware.AdminSuperRequired" {
			hasSuperOnly = true
		}
	}
	if !hasSuperOnly {
		t.Fatalf("ffmpeg installation must be super-admin only: %v", installMiddlewares)
	}

	for _, uri := range []string{
		"/setting/video/ffmpeg",
		"/setting/video/ffmpeg/install",
		"/setting/video/tasks",
		"/setting/video/tasks/retry",
		"/setting/video/tasks/historical",
	} {
		front, mapped := AdminMenuPathForURI(uri, config.MenuGroups())
		if !mapped || front != "/setting/content" {
			t.Errorf("%s mapped to %q (%v), want /setting/content", uri, front, mapped)
		}
	}

	withoutVideo := []*config.MenuGroup{{
		Key:  "setting",
		Name: "全局设置",
		Menus: []*config.Menu{{
			Path: "/setting/content", GroupKey: "setting", Name: "内容设置",
		}},
	}}
	if front, mapped := AdminMenuPathForURI("/setting/video/tasks", withoutVideo); mapped || front != "" {
		t.Fatalf("menu without content backend must remain unmapped: front=%q mapped=%v", front, mapped)
	}
	withContent := []*config.MenuGroup{{
		Key:  "setting",
		Name: "全局设置",
		Menus: []*config.Menu{{
			Path: "/setting/content", GroupKey: "setting", Name: "内容设置", Backend: "/setting/content",
		}},
	}}
	if front, mapped := AdminMenuPathForURI("/setting/video/tasks", withContent); !mapped || front != "/setting/content" {
		t.Fatalf("video routes must reuse content permission: front=%q mapped=%v", front, mapped)
	}
	if front, mapped := AdminMenuPathForURI("/setting/safe", withContent); mapped || front != "" {
		t.Fatalf("video alias must not grant access to other settings: front=%q mapped=%v", front, mapped)
	}
}

func TestAdminSuperRequiredStopsNonSuperBeforeInstall(t *testing.T) {
	for _, tt := range []struct {
		name       string
		adminID    uint
		wantStatus int
		wantRun    bool
	}{
		{name: "content administrator", adminID: 2, wantStatus: http.StatusForbidden},
		{name: "missing identity", adminID: 0, wantStatus: http.StatusForbidden},
		{name: "site super administrator", adminID: 1, wantStatus: http.StatusOK, wantRun: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ran := false
			app := iris.New()
			app.Use(func(ctx iris.Context) {
				ctx.Values().Set("adminId", tt.adminID)
				ctx.Next()
			})
			app.Post("/ffmpeg/install", AdminSuperRequired, func(ctx iris.Context) {
				ran = true
				_, _ = ctx.WriteString("installed")
			})
			if err := app.Build(); err != nil {
				t.Fatal(err)
			}
			rec := httptest.NewRecorder()
			app.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/ffmpeg/install", nil))
			if rec.Code != tt.wantStatus || ran != tt.wantRun {
				t.Fatalf("status=%d ran=%v, want status=%d ran=%v body=%q", rec.Code, ran, tt.wantStatus, tt.wantRun, rec.Body.String())
			}
		})
	}
}

func videoRouteMiddlewares(t *testing.T, route string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "../route/manage.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var middlewares []string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Post" {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Value != `"`+route+`"` {
			return true
		}
		for _, arg := range call.Args[1 : len(call.Args)-1] {
			var builder strings.Builder
			if err := printer.Fprint(&builder, fset, arg); err == nil {
				middlewares = append(middlewares, builder.String())
			}
		}
		return false
	})
	return middlewares
}

func videoPartyMiddlewares(t *testing.T) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "../route/manage.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var middlewares []string
	ast.Inspect(file, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			return true
		}
		identifier, ok := assign.Lhs[0].(*ast.Ident)
		if !ok || identifier.Name != "video" {
			return true
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Party" || len(call.Args) < 2 {
			return true
		}
		literal, ok := call.Args[0].(*ast.BasicLit)
		if !ok || literal.Value != `"/video"` {
			return true
		}
		for _, arg := range call.Args[1:] {
			var builder strings.Builder
			if err := printer.Fprint(&builder, fset, arg); err == nil {
				middlewares = append(middlewares, builder.String())
			}
		}
		return false
	})
	return middlewares
}
