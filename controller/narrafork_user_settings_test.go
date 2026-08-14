// controller/narrafork_user_settings_test.go
// 验证 NarraFork 用户设置的字段授权、全局上限和模板字段校验。
package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
	"github.com/stretchr/testify/require"
)

func TestValidateNarraForkUserSettingsRejectsUnauthorizedField(t *testing.T) {
	global := narrafork_setting.NarraForkSetting{
		Enabled:                  true,
		IncludeDetailed:          true,
		AllowUserDisplayOverride: true,
	}
	capabilities := narrafork_setting.BuildNarraForkUserOverrideCapabilities(global)
	value := false

	err := validateNarraForkUserSettings(&dto.NarraForkUserSettings{
		ShowBalance: &value,
	}, capabilities)

	require.Error(t, err)
}

func TestValidateNarraForkUserSettingsAcceptsAuthorizedField(t *testing.T) {
	global := narrafork_setting.NarraForkSetting{
		Enabled:                  true,
		IncludeDetailed:          true,
		ShowBalance:              true,
		AllowUserDisplayOverride: true,
		AllowUserShowBalance:     true,
	}
	capabilities := narrafork_setting.BuildNarraForkUserOverrideCapabilities(global)
	value := false

	err := validateNarraForkUserSettings(&dto.NarraForkUserSettings{
		ShowBalance: &value,
	}, capabilities)

	require.NoError(t, err)
}

func TestValidateNarraForkTemplateFieldsRejectsHiddenGlobalField(t *testing.T) {
	caps := map[string]bool{
		narrafork_setting.UserOverrideFieldShowBalance: false,
	}

	err := validateNarraForkTemplateFields("Balance: {{balance}}", caps)

	require.Error(t, err)
}
