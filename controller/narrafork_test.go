// controller/narrafork_test.go
// 验证 NarraFork 预览和一次性 SSE 测试接口输出。
package controller

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestPreviewNarraForkEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest("POST", "/api/option/narrafork/preview", strings.NewReader(`{"config":{"detail_template":"{{balance}}"}}`))
	request.Header.Set("Content-Type", "application/json")
	writer := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(writer)
	context.Request = request

	PreviewNarraForkEvent(context)

	require.Equal(t, 200, writer.Code)
	require.NotContains(t, writer.Body.String(), "protocolVersion")
	require.NotContains(t, writer.Body.String(), "metrics")
	require.Contains(t, writer.Body.String(), "quotaBalance")
	require.Contains(t, writer.Body.String(), "detailedQuotaBalance")
}

func TestTestNarraForkEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest("POST", "/api/option/narrafork/test", strings.NewReader(`{"config":{}}`))
	request.Header.Set("Content-Type", "application/json")
	writer := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(writer)
	context.Request = request

	TestNarraForkEvent(context)

	require.Equal(t, 200, writer.Code)
	require.Contains(t, writer.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, writer.Body.String(), "event: quotaBalanceEvent")
}
