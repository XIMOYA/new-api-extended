// model/narrafork_policy_test.go
// 验证 NarraFork 用户组/用户策略表的 CRUD、JSON 编码和作用域校验。
package model

import (
	"testing"

	"github.com/QuantumNous/new-api/setting/narrafork_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestNarraForkQuotaEventPolicyCRUD(t *testing.T) {
	oldDB := DB
	testDB, err := gorm.Open(sqlite.Open("file:narrafork_policy_test?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	DB = testDB
	require.NoError(t, testDB.AutoMigrate(&NarraForkQuotaEventPolicy{}))
	t.Cleanup(func() {
		DB = oldDB
		db, closeErr := testDB.DB()
		if closeErr == nil {
			_ = db.Close()
		}
	})

	showTokens := false
	patch := narrafork_setting.NarraForkPolicyPatch{
		ShowTodayTokens: &showTokens,
		DetailTemplate:  narraforkStringPointer("Balance {{balance}}"),
	}
	require.NoError(t, UpsertNarraForkQuotaEventPolicy(NarraForkPolicyScopeGroup, "vip", patch))

	loaded, err := GetNarraForkQuotaEventPolicy(NarraForkPolicyScopeGroup, "vip")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.NotNil(t, loaded.ShowTodayTokens)
	require.False(t, *loaded.ShowTodayTokens)

	policies, err := GetNarraForkQuotaEventPolicies(NarraForkPolicyScopeGroup, []string{"vip"})
	require.NoError(t, err)
	require.Contains(t, policies, "vip")

	require.NoError(t, DeleteNarraForkQuotaEventPolicy(NarraForkPolicyScopeGroup, "vip"))
	loaded, err = GetNarraForkQuotaEventPolicy(NarraForkPolicyScopeGroup, "vip")
	require.NoError(t, err)
	require.Nil(t, loaded)
}

func TestDecodeNarraForkPolicyPatchIgnoresLegacyProtocolVersion(t *testing.T) {
	patch, err := DecodeNarraForkPolicyPatch(`{"protocol_version":"v2","show_latency":true}`)
	require.NoError(t, err)
	require.NotNil(t, patch)
	require.NotNil(t, patch.ShowLatency)
	require.True(t, *patch.ShowLatency)
}

func narraforkStringPointer(value string) *string {
	return &value
}
