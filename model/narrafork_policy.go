// model/narrafork_policy.go
// NarraFork 用户组与用户作用域策略的持久化、校验和查询。
package model

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
	"gorm.io/gorm"
)

const (
	NarraForkPolicyScopeGroup = "group"
	NarraForkPolicyScopeUser  = "user"
)

type NarraForkQuotaEventPolicy struct {
	ID        int64  `json:"id" gorm:"primaryKey"`
	ScopeType string `json:"scope_type" gorm:"type:varchar(16);not null;uniqueIndex:idx_narrafork_policy_scope"`
	ScopeKey  string `json:"scope_key" gorm:"type:varchar(128);not null;uniqueIndex:idx_narrafork_policy_scope"`
	Config    string `json:"config" gorm:"type:text;not null"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (NarraForkQuotaEventPolicy) TableName() string {
	return "narrafork_quota_event_policies"
}

func NormalizeNarraForkPolicyScope(scopeType string) string {
	switch strings.ToLower(strings.TrimSpace(scopeType)) {
	case NarraForkPolicyScopeGroup:
		return NarraForkPolicyScopeGroup
	case NarraForkPolicyScopeUser:
		return NarraForkPolicyScopeUser
	default:
		return ""
	}
}

func ValidateNarraForkPolicyScope(scopeType, scopeKey string) error {
	if NormalizeNarraForkPolicyScope(scopeType) == "" {
		return fmt.Errorf("invalid NarraFork policy scope type: %s", scopeType)
	}
	if strings.TrimSpace(scopeKey) == "" || len(scopeKey) > 128 {
		return fmt.Errorf("invalid NarraFork policy scope key")
	}
	return nil
}

func EncodeNarraForkPolicyPatch(patch narrafork_setting.NarraForkPolicyPatch) (string, error) {
	if err := patch.Validate(); err != nil {
		return "", err
	}
	data, err := common.Marshal(patch)
	if err != nil {
		return "", fmt.Errorf("marshal NarraFork policy: %w", err)
	}
	return string(data), nil
}

func DecodeNarraForkPolicyPatch(value string) (*narrafork_setting.NarraForkPolicyPatch, error) {
	patch := &narrafork_setting.NarraForkPolicyPatch{}
	if strings.TrimSpace(value) == "" {
		return patch, nil
	}
	if err := common.Unmarshal([]byte(value), patch); err != nil {
		return nil, fmt.Errorf("unmarshal NarraFork policy: %w", err)
	}
	if err := patch.Validate(); err != nil {
		return nil, err
	}
	return patch, nil
}

func GetNarraForkQuotaEventPolicy(scopeType, scopeKey string) (*narrafork_setting.NarraForkPolicyPatch, error) {
	if err := ValidateNarraForkPolicyScope(scopeType, scopeKey); err != nil {
		return nil, err
	}
	var row NarraForkQuotaEventPolicy
	if err := DB.Where("scope_type = ? AND scope_key = ?", NormalizeNarraForkPolicyScope(scopeType), strings.TrimSpace(scopeKey)).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return DecodeNarraForkPolicyPatch(row.Config)
}

func GetNarraForkQuotaEventPolicies(scopeType string, scopeKeys []string) (map[string]*narrafork_setting.NarraForkPolicyPatch, error) {
	scopeType = NormalizeNarraForkPolicyScope(scopeType)
	if scopeType == "" {
		return nil, fmt.Errorf("invalid NarraFork policy scope type")
	}
	query := DB.Where("scope_type = ?", scopeType)
	if len(scopeKeys) > 0 {
		query = query.Where("scope_key IN ?", scopeKeys)
	}
	var rows []NarraForkQuotaEventPolicy
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	policies := make(map[string]*narrafork_setting.NarraForkPolicyPatch, len(rows))
	for _, row := range rows {
		patch, err := DecodeNarraForkPolicyPatch(row.Config)
		if err != nil {
			return nil, fmt.Errorf("decode NarraFork policy %s/%s: %w", row.ScopeType, row.ScopeKey, err)
		}
		policies[row.ScopeKey] = patch
	}
	return policies, nil
}

func ListNarraForkQuotaEventPolicies(scopeType string) ([]NarraForkQuotaEventPolicy, error) {
	scopeType = NormalizeNarraForkPolicyScope(scopeType)
	if scopeType == "" {
		return nil, fmt.Errorf("invalid NarraFork policy scope type")
	}
	var rows []NarraForkQuotaEventPolicy
	if err := DB.Where("scope_type = ?", scopeType).Order("scope_key ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func UpsertNarraForkQuotaEventPolicy(scopeType, scopeKey string, patch narrafork_setting.NarraForkPolicyPatch) error {
	scopeType = NormalizeNarraForkPolicyScope(scopeType)
	scopeKey = strings.TrimSpace(scopeKey)
	if err := ValidateNarraForkPolicyScope(scopeType, scopeKey); err != nil {
		return err
	}
	config, err := EncodeNarraForkPolicyPatch(patch)
	if err != nil {
		return err
	}
	row := NarraForkQuotaEventPolicy{}
	result := DB.Where("scope_type = ? AND scope_key = ?", scopeType, scopeKey).First(&row)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		return result.Error
	}
	if result.Error == gorm.ErrRecordNotFound {
		row = NarraForkQuotaEventPolicy{ScopeType: scopeType, ScopeKey: scopeKey, Config: config}
		return DB.Create(&row).Error
	}
	return DB.Model(&row).Updates(map[string]interface{}{"config": config}).Error
}

func DeleteNarraForkQuotaEventPolicy(scopeType, scopeKey string) error {
	if err := ValidateNarraForkPolicyScope(scopeType, scopeKey); err != nil {
		return err
	}
	return DB.Where("scope_type = ? AND scope_key = ?", NormalizeNarraForkPolicyScope(scopeType), strings.TrimSpace(scopeKey)).Delete(&NarraForkQuotaEventPolicy{}).Error
}
