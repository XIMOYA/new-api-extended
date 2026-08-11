// service/narrafork_quota_details.go
// 构造 NarraFork 额度事件的 Token、缓存命中率与日/月消费详情。
package service

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
)

type narraForkTokenStats struct {
	InputTokens      int64
	CacheInputTokens int64
	OutputTokens     int64
	TotalTokens      int64
	CacheHitTokens   int64
	CacheHitRate     float64
	ReasoningTokens  int64
}

func (stats narraForkTokenStats) HasData() bool {
	return stats.InputTokens > 0 || stats.OutputTokens > 0 || stats.TotalTokens > 0 ||
		stats.CacheHitTokens > 0 || stats.ReasoningTokens > 0
}

type narraForkQuotaDetails struct {
	TokenStats            narraForkTokenStats
	Metrics               NarraForkEventMetrics
	UsageSummary          model.NarraForkUsageSummary
	SummaryAvailable      bool
	CacheHitRate          float64
	CacheHitRateAvailable bool
	CacheHitRatePeriod    string
	TotalQuota            int64
	UsedQuota             int64
	TotalAvailable        bool
}

func buildNarraForkQuotaDetails(info *relaycommon.RelayInfo, usage *dto.Usage, requestQuota int) narraForkQuotaDetails {
	tokenStats := narraForkTokenStatistics(usage)
	details := narraForkQuotaDetails{
		TokenStats:            tokenStats,
		CacheHitRate:          tokenStats.CacheHitRate,
		CacheHitRateAvailable: tokenStats.CacheInputTokens > 0,
		CacheHitRatePeriod:    narraForkCacheHitRatePeriod(nil),
	}
	if info == nil {
		return details
	}

	details.CacheHitRatePeriod = narraForkCacheHitRatePeriod(&info.NarraForkQuotaEvent)
	now := time.Now()
	if info.UserId > 0 {
		applyNarraForkCacheHitRateWindow(&details, info, now)

		summary, err := model.GetNarraForkUsageSummary(info.UserId, now)
		if err != nil {
			common.SysLog(fmt.Sprintf("failed to build NarraFork usage summary: %v", err))
		} else {
			if summary.Available {
				// 当前请求的消费日志在额度事件之后写入，因此显式补入当前请求。
				summary.TodayQuota += int64(requestQuota)
				summary.MonthQuota += int64(requestQuota)
				summary.TodayTokens += details.TokenStats.TotalTokens
				summary.MonthTokens += details.TokenStats.TotalTokens
			}
			details.UsageSummary = summary
			details.SummaryAvailable = summary.Available
		}
	}
	details.TotalQuota, details.UsedQuota, details.TotalAvailable = resolveNarraForkTotalQuota(info, requestQuota)
	return details
}

func narraForkCacheHitRatePeriod(config *relaycommon.NarraForkQuotaEventConfig) string {
	if config == nil {
		return "当次请求"
	}
	scope := narrafork_setting.NormalizeCacheHitRateScope(config.CacheHitRateScope)
	switch scope {
	case narrafork_setting.CacheHitRateScopeToday:
		return "今日"
	case narrafork_setting.CacheHitRateScopeRecentDays:
		return fmt.Sprintf("近%d天", narrafork_setting.NormalizeCacheHitRateDays(config.CacheHitRateDays))
	default:
		return "当次请求"
	}
}

func applyNarraForkCacheHitRateWindow(details *narraForkQuotaDetails, info *relaycommon.RelayInfo, now time.Time) {
	if details == nil || info == nil {
		return
	}
	config := info.NarraForkQuotaEvent
	scope := narrafork_setting.NormalizeCacheHitRateScope(config.CacheHitRateScope)
	if scope == narrafork_setting.CacheHitRateScopeRequest {
		return
	}

	location := now.Location()
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	if scope == narrafork_setting.CacheHitRateScopeRecentDays {
		days := narrafork_setting.NormalizeCacheHitRateDays(config.CacheHitRateDays)
		start = start.AddDate(0, 0, -(days - 1))
	}
	aggregate, err := model.GetNarraForkCacheHitRateSummary(info.UserId, start.Unix(), now.Unix())
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to build NarraFork cache hit rate summary: %v", err))
		details.CacheHitRateAvailable = false
		return
	}

	aggregate.InputTokens += details.TokenStats.CacheInputTokens
	aggregate.CacheHitTokens += details.TokenStats.CacheHitTokens
	details.CacheHitRateAvailable = aggregate.Complete && aggregate.InputTokens > 0
	if details.CacheHitRateAvailable {
		details.CacheHitRate = float64(aggregate.CacheHitTokens) / float64(aggregate.InputTokens) * 100
		if details.CacheHitRate > 100 {
			details.CacheHitRate = 100
		}
	}
}

