// service/channel_affinity_skip_retry_test.go
// 验证渠道亲和的"失败后不重试"标记与缓存清理的联动：
// 上游资源耗尽时清掉亲和缓存后，本次请求必须重新获得换渠道的机会。
package service

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func buildSkipRetryAffinityContextForTest(t *testing.T, suffix string, channelID int) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	cache := getChannelAffinityCache()
	require.NoError(t, cache.SetWithTTL(suffix, channelID, time.Minute))

	setChannelAffinityContext(ctx, channelAffinityMeta{
		CacheKey:   channelAffinityCacheNamespace + ":" + suffix,
		TTLSeconds: 60,
		RuleName:   "claude cli trace",
		SkipRetry:  true,
		UsingGroup: "default",
		ModelName:  "claude-sonnet-4-5",
	})
	return ctx
}

func TestChannelAffinitySkipRetryReadsRuleFlag(t *testing.T) {
	ctx := buildSkipRetryAffinityContextForTest(t, "skip-retry-flag:default:claude", 46)

	// 规则里开着 SkipRetryOnFailure 时，中继侧的 shouldRetry 会在第一步就放弃重试。
	assert.True(t, ShouldSkipRetryAfterChannelAffinityFailure(ctx))
}

func TestClearCurrentChannelAffinityCacheUnblocksRetry(t *testing.T) {
	suffix := "clear-unblocks-retry:default:claude"
	ctx := buildSkipRetryAffinityContextForTest(t, suffix, 46)
	require.True(t, ShouldSkipRetryAfterChannelAffinityFailure(ctx))

	require.True(t, ClearCurrentChannelAffinityCache(ctx))

	// 缓存清掉后既不该再钉着旧渠道，也不该继续拦着重试。
	assert.False(t, ShouldSkipRetryAfterChannelAffinityFailure(ctx))
	_, found, err := getChannelAffinityCache().Get(suffix)
	require.NoError(t, err)
	assert.False(t, found, "亲和缓存应已被删除")
}

func TestClearCurrentChannelAffinityCacheWithoutContextIsNoop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	assert.False(t, ClearCurrentChannelAffinityCache(ctx))
	assert.False(t, ShouldSkipRetryAfterChannelAffinityFailure(ctx))
}
