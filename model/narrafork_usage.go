// model/narrafork_usage.go
// 提供 NarraFork 额度详情所需的用户日/月消费汇总。
package model

import (
	"encoding/json"
	"fmt"
	"time"
)

type NarraForkUsageSummary struct {
	Available   bool  `json:"summaryAvailable"`
	TodayQuota  int64 `json:"todayQuota"`
	TodayTokens int64 `json:"todayTokens"`
	MonthQuota  int64 `json:"monthQuota"`
	MonthTokens int64 `json:"monthTokens"`
}

type NarraForkCacheHitRateSummary struct {
	InputTokens    int64
	CacheHitTokens int64
	Available      bool
	Complete       bool
}

// NarraForkDashboardCacheHitRateSummary 是模型调用分析页的缓存 Token 汇总。
type NarraForkDashboardCacheHitRateSummary struct {
	InputTokens      int64   `json:"input_tokens"`
	OutputTokens     int64   `json:"output_tokens"`
	CacheHitTokens   int64   `json:"cache_hit_tokens"`
	CacheWriteTokens int64   `json:"cache_write_tokens"`
	CacheInputTokens int64   `json:"cache_input_tokens"`
	TotalTokens      int64   `json:"total_tokens"`
	CacheHitRate     float64 `json:"cache_hit_rate"`
	Available        bool    `json:"available"`
	Complete         bool    `json:"complete"`
}

type narraForkUsageAggregate struct {
	Quota            int64 `gorm:"column:quota"`
	PromptTokens     int64 `gorm:"column:prompt_tokens"`
	CompletionTokens int64 `gorm:"column:completion_tokens"`
}

type narraForkCacheLogRow struct {
	PromptTokens     int64  `gorm:"column:prompt_tokens"`
	CompletionTokens int64  `gorm:"column:completion_tokens"`
	Other            string `gorm:"column:other"`
}

type narraForkDashboardCacheOther struct {
	CacheTokens           *int64 `json:"cache_tokens"`
	CacheWriteTokens      *int64 `json:"cache_write_tokens"`
	CacheCreationTokens   *int64 `json:"cache_creation_tokens"`
	CacheCreationTokens5m *int64 `json:"cache_creation_tokens_5m"`
	CacheCreationTokens1h *int64 `json:"cache_creation_tokens_1h"`
}

func GetNarraForkCacheHitRateSummary(userID int, startTimestamp int64, endTimestamp int64) (NarraForkCacheHitRateSummary, error) {
	if userID <= 0 {
		return NarraForkCacheHitRateSummary{}, fmt.Errorf("invalid NarraFork user id: %d", userID)
	}
	if LOG_DB == nil {
		return NarraForkCacheHitRateSummary{}, fmt.Errorf("log database is not initialized")
	}
	if endTimestamp < startTimestamp {
		return NarraForkCacheHitRateSummary{}, fmt.Errorf("invalid NarraFork cache hit rate time range")
	}

	var rows []narraForkCacheLogRow
	err := LOG_DB.Table("logs").
		Select("prompt_tokens, other").
		Where("user_id = ? AND type = ? AND created_at >= ? AND created_at <= ?", userID, LogTypeConsume, startTimestamp, endTimestamp).
		Find(&rows).Error
	if err != nil {
		return NarraForkCacheHitRateSummary{}, err
	}

	summary := NarraForkCacheHitRateSummary{Complete: true}
	for _, row := range rows {
		var other struct {
			CacheTokens *int64 `json:"cache_tokens"`
		}
		if err := json.Unmarshal([]byte(row.Other), &other); err != nil || other.CacheTokens == nil {
			summary.Complete = false
			continue
		}

		inputTokens := row.PromptTokens
		if inputTokens < 0 {
			inputTokens = 0
		}
		cacheTokens := *other.CacheTokens
		if cacheTokens < 0 {
			cacheTokens = 0
		}
		if cacheTokens > inputTokens {
			cacheTokens = inputTokens
		}
		summary.InputTokens += inputTokens
		summary.CacheHitTokens += cacheTokens
	}
	summary.Available = summary.Complete && summary.InputTokens > 0
	return summary, nil
}

