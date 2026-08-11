// service/narrafork_quota_test.go
// 验证 NarraFork 钱包、订阅、Token、无限 Token 与 custom 额度投影。
package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	relaytypes "github.com/QuantumNous/new-api/relaykit/types"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newNarraForkQuotaTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Set("token_name", "test-token")
	c.Set("token_quota", 1000)
	return c
}

func newNarraForkQuotaTestInfo() *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: "gpt-test",
		StartTime:       time.Now(),
		UserQuota:       1000,
		BillingSource:   BillingSourceWallet,
		RelayFormat:     relaytypes.RelayFormatOpenAI,
		ChannelMeta:     &relaycommon.ChannelMeta{},
		PriceData: hosttypes.PriceData{
			ModelRatio:      1,
			CompletionRatio: 1,
			CacheRatio:      1,
			ImageRatio:      1,
			AudioRatio:      1,
			GroupRatioInfo:  hosttypes.GroupRatioInfo{GroupRatio: 1},
		},
		NarraForkQuotaEvent: relaycommon.NarraForkQuotaEventConfig{
			Activated:             true,
			BalanceSource:         "effective",
			IncludeDetailed:       true,
			ShowBalance:           true,
			ShowRequestQuota:      true,
			ShowTodayQuota:        true,
			ShowTodayTokens:       true,
			ShowMonthQuota:        true,
			ShowMonthTokens:       true,
			ShowTotalQuota:        true,
			ShowUsedQuota:         true,
			ShowInputTokens:       true,
			ShowOutputTokens:      true,
			ShowTotalTokens:       true,
			ShowCacheHitTokens:    true,
			ShowCacheHitRate:      true,
			ShowReasoningTokens:   true,
			ShowModel:             true,
			ShowBillingSource:     true,
			ShowUnavailableFields: false,
		},
	}
}

func TestBuildNarraForkQuotaBalanceWalletEffective(t *testing.T) {
	c := newNarraForkQuotaTestContext(t)
	info := newNarraForkQuotaTestInfo()
	usage := &dto.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}

	requestQuota := CalculateTextConsumeQuota(c, info, usage)
	require.Positive(t, requestQuota)
	balance, err := BuildNarraForkQuotaBalance(c, info, usage)
	require.NoError(t, err)
	require.Equal(t, formatNarraForkQuota(int64(info.UserQuota-requestQuota)), balance.QuotaBalance)
	require.Contains(t, balance.DetailedQuotaBalance, "本次消耗:")
}

func TestBuildNarraForkQuotaBalanceSubscriptionEffective(t *testing.T) {
	c := newNarraForkQuotaTestContext(t)
	info := newNarraForkQuotaTestInfo()
	info.BillingSource = BillingSourceSubscription
	info.SubscriptionAmountTotal = 1000
	info.SubscriptionAmountUsedAfterPreConsume = 100
	info.FinalPreConsumedQuota = 20
	usage := &dto.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}

	requestQuota := CalculateTextConsumeQuota(c, info, usage)
	balance, err := BuildNarraForkQuotaBalance(c, info, usage)
	require.NoError(t, err)
	require.Equal(t, formatNarraForkQuota(narraForkSubscriptionQuota(info, requestQuota)), balance.QuotaBalance)
}

func TestBuildNarraForkQuotaBalanceUserAndTokenQuota(t *testing.T) {
	c := newNarraForkQuotaTestContext(t)
	usage := &dto.Usage{PromptTokens: 2, CompletionTokens: 3, TotalTokens: 5}

	userInfo := newNarraForkQuotaTestInfo()
	userInfo.NarraForkQuotaEvent.BalanceSource = "user_quota"
	requestQuota := CalculateTextConsumeQuota(c, userInfo, usage)
	userBalance, err := BuildNarraForkQuotaBalance(c, userInfo, usage)
	require.NoError(t, err)
	require.Equal(t, formatNarraForkQuota(int64(userInfo.UserQuota-requestQuota)), userBalance.QuotaBalance)

	tokenInfo := newNarraForkQuotaTestInfo()
	tokenInfo.NarraForkQuotaEvent.BalanceSource = "token_quota"
	tokenBalance, err := BuildNarraForkQuotaBalance(c, tokenInfo, usage)
	require.NoError(t, err)
	require.Equal(t, formatNarraForkQuota(int64(c.GetInt("token_quota")-requestQuota)), tokenBalance.QuotaBalance)
}

func TestBuildNarraForkQuotaBalanceUnlimitedToken(t *testing.T) {
	c := newNarraForkQuotaTestContext(t)
	info := newNarraForkQuotaTestInfo()
	info.TokenUnlimited = true
	info.NarraForkQuotaEvent.BalanceSource = "token_quota"
	info.NarraForkQuotaEvent.ExposeExtra = true

	balance, err := BuildNarraForkQuotaBalance(c, info, &dto.Usage{PromptTokens: 2, CompletionTokens: 3})
	require.NoError(t, err)
	require.Equal(t, "unlimited", balance.QuotaBalance)
	require.Equal(t, true, balance.Extra["tokenUnlimited"])
}

func TestBuildNarraForkQuotaBalanceCustom(t *testing.T) {
	c := newNarraForkQuotaTestContext(t)
	info := newNarraForkQuotaTestInfo()
	info.NarraForkQuotaEvent = relaycommon.NarraForkQuotaEventConfig{
		Activated:                  true,
		BalanceSource:              "custom",
		IncludeDetailed:            true,
		CustomQuotaBalance:         "custom-balance",
		CustomDetailedQuotaBalance: "custom-detail",
	}

	balance, err := BuildNarraForkQuotaBalance(c, info, &dto.Usage{PromptTokens: 2, CompletionTokens: 3})
	require.NoError(t, err)
	require.Equal(t, "custom-balance", balance.QuotaBalance)
	require.Equal(t, "custom-detail", balance.DetailedQuotaBalance)
}

