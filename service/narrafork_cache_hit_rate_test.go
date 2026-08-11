// service/narrafork_cache_hit_rate_test.go
// 验证 NarraFork 缓存命中率按请求、今日和最近 N 天窗口切换。
package service

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestBuildNarraForkQuotaDetailsCacheHitRateWindows(t *testing.T) {
	oldLogDB := model.LOG_DB
	testDB, err := gorm.Open(sqlite.Open("file:narrafork_cache_hit_rate_service_test?mode=memory&cache=shared"), &gorm.Config{})
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

	now := time.Now()
	require.NoError(t, testDB.Create(&[]model.Log{
		{UserId: 7, CreatedAt: now.Add(-time.Hour).Unix(), Type: model.LogTypeConsume, PromptTokens: 1000, Other: `{"cache_tokens":500}`},
		{UserId: 7, CreatedAt: now.Add(-25 * time.Hour).Unix(), Type: model.LogTypeConsume, PromptTokens: 2000, Other: `{"cache_tokens":1000}`},
	}).Error)

	info := &relaycommon.RelayInfo{
		UserId: 7,
		NarraForkQuotaEvent: relaycommon.NarraForkQuotaEventConfig{
			ShowCacheHitRate:  true,
			CacheHitRateScope: narrafork_setting.CacheHitRateScopeToday,
			CacheHitRateDays:  narrafork_setting.DefaultCacheHitRateDays,
		},
	}
	details := buildNarraForkQuotaDetails(info, &dto.Usage{PromptTokens: 1000, PromptCacheHitTokens: 100}, 0)
	require.True(t, details.CacheHitRateAvailable)
	require.InDelta(t, 30, details.CacheHitRate, 0.0001)
	require.Equal(t, "今日", details.CacheHitRatePeriod)

	info.NarraForkQuotaEvent.CacheHitRateScope = narrafork_setting.CacheHitRateScopeRecentDays
	info.NarraForkQuotaEvent.CacheHitRateDays = 3
	details = buildNarraForkQuotaDetails(info, &dto.Usage{PromptTokens: 1000, PromptCacheHitTokens: 100}, 0)
	require.True(t, details.CacheHitRateAvailable)
	require.InDelta(t, 40, details.CacheHitRate, 0.0001)
	require.Equal(t, "近3天", details.CacheHitRatePeriod)

	lines := appendNarraForkQuotaDetailLines(nil, "$1.00", 0, info, details)
	require.Contains(t, lines, "缓存命中率（近3天）: 40.00%")
}

func TestBuildNarraForkQuotaDetailsRequestCacheHitRatePreservesCurrentBehavior(t *testing.T) {
	info := &relaycommon.RelayInfo{
		NarraForkQuotaEvent: relaycommon.NarraForkQuotaEventConfig{
			CacheHitRateScope: narrafork_setting.CacheHitRateScopeRequest,
		},
	}
	details := buildNarraForkQuotaDetails(info, &dto.Usage{PromptTokens: 1000, PromptCacheHitTokens: 250}, 0)
	require.True(t, details.CacheHitRateAvailable)
	require.InDelta(t, 25, details.CacheHitRate, 0.0001)
	require.Equal(t, "当次请求", details.CacheHitRatePeriod)
}
