// controller/relay_silent_switch_test.go
// 验证静默切换在中继出口的行为：上游错误替换成统一文案并保留状态码，
// 本站侧错误原样返回，且开关关闭时与原行为一致。
package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type taskErrorFixture struct {
	Message    string
	StatusCode int
	LocalError bool
}

func (fixture *taskErrorFixture) toTaskError() *taskdto.TaskError {
	return &taskdto.TaskError{
		Code:       "upstream_failed",
		Message:    fixture.Message,
		StatusCode: fixture.StatusCode,
		LocalError: fixture.LocalError,
	}
}

func withSilentSwitch(t *testing.T, enabled bool) {
	t.Helper()
	previous := operation_setting.SilentChannelSwitchEnabled
	t.Cleanup(func() { operation_setting.SilentChannelSwitchEnabled = previous })
	operation_setting.SilentChannelSwitchEnabled = enabled
}

func TestShouldRetryForcesSwitchOnUpstreamQuotaWith400(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	// 上游把额度不足回成 400：默认状态码规则不重试，开启静默切换后必须继续换渠道。
	quotaError := types.NewOpenAIError(
		errors.New("You exceeded your current quota"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
	)

	withSilentSwitch(t, false)
	assert.False(t, shouldRetry(context, quotaError, 3), "开关关闭时保持原有的 400 不重试")

	withSilentSwitch(t, true)
	assert.True(t, shouldRetry(context, quotaError, 3))
}

func TestShouldRetryKeepsLocalQuotaErrorTerminal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	withSilentSwitch(t, true)

	userQuota := types.NewError(errors.New("用户额度不足"), types.ErrorCodeInsufficientUserQuota,
		types.ErrOptionWithStatusCode(http.StatusBadRequest))
	assert.False(t, shouldRetry(context, userQuota, 3), "本站额度不足不该被当成渠道故障重试")
}

// 与 service 包内的 ginKeyChannelAffinitySkipRetry 保持一致，
// 这里直接写字面量是为了在 controller 包里复现"亲和已锁定不重试"的现场。
const affinitySkipRetryGinKey = "channel_affinity_skip_retry_on_failure"

func TestShouldRetryBlockedByChannelAffinitySkipRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	withSilentSwitch(t, true)

	quotaError := types.NewOpenAIError(
		errors.New("You exceeded your current quota"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusBadRequest,
	)

	// 亲和规则开着"失败后不重试"：即使静默切换开着也不换渠道，错误直接回给用户。
	context.Set(affinitySkipRetryGinKey, true)
	require.False(t, shouldRetry(context, quotaError, 3), "亲和锁定优先于静默切换")

	// 上游资源耗尽时 processChannelError 会清掉亲和缓存并解除锁定，此时必须恢复换渠道。
	context.Set(affinitySkipRetryGinKey, false)
	assert.True(t, shouldRetry(context, quotaError, 3))
}

func TestRespondTaskErrorHidesUpstreamMessage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withSilentSwitch(t, true)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	upstream := &taskErrorFixture{Message: "upstream account is out of credit", StatusCode: http.StatusBadRequest}
	respondTaskError(context, upstream.toTaskError())
	require.Equal(t, http.StatusBadRequest, recorder.Code, "状态码必须保留")
	assert.Contains(t, recorder.Body.String(), operation_setting.DefaultSilentChannelSwitchMessage)
	assert.NotContains(t, recorder.Body.String(), "out of credit")

	recorder = httptest.NewRecorder()
	context, _ = gin.CreateTestContext(recorder)
	local := &taskErrorFixture{Message: "本站参数校验失败", StatusCode: http.StatusBadRequest, LocalError: true}
	respondTaskError(context, local.toTaskError())
	assert.Contains(t, recorder.Body.String(), "本站参数校验失败")
}

func TestRespondTaskErrorKeepsMessageWhenSwitchDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	withSilentSwitch(t, false)

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	upstream := &taskErrorFixture{Message: "upstream account is out of credit", StatusCode: http.StatusInternalServerError}
	respondTaskError(context, upstream.toTaskError())
	assert.Contains(t, recorder.Body.String(), "out of credit")
}