// GetNarraForkDashboardCacheHitRateSummary 汇总模型调用分析页所需的缓存 Token。
// userID > 0 时按用户过滤；否则 username 非空时按用户名过滤；两者为空时汇总全站。
func GetNarraForkDashboardCacheHitRateSummary(userID int, username string, startTimestamp int64, endTimestamp int64) (NarraForkDashboardCacheHitRateSummary, error) {
	if LOG_DB == nil {
		return NarraForkDashboardCacheHitRateSummary{}, fmt.Errorf("log database is not initialized")
	}
	if endTimestamp < startTimestamp {
		return NarraForkDashboardCacheHitRateSummary{}, fmt.Errorf("invalid NarraFork dashboard cache hit rate time range")
	}

	query := LOG_DB.Table("logs").
		Select("prompt_tokens, completion_tokens, other").
		Where("type = ? AND created_at >= ? AND created_at <= ?", LogTypeConsume, startTimestamp, endTimestamp)
	if userID > 0 {
		query = query.Where("user_id = ?", userID)
	} else if username != "" {
		query = query.Where("username = ?", username)
	}

	var rows []narraForkCacheLogRow
	if err := query.Find(&rows).Error; err != nil {
		return NarraForkDashboardCacheHitRateSummary{}, err
	}

	summary := NarraForkDashboardCacheHitRateSummary{Complete: true}
	for _, row := range rows {
		inputTokens := nonNegativeNarraForkTokenCount(row.PromptTokens)
		outputTokens := nonNegativeNarraForkTokenCount(row.CompletionTokens)
		summary.InputTokens += inputTokens
		summary.OutputTokens += outputTokens

		var other narraForkDashboardCacheOther
		if err := json.Unmarshal([]byte(row.Other), &other); err != nil || other.CacheTokens == nil {
			summary.Complete = false
			continue
		}

		summary.CacheHitTokens += nonNegativeNarraForkTokenCount(*other.CacheTokens)
		summary.CacheWriteTokens += narraForkDashboardCacheWriteTokens(other)
	}

	summary.CacheInputTokens = summary.InputTokens + summary.CacheHitTokens + summary.CacheWriteTokens
	summary.TotalTokens = summary.CacheInputTokens + summary.OutputTokens
	summary.Available = summary.Complete && summary.CacheInputTokens > 0
	if summary.Available {
		summary.CacheHitRate = float64(summary.CacheHitTokens) / float64(summary.CacheInputTokens) * 100
		if summary.CacheHitRate > 100 {
			summary.CacheHitRate = 100
		}
	}
	return summary, nil
}

func narraForkDashboardCacheWriteTokens(other narraForkDashboardCacheOther) int64 {
	if other.CacheWriteTokens != nil {
		return nonNegativeNarraForkTokenCount(*other.CacheWriteTokens)
	}
	if other.CacheCreationTokens5m != nil || other.CacheCreationTokens1h != nil {
		return nonNegativeNarraForkTokenCount(pointerValue(other.CacheCreationTokens5m)) +
			nonNegativeNarraForkTokenCount(pointerValue(other.CacheCreationTokens1h))
	}
	if other.CacheCreationTokens != nil {
		return nonNegativeNarraForkTokenCount(*other.CacheCreationTokens)
	}
	return 0
}

func pointerValue(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}

func nonNegativeNarraForkTokenCount(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func GetNarraForkUsageSummary(userID int, now time.Time) (NarraForkUsageSummary, error) {
	if userID <= 0 {
		return NarraForkUsageSummary{}, fmt.Errorf("invalid NarraFork user id: %d", userID)
	}
	if LOG_DB == nil {
		return NarraForkUsageSummary{}, fmt.Errorf("log database is not initialized")
	}

	location := now.Location()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, location)
	endTimestamp := now.Unix()

	today, err := getNarraForkUsageAggregate(userID, dayStart.Unix(), endTimestamp)
	if err != nil {
		return NarraForkUsageSummary{}, err
	}
	month, err := getNarraForkUsageAggregate(userID, monthStart.Unix(), endTimestamp)
	if err != nil {
		return NarraForkUsageSummary{}, err
	}

	return NarraForkUsageSummary{
		Available:   true,
		TodayQuota:  today.Quota,
		TodayTokens: today.PromptTokens + today.CompletionTokens,
		MonthQuota:  month.Quota,
		MonthTokens: month.PromptTokens + month.CompletionTokens,
	}, nil
}

func getNarraForkUsageAggregate(userID int, startTimestamp int64, endTimestamp int64) (narraForkUsageAggregate, error) {
	var aggregate narraForkUsageAggregate
	err := LOG_DB.Table("logs").
		Select("COALESCE(SUM(quota), 0) AS quota, COALESCE(SUM(prompt_tokens), 0) AS prompt_tokens, COALESCE(SUM(completion_tokens), 0) AS completion_tokens").
		Where("user_id = ? AND type = ? AND created_at >= ? AND created_at <= ?", userID, LogTypeConsume, startTimestamp, endTimestamp).
		Scan(&aggregate).Error
	return aggregate, err
}