func narraForkTokenStatistics(usage *dto.Usage) narraForkTokenStats {
	if usage == nil {
		return narraForkTokenStats{}
	}

	inputTokenCount := usage.PromptTokens
	if inputTokenCount <= 0 {
		inputTokenCount = usage.InputTokens
	}
	cacheInputTokenCount := inputTokenCount
	cacheInputTokens := int64(cacheInputTokenCount)
	outputTokenCount := usage.CompletionTokens
	if outputTokenCount <= 0 {
		outputTokenCount = usage.OutputTokens
	}
	inputTokens := int64(inputTokenCount)
	outputTokens := int64(outputTokenCount)
	totalTokens := int64(usage.TotalTokens)
	if totalTokens <= 0 {
		totalTokens = inputTokens + outputTokens
	}

	cacheHitTokens := int64(usage.PromptCacheHitTokens)
	cacheHitTokens = maxInt64(cacheHitTokens, int64(usage.PromptTokensDetails.CachedTokens))
	if usage.InputTokensDetails != nil {
		cacheHitTokens = maxInt64(cacheHitTokens, int64(usage.InputTokensDetails.CachedTokens))
	}
	if cacheHitTokens > cacheInputTokens {
		cacheHitTokens = cacheInputTokens
	}

	cacheHitRate := 0.0
	if cacheInputTokens > 0 {
		cacheHitRate = float64(cacheHitTokens) / float64(cacheInputTokens) * 100
	}
	if cacheHitRate > 100 {
		cacheHitRate = 100
	}

	return narraForkTokenStats{
		InputTokens:      inputTokens,
		CacheInputTokens: cacheInputTokens,
		OutputTokens:     outputTokens,
		TotalTokens:      totalTokens,
		CacheHitTokens:   cacheHitTokens,
		CacheHitRate:     cacheHitRate,
		ReasoningTokens:  int64(usage.CompletionTokenDetails.ReasoningTokens),
	}
}

func resolveNarraForkTotalQuota(info *relaycommon.RelayInfo, requestQuota int) (total int64, used int64, available bool) {
	if info == nil {
		return 0, 0, false
	}

	config := info.NarraForkQuotaEvent
	if config.BalanceSource == narrafork_setting.BalanceSourceCustom ||
		config.BalanceSource == narrafork_setting.BalanceSourceTokenQuota {
		return 0, 0, false
	}

	if config.BalanceSource == narrafork_setting.BalanceSourceEffective &&
		info.BillingSource == BillingSourceSubscription && info.SubscriptionAmountTotal > 0 {
		used = info.SubscriptionAmountUsedAfterPreConsume + int64(requestQuota-info.FinalPreConsumedQuota)
		if used < 0 {
			used = 0
		}
		return info.SubscriptionAmountTotal, used, true
	}

	if info.UserId <= 0 {
		return 0, 0, false
	}
	usedBefore, err := model.GetUserUsedQuota(info.UserId)
	if err != nil {
		common.SysLog(fmt.Sprintf("failed to read NarraFork user used quota: %v", err))
		return 0, 0, false
	}
	remaining := int64(info.UserQuota) - int64(requestQuota)
	if remaining < 0 {
		remaining = 0
	}
	used = int64(usedBefore) + int64(requestQuota)
	return remaining + used, used, true
}

func formatNarraForkTokens(value int64, mode string) string {
	if value < 0 {
		value = 0
	}
	if narrafork_setting.NormalizeTokenDisplayMode(mode) == narrafork_setting.TokenDisplayModeCompact {
		return formatCompactNarraForkTokens(value)
	}
	return formatExactNarraForkTokens(value)
}

func formatExactNarraForkTokens(value int64) string {
	text := strconv.FormatInt(value, 10)
	if len(text) <= 3 {
		return text
	}

	first := len(text) % 3
	if first == 0 {
		first = 3
	}
	var builder strings.Builder
	builder.WriteString(text[:first])
	for index := first; index < len(text); index += 3 {
		builder.WriteByte(',')
		builder.WriteString(text[index : index+3])
	}
	return builder.String()
}

func formatCompactNarraForkTokens(value int64) string {
	units := []struct {
		threshold float64
		suffix    string
	}{
		{threshold: 1_000_000_000, suffix: "B"},
		{threshold: 1_000_000, suffix: "M"},
		{threshold: 1_000, suffix: "K"},
	}
	amount := float64(value)
	for _, unit := range units {
		if amount >= unit.threshold {
			formatted := strconv.FormatFloat(amount/unit.threshold, 'f', 2, 64)
			formatted = strings.TrimRight(strings.TrimRight(formatted, "0"), ".")
			return formatted + unit.suffix
		}
	}
	return formatExactNarraForkTokens(value)
}

