// setting/narrafork_setting/policy_test.go
// 验证 NarraFork 作用域策略补丁和详情模板占位符校验。
package narrafork_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateDetailTemplate(t *testing.T) {
	require.NoError(t, ValidateDetailTemplate("Balance {{balance}} / Latency {{latency_ms}} ms"))
	require.NoError(t, ValidateDetailTemplate(""))
	require.Error(t, ValidateDetailTemplate("{{unknown}}"))
	require.Error(t, ValidateDetailTemplate("{{balance"))
	require.Error(t, ValidateDetailTemplate("balance}}"))
}

func TestNarraForkPolicyPatchValidate(t *testing.T) {
	showTokens := false
	patch := NarraForkPolicyPatch{
		ShowTodayTokens:  &showTokens,
		DetailTemplate:   stringPointer("{{today_tokens}} / {{request_id}}"),
		TokenDisplayMode: TokenDisplayModeCompact,
	}
	require.NoError(t, patch.Validate())
}

func stringPointer(value string) *string {
	return &value
}
