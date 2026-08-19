// setting/operation_setting/channel_affinity_setting_test.go
// 锁定内置亲和规则的默认值：默认不能再吞掉失败重试，
// 否则静默故障切换在 Claude Code / Codex 这类会话上会整体失效。
package operation_setting

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuiltinAffinityRulesAllowRetryByDefault(t *testing.T) {
	setting := GetChannelAffinitySetting()
	require.NotNil(t, setting)
	require.NotEmpty(t, setting.Rules)

	for _, rule := range setting.Rules {
		assert.Falsef(t, rule.SkipRetryOnFailure,
			"内置规则 %q 默认应允许失败后换渠道，需要锁定渠道时由管理员在界面上单独打开", rule.Name)
	}
}

func TestBuiltinAffinityRulesKeepTraceCoverage(t *testing.T) {
	setting := GetChannelAffinitySetting()
	require.NotNil(t, setting)

	names := make(map[string]bool, len(setting.Rules))
	for _, rule := range setting.Rules {
		names[rule.Name] = true
	}
	assert.True(t, names["claude cli trace"], "claude cli trace 规则不应被删除")
	assert.True(t, names["codex cli trace"], "codex cli trace 规则不应被删除")
}
