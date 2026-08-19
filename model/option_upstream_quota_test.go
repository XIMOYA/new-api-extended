// model/option_upstream_quota_test.go
// 覆盖流式接力相关配置项的注册往返。
//
// 一个设置若漏进 OptionMap、或漏了 updateOptionMap 的 case，编译与既有测试都不会报错，
// 但管理员在后台改的值会被静默忽略。这里把两个方向都钉住。
//
// 额度耗尽的关键词判定已收敛到 operation_setting.AutomaticDisableKeywords
// （与渠道自动禁用同一套），因此不再单独测一份关键词配置。
package model

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestOptionMapRegistersStreamHandoffSettings(t *testing.T) {
	originEnabled := operation_setting.StreamHandoffEnabled
	originAttempts := operation_setting.StreamHandoffMaxAttempts
	t.Cleanup(func() {
		operation_setting.StreamHandoffEnabled = originEnabled
		operation_setting.StreamHandoffMaxAttempts = originAttempts
	})

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap["StreamHandoffEnabled"] = strconv.FormatBool(operation_setting.StreamHandoffEnabled)
	common.OptionMap["StreamHandoffMaxAttempts"] = strconv.Itoa(operation_setting.StreamHandoffMaxAttempts)
	enabled := common.OptionMap["StreamHandoffEnabled"]
	attempts := common.OptionMap["StreamHandoffMaxAttempts"]
	common.OptionMapRWMutex.Unlock()

	// 接力会重复计费已投递内容并可能让回答在断点处跳变，必须默认关闭。
	require.Equal(t, "false", enabled, "流式接力必须默认关闭")
	require.Equal(t, "2", attempts, "默认接力上限应为 2")
}

func TestUpdateOptionMapAppliesStreamHandoffSettings(t *testing.T) {
	originEnabled := operation_setting.StreamHandoffEnabled
	originAttempts := operation_setting.StreamHandoffMaxAttempts
	t.Cleanup(func() {
		operation_setting.StreamHandoffEnabled = originEnabled
		operation_setting.StreamHandoffMaxAttempts = originAttempts
	})

	require.NoError(t, updateOptionMap("StreamHandoffEnabled", "true"))
	require.True(t, operation_setting.StreamHandoffEnabled)
	require.NoError(t, updateOptionMap("StreamHandoffEnabled", "false"))
	require.False(t, operation_setting.StreamHandoffEnabled)

	require.NoError(t, updateOptionMap("StreamHandoffMaxAttempts", "5"))
	require.Equal(t, 5, operation_setting.StreamHandoffMaxAttempts)
}

// 额度耗尽关键词并入了自动禁用关键词表，改动必须同时影响两处语义：
// 命中既会换渠道，也会让渠道进入自动禁用判定。
func TestAutomaticDisableKeywordsCoverUpstreamQuotaWording(t *testing.T) {
	origin := append([]string(nil), operation_setting.AutomaticDisableKeywords...)
	t.Cleanup(func() { operation_setting.AutomaticDisableKeywords = origin })

	joined := operation_setting.AutomaticDisableKeywordsToString()
	for _, kw := range []string{
		"Your credit balance is too low", // Anthropic
		"insufficient_quota",             // OpenAI
		"余额不足",                           // 国内厂商
		"欠费",
	} {
		require.Contains(t, joined, kw, "默认关键词表应覆盖 %q", kw)
	}

	// 后台改动应落到运行时变量，并按行拆分、小写化
	operation_setting.AutomaticDisableKeywordsFromString("Balance Not Enough\n  额度告急  \n\n")
	require.Equal(t, []string{"balance not enough", "额度告急"},
		operation_setting.AutomaticDisableKeywords,
		"应去空行、去首尾空格并小写化")
}
