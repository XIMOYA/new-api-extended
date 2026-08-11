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

type narraForkUsageAggregate struct {
	Quota            int64 `gorm:"column:quota"`
	PromptTokens     int64 `gorm:"column:prompt_tokens"`
	CompletionTokens int64 `gorm:"column:completion_tokens"`
}

type narraForkCacheLogRow struct {
	PromptTokens int64  `gorm:"column:prompt_tokens"`
	Other        string `gorm:"column:other"`
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
