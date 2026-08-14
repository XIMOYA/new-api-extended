// setting/request_content_setting/request_content_setting.go
// 请求内容审计配置：控制记录开关、查看权限、保留期与加密文件存储参数。
package request_content_setting

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
)

const OptionPrefix = "request_content_audit."

const (
	DefaultRetentionDays = 30
	MinRetentionDays     = 1
	MaxRetentionDays     = 3650
	DefaultMaxRecordMB   = 128
	MinMaxRecordMB       = 1
	MaxMaxRecordMB       = 128
	DefaultMaxAssetMB    = 32
	MinMaxAssetMB        = 1
	MaxMaxAssetMB        = 128
	DefaultChunkSize     = 64 << 10
	MinChunkSize         = 4 << 10
	MaxChunkSize         = 16 << 20
	MaxAdminAllowlist    = 1000
)

type RequestContentSetting struct {
	Enabled        bool   `json:"enabled"`
	AllowUserView  bool   `json:"allow_user_view"`
	AdminAllowlist []int  `json:"admin_allowlist"`
	RetentionDays  int    `json:"retention_days"`
	StoragePath    string `json:"storage_path"`
	MaxRecordBytes int64  `json:"max_record_bytes"`
	MaxAssetBytes  int64  `json:"max_asset_bytes"`
	ChunkSizeBytes int    `json:"chunk_size_bytes"`
}

var requestContentSetting = RequestContentSetting{
	Enabled:        false,
	AllowUserView:  false,
	AdminAllowlist: []int{},
	RetentionDays:  DefaultRetentionDays,
	StoragePath:    "request-content-audit",
	MaxRecordBytes: int64(DefaultMaxRecordMB) << 20,
	MaxAssetBytes:  int64(DefaultMaxAssetMB) << 20,
	ChunkSizeBytes: DefaultChunkSize,
}

func init() {
	config.GlobalConfig.Register("request_content_audit", &requestContentSetting)
}

func GetSettings() RequestContentSetting {
	settings := requestContentSetting
	settings.AdminAllowlist = append([]int(nil), requestContentSetting.AdminAllowlist...)
	settings.RetentionDays = NormalizeRetentionDays(settings.RetentionDays)
	settings.StoragePath = NormalizeStoragePath(settings.StoragePath)
	settings.MaxRecordBytes = NormalizeMaxBytes(settings.MaxRecordBytes, int64(DefaultMaxRecordMB)<<20, int64(MinMaxRecordMB)<<20, int64(MaxMaxRecordMB)<<20)
	settings.MaxAssetBytes = NormalizeMaxBytes(settings.MaxAssetBytes, int64(DefaultMaxAssetMB)<<20, int64(MinMaxAssetMB)<<20, int64(MaxMaxAssetMB)<<20)
	settings.ChunkSizeBytes = NormalizeChunkSize(settings.ChunkSizeBytes)
	return settings
}

func NormalizeRetentionDays(value int) int {
	if value < MinRetentionDays || value > MaxRetentionDays {
		return DefaultRetentionDays
	}
	return value
}

func NormalizeStoragePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "." {
		return "request-content-audit"
	}
	return value
}

func NormalizeMaxBytes(value, fallback, minimum, maximum int64) int64 {
	if value < minimum || value > maximum {
		return fallback
	}
	return value
}

func NormalizeChunkSize(value int) int {
	if value < MinChunkSize || value > MaxChunkSize {
		return DefaultChunkSize
	}
	return value
}

func IsOptionKey(key string) bool {
	return strings.HasPrefix(strings.TrimSpace(key), OptionPrefix)
}

func ValidateOption(key string, value string) error {
	if !IsOptionKey(key) {
		return nil
	}
	field := strings.TrimPrefix(strings.TrimSpace(key), OptionPrefix)
	value = strings.TrimSpace(value)
	switch field {
	case "enabled", "allow_user_view":
		if _, err := strconv.ParseBool(value); err != nil {
			return fmt.Errorf("%s must be a boolean", key)
		}
	case "admin_allowlist":
		var userIDs []int
		if err := common.UnmarshalJsonStr(value, &userIDs); err != nil {
			return fmt.Errorf("%s must be a JSON array of user IDs", key)
		}
		if len(userIDs) > MaxAdminAllowlist {
			return fmt.Errorf("%s contains too many users", key)
		}
		seen := make(map[int]struct{}, len(userIDs))
		for _, userID := range userIDs {
			if userID <= 0 {
				return fmt.Errorf("%s contains an invalid user ID", key)
			}
			if _, exists := seen[userID]; exists {
				return fmt.Errorf("%s contains duplicate user ID %d", key, userID)
			}
			seen[userID] = struct{}{}
		}
	case "retention_days":
		value, err := strconv.Atoi(value)
		if err != nil || value < MinRetentionDays || value > MaxRetentionDays {
			return fmt.Errorf("%s must be between %d and %d", key, MinRetentionDays, MaxRetentionDays)
		}
	case "storage_path":
		if value == "" || strings.IndexByte(value, 0) >= 0 {
			return errors.New("request content audit storage path is invalid")
		}
		cleaned := filepath.Clean(filepath.FromSlash(value))
		if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
			return fmt.Errorf("%s must not escape its configured data root", key)
		}
	case "max_record_bytes":
		bytes, err := strconv.ParseInt(value, 10, 64)
		if err != nil || bytes < int64(MinMaxRecordMB)<<20 || bytes > int64(MaxMaxRecordMB)<<20 {
			return fmt.Errorf("%s must be between %d MiB and %d MiB", key, MinMaxRecordMB, MaxMaxRecordMB)
		}
	case "max_asset_bytes":
		bytes, err := strconv.ParseInt(value, 10, 64)
		if err != nil || bytes < int64(MinMaxAssetMB)<<20 || bytes > int64(MaxMaxAssetMB)<<20 {
			return fmt.Errorf("%s must be between %d MiB and %d MiB", key, MinMaxAssetMB, MaxMaxAssetMB)
		}
	case "chunk_size_bytes":
		chunkSize, err := strconv.Atoi(value)
		if err != nil || chunkSize < MinChunkSize || chunkSize > MaxChunkSize {
			return fmt.Errorf("%s must be between %d and %d bytes", key, MinChunkSize, MaxChunkSize)
		}
	default:
		return fmt.Errorf("unknown request content audit setting: %s", key)
	}
	return nil
}
