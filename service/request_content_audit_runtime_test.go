// service/request_content_audit_runtime_test.go
// 验证请求内容审计始终要求显式配置可跨重启复用的稳定密钥。
package service

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRequestContentAuditEncryptionReadyRequiresStableSecret(t *testing.T) {
	previousCryptoSecret, hadCryptoSecret := os.LookupEnv("CRYPTO_SECRET")
	previousSessionSecret, hadSessionSecret := os.LookupEnv("SESSION_SECRET")
	t.Cleanup(func() {
		if hadCryptoSecret {
			_ = os.Setenv("CRYPTO_SECRET", previousCryptoSecret)
		} else {
			_ = os.Unsetenv("CRYPTO_SECRET")
		}
		if hadSessionSecret {
			_ = os.Setenv("SESSION_SECRET", previousSessionSecret)
		} else {
			_ = os.Unsetenv("SESSION_SECRET")
		}
	})

	_ = os.Unsetenv("CRYPTO_SECRET")
	_ = os.Unsetenv("SESSION_SECRET")
	require.False(t, RequestContentAuditEncryptionReady())

	require.NoError(t, os.Setenv("SESSION_SECRET", "stable-test-session-secret"))
	require.True(t, RequestContentAuditEncryptionReady())

	require.NoError(t, os.Setenv("CRYPTO_SECRET", "stable-test-crypto-secret"))
	require.True(t, RequestContentAuditEncryptionReady())
}