func appendNarraForkQuotaDetailLines(lines []string, balance string, requestQuota int, info *relaycommon.RelayInfo, details narraForkQuotaDetails) []string {
	config := info.NarraForkQuotaEvent
	if config.ShowBalance {
		lines = append(lines, fmt.Sprintf("余额: %s", balance))
	}
	if config.ShowRequestQuota {
		lines = append(lines, fmt.Sprintf("本次消耗: %s", logger.FormatQuota(requestQuota)))
	}

	mode := config.TokenDisplayMode
	lines = appendNarraForkUsageSummaryLine(
		lines,
		"今日消耗",
		"今日 Token",
		config.ShowTodayQuota,
		config.ShowTodayTokens,
		details.SummaryAvailable,
		config.ShowUnavailableFields,
		details.UsageSummary.TodayQuota,
		details.UsageSummary.TodayTokens,
		mode,
	)
	lines = appendNarraForkUsageSummaryLine(
		lines,
		"本月消耗",
		"本月 Token",
		config.ShowMonthQuota,
		config.ShowMonthTokens,
		details.SummaryAvailable,
		config.ShowUnavailableFields,
		details.UsageSummary.MonthQuota,
		details.UsageSummary.MonthTokens,
		mode,
	)

	if config.ShowTotalQuota {
		if details.TotalAvailable {
			lines = append(lines, fmt.Sprintf("总额度: %s", formatNarraForkQuota(details.TotalQuota)))
		} else if config.ShowUnavailableFields {
			lines = append(lines, "总额度: 未提供")
		}
	}
	if config.ShowUsedQuota {
		if details.TotalAvailable {
			lines = append(lines, fmt.Sprintf("累计消耗: %s", formatNarraForkQuota(details.UsedQuota)))
		} else if config.ShowUnavailableFields {
			lines = append(lines, "累计消耗: 未提供")
		}
	}

	tokenStatsAvailable := details.TokenStats.HasData()
	if config.ShowInputTokens {
		lines = appendNarraForkTokenDetailLine(lines, "输入 Token", details.TokenStats.InputTokens, tokenStatsAvailable, config.ShowUnavailableFields, mode)
	}
	if config.ShowOutputTokens {
		lines = appendNarraForkTokenDetailLine(lines, "输出 Token", details.TokenStats.OutputTokens, tokenStatsAvailable, config.ShowUnavailableFields, mode)
	}
	if config.ShowTotalTokens {
		lines = appendNarraForkTokenDetailLine(lines, "总 Token", details.TokenStats.TotalTokens, tokenStatsAvailable, config.ShowUnavailableFields, mode)
	}
	if config.ShowCacheHitTokens {
		lines = appendNarraForkTokenDetailLine(lines, "缓存命中", details.TokenStats.CacheHitTokens, tokenStatsAvailable, config.ShowUnavailableFields, mode)
	}
	if config.ShowCacheHitRate {
		label := "缓存命中率"
		if details.CacheHitRatePeriod != "当次请求" {
			label = fmt.Sprintf("缓存命中率（%s）", details.CacheHitRatePeriod)
		}
		if details.CacheHitRateAvailable {
			lines = append(lines, fmt.Sprintf("%s: %.2f%%", label, details.CacheHitRate))
		} else if config.ShowUnavailableFields {
			lines = append(lines, fmt.Sprintf("%s: 未提供", label))
		}
	}
	if config.ShowReasoningTokens {
		if details.TokenStats.ReasoningTokens > 0 {
			lines = append(lines, fmt.Sprintf("推理 Token: %s", formatNarraForkTokens(details.TokenStats.ReasoningTokens, mode)))
		} else if config.ShowUnavailableFields {
			lines = append(lines, "推理 Token: 未提供")
		}
	}

	modelName := info.OriginModelName
	if info.ChannelMeta != nil && info.ChannelMeta.UpstreamModelName != "" {
		modelName = info.ChannelMeta.UpstreamModelName
	}
	if config.ShowModel {
		if modelName != "" {
			lines = append(lines, fmt.Sprintf("模型: %s", modelName))
		} else if config.ShowUnavailableFields {
			lines = append(lines, "模型: 未提供")
		}
	}
	if config.ShowBillingSource {
		if billingSource := formatNarraForkBillingSource(info.BillingSource); billingSource != "" {
			lines = append(lines, fmt.Sprintf("计费来源: %s", billingSource))
		} else if config.ShowUnavailableFields {
			lines = append(lines, "计费来源: 未提供")
		}
	}
	if config.ShowLatency {
		lines = append(lines, fmt.Sprintf("请求耗时: %d ms", details.Metrics.LatencyMs))
	}
	if config.ShowTTFT {
		if details.Metrics.TTFTMs != nil {
			lines = append(lines, fmt.Sprintf("首 Token 延迟: %d ms", *details.Metrics.TTFTMs))
		} else if config.ShowUnavailableFields {
			lines = append(lines, "首 Token 延迟: 未提供")
		}
	}
	if config.ShowRequestID {
		if details.Metrics.RequestID != "" {
			lines = append(lines, fmt.Sprintf("请求 ID: %s", details.Metrics.RequestID))
		} else if config.ShowUnavailableFields {
			lines = append(lines, "请求 ID: 未提供")
		}
	}
	if config.ShowRetryCount {
		lines = append(lines, fmt.Sprintf("重试次数: %d", details.Metrics.RetryCount))
	}
	return lines
}

