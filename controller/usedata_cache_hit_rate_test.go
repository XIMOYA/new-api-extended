// controller/usedata_cache_hit_rate_test.go
// 验证模型调用分析页缓存统计接口的时间范围、用户隔离和管理员筛选。
package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type dashboardCacheHitRateResponse struct {
	Success bool                                        `json:"success"`
	Message string                                      `json:"message"`
	Data    model.NarraForkDashboardCacheHitRateSummary `json:"data"`
}

func setupDashboardCacheHitRateTestDB(t *testing.T) {
	t.Helper()
	oldLogDB := model.LOG_DB
	testDB, err := gorm.Open(sqlite.Open("file:controller_dashboard_cache_hit_rate_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, testDB.AutoMigrate(&model.Log{}))
	model.LOG_DB = testDB
	t.Cleanup(func() {
		model.LOG_DB = oldLogDB
		db, closeErr := testDB.DB()
		if closeErr == nil {
			_ = db.Close()
		}
	})

	now := time.Now().Unix()
	require.NoError(t, testDB.Create(&[]model.Log{
		{UserId: 7, Username: "alice", CreatedAt: now - 60, Type: model.LogTypeConsume, PromptTokens: 100, CompletionTokens: 10, Other: `{"cache_tokens":300,"cache_write_tokens":20}`},
		{UserId: 8, Username: "bob", CreatedAt: now - 60, Type: model.LogTypeConsume, PromptTokens: 200, CompletionTokens: 20, Other: `{"cache_tokens":200,"cache_write_tokens":40}`},
	}).Error)
}

func decodeDashboardCacheHitRateResponse(t *testing.T, recorder *httptest.ResponseRecorder) dashboardCacheHitRateResponse {
	t.Helper()
	require.Equal(t, http.StatusOK, recorder.Code)
	var payload dashboardCacheHitRateResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success, payload.Message)
	return payload
}

func TestGetUserCacheHitRateStatsRestrictsToAuthenticatedUser(t *testing.T) {
	setupDashboardCacheHitRateTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("id", 7)
	startTimestamp := time.Now().Add(-time.Hour).Unix()
	endTimestamp := time.Now().Add(time.Hour).Unix()
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/data/cache-hit-rate/self?start_timestamp=%d&end_timestamp=%d", startTimestamp, endTimestamp),
		nil,
	)

	GetUserCacheHitRateStats(ctx)

	payload := decodeDashboardCacheHitRateResponse(t, recorder)
	require.Equal(t, int64(100), payload.Data.InputTokens)
	require.Equal(t, int64(10), payload.Data.OutputTokens)
	require.Equal(t, int64(300), payload.Data.CacheHitTokens)
	require.Equal(t, int64(20), payload.Data.CacheWriteTokens)
}

func TestGetCacheHitRateStatsSupportsAdminUsernameFilter(t *testing.T) {
	setupDashboardCacheHitRateTestDB(t)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Set("role", common.RoleAdminUser)
	startTimestamp := time.Now().Add(-time.Hour).Unix()
	endTimestamp := time.Now().Add(time.Hour).Unix()
	ctx.Request = httptest.NewRequest(
		http.MethodGet,
		fmt.Sprintf("/api/data/cache-hit-rate?username=bob&start_timestamp=%d&end_timestamp=%d", startTimestamp, endTimestamp),
		nil,
	)

	GetCacheHitRateStats(ctx)

	payload := decodeDashboardCacheHitRateResponse(t, recorder)
	require.Equal(t, int64(200), payload.Data.InputTokens)
	require.Equal(t, int64(20), payload.Data.OutputTokens)
	require.Equal(t, int64(200), payload.Data.CacheHitTokens)
	require.Equal(t, int64(40), payload.Data.CacheWriteTokens)
}
