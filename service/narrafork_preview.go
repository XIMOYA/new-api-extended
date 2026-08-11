// service/narrafork_preview.go
// 生成 NarraFork 后台预览和一次性 SSE 测试事件，不访问真实用户额度或渠道。
package service

import (
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
)

type NarraForkQuotaEventPreview struct {
	EventName string                 `json:"eventName"`
	Payload   map[string]interface{} `json:"payload"`
	SSE       string                 `json:"sse"`
}

func applyNarraForkPreviewCacheHitRateWindow(details *narraForkQuotaDetails, config relaycommon.NarraForkQuotaEventConfig) {
	if details == nil {
		return
	}
	details.CacheHitRate = details.TokenStats.CacheHitRate
	details.CacheHitRateAvailable = details.TokenStats.CacheInputTokens > 0
	details.CacheHitRatePeriod = narraForkCacheHitRatePeriod(&config)
	if narrafork_setting.NormalizeCacheHitRateScope(config.CacheHitRateScope) == narrafork_setting.CacheHitRateScopeRequest {
		return
	}

	historicalInputTokens := int64(4000)
	historicalCacheHitTokens := int64(1600)
	if narrafork_setting.NormalizeCacheHitRateScope(config.CacheHitRateScope) == narrafork_setting.CacheHitRateScopeRecentDays {
		historicalInputTokens = 10000
		historicalCacheHitTokens = 5000
	}
	inputTokens := historicalInputTokens + details.TokenStats.CacheInputTokens
	cacheHitTokens := historicalCacheHitTokens + details.TokenStats.CacheHitTokens
	if inputTokens <= 0 {
		return
	}
	if cacheHitTokens > inputTokens {
		cacheHitTokens = inputTokens
	}
	details.CacheHitRate = float64(cacheHitTokens) / float64(inputTokens) * 100
	details.CacheHitRateAvailable = true
}

func BuildNarraForkQuotaEventPreview(config relaycommon.NarraForkQuotaEventConfig) (NarraForkQuotaEventPreview, error) {
	now := time.Now()
	info := &relaycommon.RelayInfo{
		OriginModelName:     "preview-model",
		RequestId:           "narrafork-preview-request",
		RetryIndex:          1,
		StartTime:           now.Add(-1234 * time.Millisecond),
		FirstResponseTime:   now.Add(-900 * time.Millisecond),
		BillingSource:       BillingSourceWallet,
		NarraForkQuotaEvent: config,
	}
	usage := &dto.Usage{
		PromptTokens:         1000,
		CompletionTokens:     250,
		TotalTokens:          1250,
		PromptCacheHitTokens: 120,
	}
	metrics := BuildNarraForkEventMetrics(info, now)
	details := narraForkQuotaDetails{
		TokenStats: narraForkTokenStatistics(usage),
		Metrics:    metrics,
		UsageSummary: model.NarraForkUsageSummary{
			Available:   true,
			TodayQuota:  12345,
			TodayTokens: 5000,
			MonthQuota:  67890,
			MonthTokens: 25000,
		},
		SummaryAvailable: true,
		TotalQuota:       100000,
		UsedQuota:        32110,
		TotalAvailable:   true,
	}
	applyNarraForkPreviewCacheHitRateWindow(&details, config)
	requestQuota := 1234
	balance := "$12.34"
	if config.BalanceSource == narrafork_setting.BalanceSourceCustom && strings.TrimSpace(config.CustomQuotaBalance) != "" {
		balance = config.CustomQuotaBalance
	}

	detailed := ""
	customDetailed := strings.TrimSpace(config.CustomDetailedQuotaBalance)
	if config.IncludeDetailed {
		if config.BalanceSource == narrafork_setting.BalanceSourceCustom && customDetailed != "" {
			detailed = customDetailed
		} else if strings.TrimSpace(config.DetailTemplate) != "" {
			values := buildNarraForkDetailTemplateValues(balance, requestQuota, info, details)
			rendered, err := RenderNarraForkDetailTemplate(config.DetailTemplate, values)
			if err != nil {
				return NarraForkQuotaEventPreview{}, err
			}
			detailed = rendered
		} else {
			lines := appendNarraForkQuotaDetailLines(nil, balance, requestQuota, info, details)
			detailed = strings.Join(lines, "\n")
		}
	}

	payload := map[string]interface{}{
		"quotaBalance": balance,
	}
	if detailed != "" {
		payload["detailedQuotaBalance"] = detailed
	}
	if config.ExposeExtra {
		payload["extra"] = buildNarraForkExtra(config, info, requestQuota, details, false)
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return NarraForkQuotaEventPreview{}, err
	}
	return NarraForkQuotaEventPreview{
		EventName: relaycommon.NarraForkQuotaEventName,
		Payload:   payload,
		SSE:       "event: " + relaycommon.NarraForkQuotaEventName + "\ndata: " + string(data) + "\n\n",
	}, nil
}