func appendNarraForkUsageSummaryLine(lines []string, quotaLabel string, tokenLabel string, showQuota bool, showTokens bool, available bool, showUnavailable bool, quota int64, tokens int64, mode string) []string {
	if !showQuota && !showTokens {
		return lines
	}
	if !available && !showUnavailable {
		return lines
	}

	quotaText := "未提供"
	tokensText := "未提供"
	if available {
		quotaText = logger.FormatQuota(int(quota))
		tokensText = formatNarraForkTokens(tokens, mode)
	}
	if showQuota && showTokens {
		return append(lines, fmt.Sprintf("%s: %s（%s tokens）", quotaLabel, quotaText, tokensText))
	}
	if showQuota {
		return append(lines, fmt.Sprintf("%s: %s", quotaLabel, quotaText))
	}
	return append(lines, fmt.Sprintf("%s: %s", tokenLabel, tokensText))
}

func appendNarraForkTokenDetailLine(lines []string, label string, value int64, available bool, showUnavailable bool, mode string) []string {
	if available {
		return append(lines, fmt.Sprintf("%s: %s", label, formatNarraForkTokens(value, mode)))
	}
	if showUnavailable {
		return append(lines, fmt.Sprintf("%s: 未提供", label))
	}
	return lines
}

func buildNarraForkExtra(config relaycommon.NarraForkQuotaEventConfig, info *relaycommon.RelayInfo, requestQuota int, details narraForkQuotaDetails, estimated bool) map[string]interface{} {
	extra := map[string]interface{}{
		"source":                config.BalanceSource,
		"billingSource":         info.BillingSource,
		"estimated":             estimated,
		"requestQuota":          requestQuota,
		"summaryAvailable":      details.SummaryAvailable,
		"tokenDisplayMode":      narrafork_setting.NormalizeTokenDisplayMode(config.TokenDisplayMode),
		"inputTokens":           details.TokenStats.InputTokens,
		"outputTokens":          details.TokenStats.OutputTokens,
		"totalTokens":           details.TokenStats.TotalTokens,
		"cacheHitTokens":        details.TokenStats.CacheHitTokens,
		"cacheHitRate":          math.Round(details.CacheHitRate*100) / 100,
		"cacheHitRateScope":     narrafork_setting.NormalizeCacheHitRateScope(config.CacheHitRateScope),
		"cacheHitRateDays":      narrafork_setting.NormalizeCacheHitRateDays(config.CacheHitRateDays),
		"cacheHitRateAvailable": details.CacheHitRateAvailable,
		"reasoningTokens":       details.TokenStats.ReasoningTokens,
	}
	if details.SummaryAvailable {
		extra["todayQuota"] = details.UsageSummary.TodayQuota
		extra["todayTokens"] = details.UsageSummary.TodayTokens
		extra["monthQuota"] = details.UsageSummary.MonthQuota
		extra["monthTokens"] = details.UsageSummary.MonthTokens
	}
	if details.TotalAvailable {
		extra["totalQuota"] = details.TotalQuota
		extra["usedQuota"] = details.UsedQuota
	}
	if info.TokenUnlimited {
		extra["tokenUnlimited"] = true
	}
	return extra
}

func formatNarraForkBillingSource(source string) string {
	switch source {
	case BillingSourceWallet:
		return "用户额度"
	case BillingSourceSubscription:
		return "订阅额度"
	default:
		return ""
	}
}

func maxInt64(left, right int64) int64 {
	if left > right {
		return left
	}
	return right
}
