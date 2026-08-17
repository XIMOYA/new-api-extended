// controller/request_content_audit_test.go
// 验证请求内容审计关闭且缺少稳定密钥时，历史元数据接口仍可正常返回。
package controller

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	auditstore "github.com/QuantumNous/new-api/service/request_content_audit"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestListRequestContentAuditsWithoutEncryptionSecretReturnsMetadataResponse(t *testing.T) {
	previousDB := model.DB
	previousLogDir := common.LogDir
	previousCryptoSecret, hadCryptoSecret := os.LookupEnv("CRYPTO_SECRET")
	previousSessionSecret, hadSessionSecret := os.LookupEnv("SESSION_SECRET")
	var testDB *gorm.DB
	t.Cleanup(func() {
		if testDB != nil {
			if sqlDB, err := testDB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		model.DB = previousDB
		common.LogDir = previousLogDir
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

	root := t.TempDir()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	testDB = db
	model.DB = db
	logDir := root
	common.LogDir = &logDir

	gin.SetMode(gin.TestMode)
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(http.MethodGet, "/api/request-records/?p=1&page_size=20", nil)
	context.Set("role", common.RoleRootUser)
	context.Set("id", 1)

	ListRequestContentAudits(context)

	require.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Total int   `json:"total"`
			Items []any `json:"items"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Equal(t, 0, payload.Data.Total)
	require.Empty(t, payload.Data.Items)
}

func TestUpdateOptionRejectsRequestContentAuditEnableWithoutEncryptionSecret(t *testing.T) {
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

	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)
	context.Request = httptest.NewRequest(
		http.MethodPut,
		"/api/option/",
		strings.NewReader(`{"key":"request_content_audit.enabled","value":true}`),
	)
	context.Set("role", common.RoleRootUser)

	UpdateOption(context)

	require.Equal(t, http.StatusOK, response.Code)
	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.False(t, payload.Success)
	require.Contains(t, payload.Message, "稳定的 CRYPTO_SECRET 或 SESSION_SECRET")
}

func TestWriteRequestContentAuditErrorMapsUnavailableKey(t *testing.T) {
	response := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(response)

	writeRequestContentAuditError(context, http.StatusInternalServerError, auditstore.ErrEncryptionKeyUnavailable)

	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.False(t, payload.Success)
	require.Equal(t, "request content audit encryption key is unavailable", payload.Message)
}
