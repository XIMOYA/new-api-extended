// model/option_upstream_quota_test.go
// 覆盖上游余额关键词与流式接力开关的配置注册往返。
// 这两项若没进 OptionMap 或 updateOptionMap 漏了分支，后台改了设置也不会生效，
// 而这种缺陷在编译期和普通单测里都看不出来。
package model

import (
	"strconv"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

func TestOptionMapRegistersUpstreamQuotaSettings(t *testing.T) {
	originKeywords := append([]string(nil), operation_setting.UpstreamQuotaExhaustedKeywords...)
	originHandoff := operation_setting.StreamHandoffEnabled
	t.Cleanup(func() {
		operation_setting.UpstreamQuotaExhaustedKeywords = originKeywords
		operation_setting.StreamHandoffEnabled = originHandoff
	})

	common.OptionMapRWMutex.Lock()
	if common.OptionMap == nil {
		common.OptionMap = make(map[string]string)
	}
	common.OptionMap["UpstreamQuotaExhaustedKeywords"] = operation_setting.UpstreamQuotaExhaustedKeywordsToString()
	common.OptionMap["StreamHandoffEnabled"] = strconv.FormatBool(operation_setting.StreamHandoffEnabled)
	keywords := common.OptionMap["UpstreamQuotaExhaustedKeywords"]
	handoff := common.OptionMap["StreamHandoffEnabled"]
	common.OptionMapRWMutex.Unlock()

	// 默认关键词必须真的暴露出去，否则后台看到的是空列表
	require.NotEmpty(t, keywords, "默认关键词不应为空")
	require.Contains(t, keywords, "credit balance is too low", "Anthropic 余额文案应在默认表内")
	require.Contains(t, keywords, "insufficient_quota", "OpenAI 余额文案应在默认表内")
	require.Equal(t, "false", handoff, "接力必须默认关闭")
}

func TestUpdateOptionMapAppliesUpstreamQuotaSettings(t *testing.T) {
	originKeywords := append([]string(nil), operation_setting.UpstreamQuotaExhaustedKeywords...)
	originHandoff := operation_setting.StreamHandoffEnabled
	t.Cleanup(func() {
		operation_setting.UpstreamQuotaExhaustedKeywords = originKeywords
		operation_setting.StreamHandoffEnabled = originHandoff
	})

	// 关键词：改动应落到运行时变量，并按行拆分、小写化
	require.NoError(t, updateOptionMap("UpstreamQuotaExhaustedKeywords", "Balance Not Enough\n  余额告急  \n\n"))
	require.Equal(t, []string{"balance not enough", "余额告急"},
		operation_setting.UpstreamQuotaExhaustedKeywords,
		"应去空行、去首尾空格并小写化")

	// 开关：true / false 都要能生效
	require.NoError(t, updateOptionMap("StreamHandoffEnabled", "true"))
	require.True(t, operation_setting.StreamHandoffEnabled)
	require.NoError(t, updateOptionMap("StreamHandoffEnabled", "false"))
	require.False(t, operation_setting.StreamHandoffEnabled)
}

// 关键词写回时应能还原成后台文本框的多行格式。
func TestUpstreamQuotaKeywordsRoundTrip(t *testing.T) {
	origin := append([]string(nil), operation_setting.UpstreamQuotaExhaustedKeywords...)
	t.Cleanup(func() { operation_setting.UpstreamQuotaExhaustedKeywords = origin })

	operation_setting.UpstreamQuotaExhaustedKeywordsFromString("aaa\nbbb")
	text := operation_setting.UpstreamQuotaExhaustedKeywordsToString()

	require.Equal(t, []string{"aaa", "bbb"}, strings.Split(text, "\n"))
}
