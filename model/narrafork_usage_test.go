// model/narrafork_usage_test.go
// 验证 NarraFork 用户日/月消费汇总的时间边界、用户过滤与 Token 汇总。
package model

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestGetNarraForkUsageSummary(t *testing.T) {
	oldLogDB := LOG_DB
	testDB, err := gorm.Open(sqlite.Open("file:narrafork_usage_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, testDB.AutoMigrate(&Log{}))
	LOG_DB = testDB
	t.Cleanup(func() {
		LOG_DB = oldLogDB
		db, closeErr := testDB.DB()
		if closeErr == nil {
			_ = db.Close()
		}
	})

	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, time.August, 9, 14, 0, 0, 0, location)
	logs := []Log{
		{UserId: 7, CreatedAt: now.Add(-time.Hour).Unix(), Type: LogTypeConsume, Quota: 100, PromptTokens: 10, CompletionTokens: 20},
		{UserId: 7, CreatedAt: now.Add(-48 * time.Hour).Unix(), Type: LogTypeConsume, Quota: 200, PromptTokens: 30, CompletionTokens: 40},
		{UserId: 7, CreatedAt: now.Add(-35 * 24 * time.Hour).Unix(), Type: LogTypeConsume, Quota: 999, PromptTokens: 90, CompletionTokens: 10},
		{UserId: 8, CreatedAt: now.Add(-time.Hour).Unix(), Type: LogTypeConsume, Quota: 500, PromptTokens: 50, CompletionTokens: 50},
		{UserId: 7, CreatedAt: now.Add(-time.Hour).Unix(), Type: LogTypeLogin, Quota: 700, PromptTokens: 70, CompletionTokens: 70},
	}
	require.NoError(t, testDB.Create(&logs).Error)

	summary, err := GetNarraForkUsageSummary(7, now)
	require.NoError(t, err)
	require.True(t, summary.Available)
	require.Equal(t, int64(100), summary.TodayQuota)
	require.Equal(t, int64(30), summary.TodayTokens)
	require.Equal(t, int64(300), summary.MonthQuota)
	require.Equal(t, int64(100), summary.MonthTokens)
}

func TestGetNarraForkCacheHitRateSummary(t *testing.T) {
	oldLogDB := LOG_DB
	testDB, err := gorm.Open(sqlite.Open("file:narrafork_cache_hit_rate_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, testDB.AutoMigrate(&Log{}))
	LOG_DB = testDB
	t.Cleanup(func() {
		LOG_DB = oldLogDB
		db, closeErr := testDB.DB()
		if closeErr == nil {
			_ = db.Close()
		}
	})

	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, time.August, 11, 14, 0, 0, 0, location)
	logs := []Log{
		{UserId: 7, CreatedAt: now.Add(-time.Hour).Unix(), Type: LogTypeConsume, PromptTokens: 1000, Other: `{"cache_tokens":250}`},
		{UserId: 7, CreatedAt: now.Add(-25 * time.Hour).Unix(), Type: LogTypeConsume, PromptTokens: 2000, Other: `{"cache_tokens":1000}`},
		{UserId: 7, CreatedAt: now.Add(-8 * 24 * time.Hour).Unix(), Type: LogTypeConsume, PromptTokens: 10000, Other: `{"cache_tokens":9000}`},
		{UserId: 8, CreatedAt: now.Add(-time.Hour).Unix(), Type: LogTypeConsume, PromptTokens: 5000, Other: `{"cache_tokens":5000}`},
		{UserId: 7, CreatedAt: now.Add(-time.Hour).Unix(), Type: LogTypeLogin, PromptTokens: 9000, Other: `{"cache_tokens":9000}`},
	}
	require.NoError(t, testDB.Create(&logs).Error)

	today, err := GetNarraForkCacheHitRateSummary(7, time.Date(2026, time.August, 11, 0, 0, 0, 0, location).Unix(), now.Unix())
	require.NoError(t, err)
	require.True(t, today.Complete)
	require.True(t, today.Available)
	require.Equal(t, int64(1000), today.InputTokens)
	require.Equal(t, int64(250), today.CacheHitTokens)

	recent, err := GetNarraForkCacheHitRateSummary(7, time.Date(2026, time.August, 5, 0, 0, 0, 0, location).Unix(), now.Unix())
	require.NoError(t, err)
	require.True(t, recent.Complete)
	require.Equal(t, int64(3000), recent.InputTokens)
	require.Equal(t, int64(1250), recent.CacheHitTokens)
}

func TestGetNarraForkCacheHitRateSummaryMarksMissingCacheData(t *testing.T) {
	oldLogDB := LOG_DB
	testDB, err := gorm.Open(sqlite.Open("file:narrafork_cache_hit_rate_missing_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, testDB.AutoMigrate(&Log{}))
	LOG_DB = testDB
	t.Cleanup(func() {
		LOG_DB = oldLogDB
		db, closeErr := testDB.DB()
		if closeErr == nil {
			_ = db.Close()
		}
	})

	now := time.Date(2026, time.August, 11, 14, 0, 0, 0, time.FixedZone("UTC+8", 8*60*60))
	require.NoError(t, testDB.Create(&Log{
		UserId:       7,
		CreatedAt:    now.Add(-time.Hour).Unix(),
		Type:         LogTypeConsume,
		PromptTokens: 1000,
		Other:        `{}`,
	}).Error)

	summary, err := GetNarraForkCacheHitRateSummary(7, now.Add(-24*time.Hour).Unix(), now.Unix())
	require.NoError(t, err)
	require.False(t, summary.Complete)
	require.False(t, summary.Available)
}
