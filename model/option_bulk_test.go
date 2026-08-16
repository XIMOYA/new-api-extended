// model/option_bulk_test.go
// 测试：系统配置批量写入的校验与事务行为。
package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
	"github.com/stretchr/testify/require"
)

func TestUpdateOptionsBulkPersistsNarraForkOptionsAtomically(t *testing.T) {
	if DB == nil {
		t.Fatal("model DB is not initialized")
	}
	require.NoError(t, DB.AutoMigrate(&Option{}))
	require.NoError(t, DB.Exec("DELETE FROM options").Error)

	previousOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.OptionMap = previousOptionMap
		DB.Exec("DELETE FROM options")
	})

	validKey := narrafork_setting.OptionPrefix + "enabled"
	invalidKey := narrafork_setting.OptionPrefix + "unknown"
	err := UpdateOptionsBulk(map[string]string{
		validKey:   "true",
		invalidKey: "true",
	})
	require.Error(t, err)

	var option Option
	require.Error(t, DB.Where("key = ?", validKey).First(&option).Error)

	require.NoError(t, UpdateOptionsBulk(map[string]string{
		validKey: "true",
	}))
	require.NoError(t, DB.Where("key = ?", validKey).First(&option).Error)
	require.Equal(t, "true", option.Value)
	require.Equal(t, "true", common.OptionMap[validKey])
}
