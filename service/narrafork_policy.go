// service/narrafork_policy.go
// NarraFork 用户组/用户策略的请求上下文装载与短期缓存。
package service

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
	"github.com/gin-gonic/gin"
)

const narraForkPolicyCacheTTL = 30 * time.Second

type narraForkPolicyCacheEntry struct {
	patch     *narrafork_setting.NarraForkPolicyPatch
	expiresAt time.Time
}

var narraForkPolicyCache = struct {
	sync.RWMutex
	entries map[string]narraForkPolicyCacheEntry
}{
	entries: make(map[string]narraForkPolicyCacheEntry),
}

func PrepareNarraForkQuotaPolicyContext(c *gin.Context) {
	if c == nil {
		return
	}
	overrides := narrafork_setting.NarraForkPolicyOverrides{}
	if group := strings.TrimSpace(c.GetString(string(constant.ContextKeyUserGroup))); group != "" {
		overrides.Group = getCachedNarraForkPolicy(model.NarraForkPolicyScopeGroup, group)
	}
	if userID := c.GetInt(string(constant.ContextKeyUserId)); userID > 0 {
		overrides.User = getCachedNarraForkPolicy(model.NarraForkPolicyScopeUser, strconv.Itoa(userID))
	}
	common.SetContextKey(c, constant.ContextKeyNarraForkPolicyOverrides, overrides)
}

func getCachedNarraForkPolicy(scopeType, scopeKey string) *narrafork_setting.NarraForkPolicyPatch {
	if model.DB == nil {
		return nil
	}
	cacheKey := scopeType + ":" + scopeKey
	now := time.Now()
	narraForkPolicyCache.RLock()
	entry, ok := narraForkPolicyCache.entries[cacheKey]
	narraForkPolicyCache.RUnlock()
	if ok && now.Before(entry.expiresAt) {
		return entry.patch
	}

	patch, err := model.GetNarraForkQuotaEventPolicy(scopeType, scopeKey)
	if err != nil {
		common.SysLog("failed to load NarraFork policy: " + err.Error())
		patch = nil
	}
	narraForkPolicyCache.Lock()
	narraForkPolicyCache.entries[cacheKey] = narraForkPolicyCacheEntry{
		patch:     patch,
		expiresAt: now.Add(narraForkPolicyCacheTTL),
	}
	narraForkPolicyCache.Unlock()
	return patch
}

func InvalidateNarraForkQuotaPolicyCache(scopeType, scopeKey string) {
	cacheKey := model.NormalizeNarraForkPolicyScope(scopeType) + ":" + strings.TrimSpace(scopeKey)
	narraForkPolicyCache.Lock()
	delete(narraForkPolicyCache.entries, cacheKey)
	narraForkPolicyCache.Unlock()
}

func ClearNarraForkQuotaPolicyCache() {
	narraForkPolicyCache.Lock()
	narraForkPolicyCache.entries = make(map[string]narraForkPolicyCacheEntry)
	narraForkPolicyCache.Unlock()
}
