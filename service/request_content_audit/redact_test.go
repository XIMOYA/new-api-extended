// service/request_content_audit/redact_test.go
// 验证非 Root 管理员的正文脱敏规则不会原样返回凭据或联系方式。
package request_content_audit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRedactValueLimitedStopsDeepAndWidePayloads(t *testing.T) {
	value := map[string]any{
		"authorization": "Bearer very-secret-token",
		"nested":        map[string]any{"password": "plain-password"},
		"items":         []any{"one", "two", "three"},
	}

	redacted, ok := RedactValueLimited(value, 1, 20).(map[string]any)
	require.True(t, ok)
	require.Equal(t, "[REDACTED]", redacted["authorization"])
	require.Equal(t, "[REDACTED_NESTED]", redacted["nested"].(map[string]any)["password"])
}

func TestRedactValueMasksSensitiveFieldsAndInlineSecrets(t *testing.T) {
	value := map[string]any{
		"authorization": "Bearer very-secret-token",
		"file_name":     "customer-records.csv",
		"message":       "联系 admin@example.com 或 415-555-0123，token=sk-test-secret",
		"nested": []any{
			map[string]any{"password": "plain-password"},
		},
	}

	redacted := RedactValue(value).(map[string]any)
	require.Equal(t, "[REDACTED]", redacted["authorization"])
	require.Equal(t, "[REDACTED]", redacted["file_name"])
	require.NotContains(t, redacted["message"], "admin@example.com")
	require.NotContains(t, redacted["message"], "415-555-0123")
	require.NotContains(t, redacted["message"], "sk-test-secret")
	require.Equal(t, "[REDACTED]", redacted["nested"].([]any)[0].(map[string]any)["password"])
}