func TestNarraForkTokenStatisticsCalculatesCacheHitRate(t *testing.T) {
	stats := narraForkTokenStatistics(&dto.Usage{
		PromptTokens:     1000,
		CompletionTokens: 200,
		TotalTokens:      1200,
		InputTokens:      2000,
		OutputTokens:     300,
		PromptTokensDetails: dto.InputTokenDetails{
			CachedTokens: 250,
		},
		CompletionTokenDetails: dto.OutputTokenDetails{
			ReasoningTokens: 40,
		},
	})

	require.Equal(t, int64(1000), stats.InputTokens)
	require.Equal(t, int64(200), stats.OutputTokens)
	require.Equal(t, int64(1200), stats.TotalTokens)
	require.Equal(t, int64(250), stats.CacheHitTokens)
	require.InDelta(t, 25, stats.CacheHitRate, 0.0001)
	require.Equal(t, int64(40), stats.ReasoningTokens)
}

func TestFormatNarraForkTokensSupportsExactAndCompactModes(t *testing.T) {
	require.Equal(t, "78,586", formatNarraForkTokens(78586, "exact"))
	require.Equal(t, "78.59K", formatNarraForkTokens(78586, "compact"))
	require.Equal(t, "1.2M", formatNarraForkTokens(1200000, "compact"))
	require.Equal(t, "1B", formatNarraForkTokens(1000000000, "compact"))
}

func TestBuildNarraForkQuotaBalanceIncludesTokenDetails(t *testing.T) {
	c := newNarraForkQuotaTestContext(t)
	info := newNarraForkQuotaTestInfo()
	info.NarraForkQuotaEvent.TokenDisplayMode = "compact"
	usage := &dto.Usage{PromptTokens: 1000, CompletionTokens: 200, TotalTokens: 1200}

	balance, err := BuildNarraForkQuotaBalance(c, info, usage)
	require.NoError(t, err)
	require.Contains(t, balance.DetailedQuotaBalance, "输入 Token: 1K")
	require.Contains(t, balance.DetailedQuotaBalance, "输出 Token: 200")
	require.Contains(t, balance.DetailedQuotaBalance, "缓存命中率: 0.00%")
	require.True(t, strings.Contains(balance.DetailedQuotaBalance, "模型: gpt-test"))
}

func TestAppendNarraForkQuotaDetailLinesHonorsDisplayOptions(t *testing.T) {
	info := newNarraForkQuotaTestInfo()
	info.NarraForkQuotaEvent = relaycommon.NarraForkQuotaEventConfig{
		ShowTodayTokens:       true,
		ShowMonthQuota:        true,
		ShowUsedQuota:         true,
		ShowOutputTokens:      true,
		ShowCacheHitRate:      true,
		ShowUnavailableFields: false,
	}
	details := narraForkQuotaDetails{
		UsageSummary: model.NarraForkUsageSummary{
			Available:   true,
			TodayQuota:  100,
			TodayTokens: 200,
			MonthQuota:  300,
			MonthTokens: 400,
		},
		SummaryAvailable: true,
		TotalQuota:       500,
		UsedQuota:        100,
		TotalAvailable:   true,
		TokenStats: narraForkTokenStats{
			InputTokens:  10,
			OutputTokens: 20,
			TotalTokens:  30,
			CacheHitRate: 25,
		},
		CacheHitRate:          25,
		CacheHitRateAvailable: true,
		CacheHitRatePeriod:    "当次请求",
	}

	lines := appendNarraForkQuotaDetailLines(nil, "$1.00", 12, info, details)
	text := strings.Join(lines, "\n")
	require.NotContains(t, text, "余额:")
	require.NotContains(t, text, "本次消耗:")
	require.Contains(t, text, "今日 Token: 200")
	require.Contains(t, text, "本月消耗:")
	require.Contains(t, text, "累计消耗:")
	require.NotContains(t, text, "总额度:")
	require.Contains(t, text, "输出 Token: 20")
	require.NotContains(t, text, "输入 Token:")
	require.NotContains(t, text, "总 Token:")
	require.NotContains(t, text, "缓存命中:")
	require.Contains(t, text, "缓存命中率: 25.00%")
	require.NotContains(t, text, "模型:")
	require.NotContains(t, text, "计费来源:")
}

func TestAppendNarraForkQuotaDetailLinesShowsUnavailableReasoningWhenConfigured(t *testing.T) {
	info := newNarraForkQuotaTestInfo()
	info.NarraForkQuotaEvent = relaycommon.NarraForkQuotaEventConfig{
		ShowReasoningTokens:   true,
		ShowUnavailableFields: false,
	}
	details := narraForkQuotaDetails{
		TokenStats: narraForkTokenStats{InputTokens: 10, OutputTokens: 20, TotalTokens: 30},
	}

	withoutPlaceholder := strings.Join(appendNarraForkQuotaDetailLines(nil, "$1.00", 12, info, details), "\n")
	require.NotContains(t, withoutPlaceholder, "推理 Token:")

	info.NarraForkQuotaEvent.ShowUnavailableFields = true
	withPlaceholder := strings.Join(appendNarraForkQuotaDetailLines(nil, "$1.00", 12, info, details), "\n")
	require.Contains(t, withPlaceholder, "推理 Token: 未提供")
}
