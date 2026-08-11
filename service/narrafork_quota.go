// service/narrafork_quota.go
// 计算 NarraFork 额度事件所需的结算后余额快照，不执行实际扣费。
package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
	"github.com/gin-gonic/gin"
)

type NarraForkQuotaBalance struct {
	QuotaBalance         string
	DetailedQuotaBalance string
	Extra                map[string]interface{}
	Metrics              NarraForkEventMetrics
}

func BuildNarraForkQuotaBalance(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage) (NarraForkQuotaBalance, error) {
	if info == nil {
		return NarraForkQuotaBalance{}, fmt.Errorf("relay info is nil")
	}

	requestQuota := CalculateTextConsumeQuota(c, info, usage)
	if requestQuota < 0 {
		requestQuota = 0
	}
	metrics := BuildNarraForkEventMetrics(info, time.Now())
	config := info.NarraForkQuotaEvent
	balance := ""
	estimated := true

	switch config.BalanceSource {
	case narrafork_setting.BalanceSourceCustom:
		balance = strings.TrimSpace(config.CustomQuotaBalance)
		estimated = false
	case narrafork_setting.BalanceSourceUserQuota:
		quota, err := narraForkUserQuota(c, info, requestQuota)
		if err != nil {
			return NarraForkQuotaBalance{}, err
		}
		balance = formatNarraForkQuota(quota)
	case narrafork_setting.BalanceSourceTokenQuota:
		balance = narraForkTokenQuota(c, info, requestQuota)
	default:
		balance = narraForkEffectiveBalance(info, requestQuota)
	}

	if balance == "" {
		return NarraForkQuotaBalance{}, fmt.Errorf("NarraFork custom quota balance is empty")
	}

	result := NarraForkQuotaBalance{
		QuotaBalance: balance,
		Metrics:      metrics,
	}
	hasCustomDetailedBalance := config.BalanceSource == narrafork_setting.BalanceSourceCustom && strings.TrimSpace(config.CustomDetailedQuotaBalance) != ""
	shouldBuildDetails := config.ExposeExtra || (config.IncludeDetailed && !hasCustomDetailedBalance)
	details := narraForkQuotaDetails{}
	if shouldBuildDetails {
		details = buildNarraForkQuotaDetails(info, usage, requestQuota)
	}
	details.Metrics = metrics
	if config.IncludeDetailed {
		customDetailed := strings.TrimSpace(config.CustomDetailedQuotaBalance)
		if config.BalanceSource == narrafork_setting.BalanceSourceCustom && customDetailed != "" {
			result.DetailedQuotaBalance = customDetailed
		} else if strings.TrimSpace(config.DetailTemplate) != "" {
			values := buildNarraForkDetailTemplateValues(balance, requestQuota, info, details)
			rendered, renderErr := RenderNarraForkDetailTemplate(config.DetailTemplate, values)
			if renderErr != nil {
				common.SysLog("skip NarraFork detail template: " + renderErr.Error())
				lines := appendNarraForkQuotaDetailLines(nil, balance, requestQuota, info, details)
				result.DetailedQuotaBalance = strings.Join(lines, "\n")
			} else {
				result.DetailedQuotaBalance = rendered
			}
		} else {
			lines := appendNarraForkQuotaDetailLines(nil, balance, requestQuota, info, details)
			result.DetailedQuotaBalance = strings.Join(lines, "\n")
		}
	}
	if config.ExposeExtra {
		result.Extra = buildNarraForkExtra(config, info, requestQuota, details, estimated)
	}
	return result, nil
}

func narraForkEffectiveBalance(info *relaycommon.RelayInfo, requestQuota int) string {
	if info.BillingSource == BillingSourceSubscription {
		if info.SubscriptionAmountTotal <= 0 {
			return "unlimited"
		}
		return formatNarraForkQuota(narraForkSubscriptionQuota(info, requestQuota))
	}
	remaining := int64(info.UserQuota) - int64(requestQuota)
	if remaining < 0 {
		remaining = 0
	}
	return formatNarraForkQuota(remaining)
}

func narraForkUserQuota(c *gin.Context, info *relaycommon.RelayInfo, requestQuota int) (int64, error) {
	if info.BillingSource != BillingSourceSubscription || info.UserId <= 0 {
		remaining := int64(info.UserQuota) - int64(requestQuota)
		if remaining < 0 {
			remaining = 0
		}
		return remaining, nil
	}
	quota, err := model.GetUserQuota(info.UserId, false)
	return int64(quota), err
}

func narraForkSubscriptionQuota(info *relaycommon.RelayInfo, requestQuota int) int64 {
	if info.SubscriptionAmountTotal <= 0 {
		return 0
	}
	netDelta := int64(requestQuota - info.FinalPreConsumedQuota)
	remaining := info.SubscriptionAmountTotal - info.SubscriptionAmountUsedAfterPreConsume - netDelta
	if remaining < 0 {
		return 0
	}
	return remaining
}

func narraForkTokenQuota(c *gin.Context, info *relaycommon.RelayInfo, requestQuota int) string {
	if info.TokenUnlimited {
		return "unlimited"
	}
	initialQuota := 0
	if c != nil {
		initialQuota = c.GetInt("token_quota")
	}
	remaining := initialQuota - requestQuota
	if remaining < 0 {
		remaining = 0
	}
	return formatNarraForkQuota(int64(remaining))
}

func formatNarraForkQuota(quota int64) string {
	return logger.FormatQuota(common.QuotaFromFloat(float64(quota)))
}
