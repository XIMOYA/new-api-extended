// service/request_content_audit_runtime.go
// 请求内容审计运行时：根据系统配置初始化持久化存储、加密密钥和授权查询入口。
package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	auditstore "github.com/QuantumNous/new-api/service/request_content_audit"
	"github.com/QuantumNous/new-api/setting/request_content_setting"
)

const requestContentAuditKeyID = "crypto-secret-v1"

var requestContentAuditRuntime struct {
	sync.RWMutex
	store     *auditstore.Store
	signature string
}

func EnsureRequestContentAuditStore(ctx context.Context) (*auditstore.Store, error) {
	settings := request_content_setting.GetSettings()
	root, err := requestContentAuditStorageRoot(settings.StoragePath)
	if err != nil {
		return nil, err
	}
	secret, secretErr := requestContentAuditEncryptionSecret()
	var keyProvider auditstore.KeyProvider
	keyFingerprint := "unavailable"
	if secretErr != nil {
		keyProvider = unavailableRequestContentAuditKeyProvider{}
	} else {
		keyValue := sha256.Sum256([]byte(secret))
		keyFingerprint = hex.EncodeToString(keyValue[:8])
		keyProvider = &auditstore.StaticKeyProvider{
			ActiveKeyId: requestContentAuditKeyID,
			Keys: map[string][]byte{
				requestContentAuditKeyID: keyValue[:],
			},
		}
	}
	signature := fmt.Sprintf("%s:%d:%d:%d:%s", root, settings.ChunkSizeBytes, settings.MaxRecordBytes, settings.MaxAssetBytes, keyFingerprint)

	requestContentAuditRuntime.RLock()
	store := requestContentAuditRuntime.store
	currentSignature := requestContentAuditRuntime.signature
	requestContentAuditRuntime.RUnlock()
	if store != nil && currentSignature == signature {
		return store, nil
	}

	requestContentAuditRuntime.Lock()
	defer requestContentAuditRuntime.Unlock()
	if requestContentAuditRuntime.store != nil && requestContentAuditRuntime.signature == signature {
		return requestContentAuditRuntime.store, nil
	}

	repository := model.NewRequestContentAuditRepository(model.DB)
	packRepository := model.NewRequestContentPackRepository(model.DB)
	packs, err := auditstore.NewPackStore(root, packRepository, keyProvider)
	if err != nil {
		return nil, err
	}
	store, err = auditstore.NewStore(
		root,
		repository,
		keyProvider,
		auditstore.WithChunkSize(settings.ChunkSizeBytes),
		auditstore.WithMaxContentSize(settings.MaxRecordBytes),
		auditstore.WithMaxAssetSize(settings.MaxAssetBytes),
		auditstore.WithPackStore(packs),
	)
	if err != nil {
		return nil, err
	}
	if err := store.Migrate(ctx); err != nil {
		return nil, err
	}
	requestContentAuditRuntime.store = store
	requestContentAuditRuntime.signature = signature
	return store, nil
}

func requestContentAuditEncryptionSecret() (string, error) {
	secret := strings.TrimSpace(os.Getenv("CRYPTO_SECRET"))
	if secret == "" {
		secret = strings.TrimSpace(os.Getenv("SESSION_SECRET"))
	}
	if secret == "" {
		return "", errors.New("request content audit requires a stable CRYPTO_SECRET or SESSION_SECRET")
	}
	return secret, nil
}

type unavailableRequestContentAuditKeyProvider struct{}

func (unavailableRequestContentAuditKeyProvider) ActiveKey(context.Context) (auditstore.EncryptionKey, error) {
	return auditstore.EncryptionKey{}, auditstore.ErrEncryptionKeyUnavailable
}

func (unavailableRequestContentAuditKeyProvider) Key(context.Context, string) ([]byte, error) {
	return nil, auditstore.ErrEncryptionKeyUnavailable
}

func RequestContentAuditEncryptionReady() bool {
	_, err := requestContentAuditEncryptionSecret()
	return err == nil
}

func RequestContentAuditEnabled() bool {
	return request_content_setting.GetSettings().Enabled
}

func RequestContentAuditUserViewEnabled() bool {
	return request_content_setting.GetSettings().AllowUserView
}

func RequestContentAuditAdminAllowlisted(userID int) bool {
	if userID <= 0 {
		return false
	}
	for _, allowedID := range request_content_setting.GetSettings().AdminAllowlist {
		if allowedID == userID {
			return true
		}
	}
	return false
}

func RequestContentAuditSettings() request_content_setting.RequestContentSetting {
	return request_content_setting.GetSettings()
}

func requestContentAuditStorageRoot(configuredPath string) (string, error) {
	configuredPath = strings.TrimSpace(configuredPath)
	if configuredPath == "" {
		return "", errors.New("request content audit storage path is empty")
	}
	if !filepath.IsAbs(configuredPath) {
		if common.LogDir == nil {
			return "", errors.New("request content audit log directory is unavailable")
		}
		configuredPath = filepath.Join(*common.LogDir, configuredPath)
	}
	root, err := filepath.Abs(configuredPath)
	if err != nil {
		return "", fmt.Errorf("resolve request content audit storage path: %w", err)
	}
	return filepath.Clean(root), nil
}
