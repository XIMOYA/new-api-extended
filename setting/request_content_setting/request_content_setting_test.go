// setting/request_content_setting/request_content_setting_test.go
// 验证请求内容审计配置的边界校验和默认归一化行为。
package request_content_setting

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateOptionRejectsUnsafeOrOutOfRangeValues(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		value string
	}{
		{name: "retention too short", key: OptionPrefix + "retention_days", value: "0"},
		{name: "record too large", key: OptionPrefix + "max_record_bytes", value: "1"},
		{name: "duplicate user", key: OptionPrefix + "admin_allowlist", value: "[1,1]"},
		{name: "path escapes root", key: OptionPrefix + "storage_path", value: "../outside"},
		{name: "chunk too small", key: OptionPrefix + "chunk_size_bytes", value: "1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Error(t, ValidateOption(test.key, test.value))
		})
	}
}

func TestNormalizeSettingsUsesSafeDefaults(t *testing.T) {
	require.Equal(t, DefaultRetentionDays, NormalizeRetentionDays(0))
	require.Equal(t, DefaultRetentionDays, NormalizeRetentionDays(MaxRetentionDays+1))
	require.Equal(t, "request-content-audit", NormalizeStoragePath("."))
	require.Equal(t, int64(DefaultMaxRecordMB)<<20, NormalizeMaxBytes(1, int64(DefaultMaxRecordMB)<<20, int64(MinMaxRecordMB)<<20, int64(MaxMaxRecordMB)<<20))
	require.Equal(t, DefaultChunkSize, NormalizeChunkSize(1))
}
