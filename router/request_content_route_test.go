// router/request_content_route_test.go
// 守护请求记录路由的中间件组合：只保留登录鉴权与 CORS，不再挂 IP 维度的 CriticalRateLimit，
// 避免浏览请求记录把登录/刷新共用的限流额度耗尽而整站爆 429。
package router

import (
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requestContentRouteMiddlewares(t *testing.T) []string {
	t.Helper()
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, "api-router.go", nil, parser.SkipObjectResolution)
	require.NoError(t, err)

	var middlewares []string
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Use" {
			return true
		}
		receiver, ok := selector.X.(*ast.Ident)
		if !ok || receiver.Name != "requestContentRoute" {
			return true
		}

		found = true
		for _, argument := range call.Args {
			switch typed := argument.(type) {
			case *ast.CallExpr:
				if inner, ok := typed.Fun.(*ast.SelectorExpr); ok {
					middlewares = append(middlewares, inner.Sel.Name)
				}
			case *ast.SelectorExpr:
				middlewares = append(middlewares, typed.Sel.Name)
			}
		}
		return false
	})

	require.True(t, found, "未找到 requestContentRoute.Use 调用")
	return middlewares
}

func TestRequestContentRouteKeepsAuthWithoutCriticalRateLimit(t *testing.T) {
	middlewares := requestContentRouteMiddlewares(t)

	assert.Contains(t, middlewares, "UserAuth")
	assert.Contains(t, middlewares, "CORS")
	assert.Contains(t, middlewares, "RequestContentAuditNoStore")
	assert.NotContains(t, middlewares, "CriticalRateLimit")
	assert.NotContains(t, middlewares, "UserCriticalRateLimit")
}

func TestRequestContentRoutesRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	routes := make(map[string]struct{}, len(engine.Routes()))
	for _, route := range engine.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}

	for _, path := range []string{
		"/api/request-records/",
		"/api/request-records/:id",
		"/api/request-records/:id/view",
		"/api/request-records/:id/content",
	} {
		_, registered := routes[http.MethodGet+" "+path]
		assert.True(t, registered, "缺少路由 %s", path)
	}
}
