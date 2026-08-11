// service/narrafork_event.go
// NarraFork 额度事件的请求指标计算和安全详情模板渲染。
package service

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
)

type NarraForkEventMetrics struct {
	LatencyMs  int64
	TTFTMs     *int64
	RequestID  string
	RetryCount int
}

func BuildNarraForkEventMetrics(info *relaycommon.RelayInfo, now time.Time) NarraForkEventMetrics {
	metrics := NarraForkEventMetrics{}
	if info == nil {
		return metrics
	}
	metrics.RequestID = info.RequestId
	metrics.RetryCount = info.RetryIndex
	if !info.StartTime.IsZero() {
		metrics.LatencyMs = now.Sub(info.StartTime).Milliseconds()
		if metrics.LatencyMs < 0 {
			metrics.LatencyMs = 0
		}
	}
	if info.HasSendResponse() && !info.FirstResponseTime.IsZero() && info.FirstResponseTime.After(info.StartTime) {
		ttft := info.FirstResponseTime.Sub(info.StartTime).Milliseconds()
		if ttft >= 0 {
			metrics.TTFTMs = &ttft
		}
	}
	return metrics
}

var narraForkTemplatePattern = regexp.MustCompile(`\{\{\s*([a-z_]+)\s*\}\}`)

func RenderNarraForkDetailTemplate(template string, values map[string]string) (string, error) {
	if err := narrafork_setting.ValidateDetailTemplate(template); err != nil {
		return "", err
	}
	var renderErr error
	result := narraForkTemplatePattern.ReplaceAllStringFunc(template, func(match string) string {
		if renderErr != nil {
			return match
		}
		parts := narraForkTemplatePattern.FindStringSubmatch(match)
		if len(parts) != 2 {
			renderErr = fmt.Errorf("invalid NarraFork template placeholder")
			return match
		}
		value, ok := values[strings.TrimSpace(parts[1])]
		if !ok {
			renderErr = fmt.Errorf("missing NarraFork template value: %s", parts[1])
			return match
		}
		return value
	})
	if renderErr != nil {
		return "", renderErr
	}
	return result, nil
}

func buildNarraForkDetailTemplateValues(balance string, requestQuota int, info *relaycommon.RelayInfo, details narraForkQuotaDetails) map[string]string {
	values := map[string]string{
		"balance":               balance,
		"request_quota":         logger.FormatQuota(requestQuota),
		"today_quota":           logger.FormatQuota(int(details.UsageSummary.TodayQuota)),
		"today_tokens":          formatNarraForkTokens(details.UsageSummary.TodayTokens, info.NarraForkQuotaEvent.TokenDisplayMode),
		"month_quota":           logger.FormatQuota(int(details.UsageSummary.MonthQuota)),
		"month_tokens":          formatNarraForkTokens(details.UsageSummary.MonthTokens, info.NarraForkQuotaEvent.TokenDisplayMode),
		"total_quota":           "未提供",
		"used_quota":            "未提供",
		"input_tokens":          "未提供",
		"output_tokens":         "未提供",
		"total_tokens":          "未提供",
		"cache_hit_tokens":      "未提供",
		"cache_hit_rate":        "未提供",
		"cache_hit_rate_period": details.CacheHitRatePeriod,
		"reasoning_tokens":      "未提供",
		"model":                 "未提供",
		"billing_source":        "未提供",
		"latency_ms":            strconv.FormatInt(details.Metrics.LatencyMs, 10),
		"ttft_ms":               "未提供",
		"request_id":            details.Metrics.RequestID,
		"retry_count":           strconv.Itoa(details.Metrics.RetryCount),
	}
	if details.SummaryAvailable {
		values["today_quota"] = logger.FormatQuota(int(details.UsageSummary.TodayQuota))
		values["today_tokens"] = formatNarraForkTokens(details.UsageSummary.TodayTokens, info.NarraForkQuotaEvent.TokenDisplayMode)
		values["month_quota"] = logger.FormatQuota(int(details.UsageSummary.MonthQuota))
		values["month_tokens"] = formatNarraForkTokens(details.UsageSummary.MonthTokens, info.NarraForkQuotaEvent.TokenDisplayMode)
	}
	if details.TotalAvailable {
		values["total_quota"] = formatNarraForkQuota(details.TotalQuota)
		values["used_quota"] = formatNarraForkQuota(details.UsedQuota)
	}
	if details.TokenStats.HasData() {
		mode := info.NarraForkQuotaEvent.TokenDisplayMode
		values["input_tokens"] = formatNarraForkTokens(details.TokenStats.InputTokens, mode)
		values["output_tokens"] = formatNarraForkTokens(details.TokenStats.OutputTokens, mode)
		values["total_tokens"] = formatNarraForkTokens(details.TokenStats.TotalTokens, mode)
		values["cache_hit_tokens"] = formatNarraForkTokens(details.TokenStats.CacheHitTokens, mode)
		if details.CacheHitRateAvailable {
			values["cache_hit_rate"] = fmt.Sprintf("%.2f%%", details.CacheHitRate)
		}
		if details.TokenStats.ReasoningTokens > 0 {
			values["reasoning_tokens"] = formatNarraForkTokens(details.TokenStats.ReasoningTokens, mode)
		}
	}
	if info != nil {
		values["model"] = info.OriginModelName
		if info.ChannelMeta != nil && info.ChannelMeta.UpstreamModelName != "" {
			values["model"] = info.ChannelMeta.UpstreamModelName
		}
		if billingSource := formatNarraForkBillingSource(info.BillingSource); billingSource != "" {
			values["billing_source"] = billingSource
		}
	}
	if details.Metrics.TTFTMs != nil {
		values["ttft_ms"] = strconv.FormatInt(*details.Metrics.TTFTMs, 10)
	}
	return values
}
