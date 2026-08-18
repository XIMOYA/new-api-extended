package controller

import (
	"errors"
	"net/http"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 上游余额耗尽必须能跨过「400 不重试」的默认状态码区间。
// Anthropic 余额耗尽正是 400，若这里判成不重试，下游用户就会直接收到余额错误，
// 无感切换随之失效。该契约依赖 channel: 前缀经 types.IsChannelError 生效。
func TestShouldRetryUpstreamQuotaExhausted(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name       string
		errorCode  types.ErrorCode
		statusCode int
		retryTimes int
		expected   bool
	}{
		{
			name:       "upstream quota exhausted with 400 retries",
			errorCode:  types.ErrorCodeChannelUpstreamQuotaExhausted,
			statusCode: http.StatusBadRequest,
			retryTimes: 1,
			expected:   true,
		},
		{
			// 渠道错误不消耗剩余次数判断，与既有 channel:no_available_key 行为一致。
			name:       "upstream quota exhausted retries even without remaining retry budget",
			errorCode:  types.ErrorCodeChannelUpstreamQuotaExhausted,
			statusCode: http.StatusBadRequest,
			retryTimes: 0,
			expected:   true,
		},
		{
			name:       "upstream quota exhausted with 402 retries",
			errorCode:  types.ErrorCodeChannelUpstreamQuotaExhausted,
			statusCode: http.StatusPaymentRequired,
			retryTimes: 1,
			expected:   true,
		},
		{
			// 对照组：未被归类的普通 400 仍然不重试，保持既有行为不变。
			name:       "ordinary bad request still does not retry",
			errorCode:  types.ErrorCodeBadResponseStatusCode,
			statusCode: http.StatusBadRequest,
			retryTimes: 1,
			expected:   false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(nil)

			newAPIError := types.NewErrorWithStatusCode(
				errors.New("upstream failure"),
				tc.errorCode,
				tc.statusCode,
			)

			require.Equal(t, tc.expected, shouldRetry(c, nil, newAPIError, tc.retryTimes))
		})
	}
}

// 一旦有 chunk 写给客户端，SSE 的 200 响应头和已发内容就无法撤回。
// 此时整轮重发会让客户端收到两段拼接的流，因此必须停止重试，交由接力续写处理。
func TestShouldRetryStopsAfterResponseSentToClient(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testCases := []struct {
		name              string
		sendResponseCount int
		errorCode         types.ErrorCode
		expected          bool
	}{
		{
			name:              "nothing sent yet allows channel error retry",
			sendResponseCount: 0,
			errorCode:         types.ErrorCodeChannelUpstreamQuotaExhausted,
			expected:          true,
		},
		{
			// 即使是渠道错误（原本无条件重试），已写出内容后也必须停下。
			name:              "already sent blocks channel error retry",
			sendResponseCount: 1,
			errorCode:         types.ErrorCodeChannelUpstreamQuotaExhausted,
			expected:          false,
		},
		{
			name:              "already sent blocks retryable status code",
			sendResponseCount: 5,
			errorCode:         types.ErrorCodeBadResponseStatusCode,
			expected:          false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(nil)

			info := &relaycommon.RelayInfo{SendResponseCount: tc.sendResponseCount}
			newAPIError := types.NewErrorWithStatusCode(
				errors.New("upstream failure"),
				tc.errorCode,
				http.StatusInternalServerError,
			)

			require.Equal(t, tc.expected, shouldRetry(c, info, newAPIError, 3))
		})
	}
}
