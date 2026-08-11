// relay/channel/openai/narrafork_quota_event_test.go
// 验证 OpenAI Chat Completions 与 Responses 流中的 quotaBalanceEvent 终止顺序。
package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOaiStreamHandlerWritesQuotaEventBeforeDone(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"id":"chatcmpl_test","object":"chat.completion.chunk","created":1710000000,"model":"gpt-test","choices":[{"index":0,"delta":{"content":"hello"},"finish_reason":null}]}`,
		`data: {"id":"chatcmpl_test","object":"chat.completion.chunk","created":1710000000,"model":"gpt-test","choices":[{"index":0,"delta":{},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":3,"total_tokens":5}}`,
		`data: [DONE]`,
		``,
	}, "\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-test",
			ChannelSetting:    dto.ChannelSettings{},
		},
		IsStream:           true,
		RelayFormat:        types.RelayFormatOpenAI,
		ShouldIncludeUsage: true,
		DisablePing:        true,
		UserQuota:          1000,
		NarraForkQuotaEvent: relaycommon.NarraForkQuotaEventConfig{
			Activated:          true,
			BalanceSource:      "custom",
			CustomQuotaBalance: "custom-balance",
		},
	}

	usage, err := OaiStreamHandler(c, info, resp)
	require.Nil(t, err)
	require.NotNil(t, usage)
	require.Equal(t, 5, usage.TotalTokens)

	got := recorder.Body.String()
	require.Equal(t, 1, strings.Count(got, "event: quotaBalanceEvent\n"))
	requireOrderedSubstrings(t, got, `"content":"hello"`, "event: quotaBalanceEvent\n", "data: [DONE]")
}

func TestOaiResponsesStreamHandlerWritesQuotaEventAfterEOF(t *testing.T) {
	oldMode := gin.Mode()
	gin.SetMode(gin.TestMode)
	t.Cleanup(func() { gin.SetMode(oldMode) })

	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() { constant.StreamingTimeout = oldTimeout })

	body := strings.Join([]string{
		`data: {"type":"response.created","response":{"id":"resp_test","status":"in_progress"}}`,
		``,
	}, "\n")

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-test",
			ChannelSetting:    dto.ChannelSettings{},
		},
		IsStream:    true,
		RelayFormat: types.RelayFormatOpenAIResponses,
		DisablePing: true,
		UserQuota:   1000,
		NarraForkQuotaEvent: relaycommon.NarraForkQuotaEventConfig{
			Activated:          true,
			BalanceSource:      "custom",
			CustomQuotaBalance: "custom-balance",
		},
	}

	usage, err := OaiResponsesStreamHandler(c, info, resp)
	require.Nil(t, err)
	require.NotNil(t, usage)

	got := recorder.Body.String()
	require.Equal(t, 1, strings.Count(got, "event: quotaBalanceEvent\n"))
	require.Contains(t, got, `"quotaBalance":"custom-balance"`)
}
