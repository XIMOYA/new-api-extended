// relay/channel/claude/narrafork_quota_event_test.go
// 验证 Anthropic Messages 流中 quotaBalanceEvent 位于 message_stop 之前。
package claude

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestHandleStreamResponseDataWritesQuotaEventBeforeMessageStop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	info := &relaycommon.RelayInfo{
		IsStream:     true,
		RelayFormat:  types.RelayFormatClaude,
		StreamStatus: relaycommon.NewStreamStatus(),
		NarraForkQuotaEvent: relaycommon.NarraForkQuotaEventConfig{
			Activated:          true,
			BalanceSource:      "custom",
			CustomQuotaBalance: "custom-balance",
		},
	}
	claudeInfo := &ClaudeResponseInfo{
		Usage: &dto.Usage{
			PromptTokens:     1,
			CompletionTokens: 1,
			TotalTokens:      2,
		},
		ResponseText: strings.Builder{},
		Done:         true,
	}

	err := HandleStreamResponseData(c, info, claudeInfo, `{"type":"message_stop"}`)
	require.Nil(t, err)

	got := recorder.Body.String()
	require.Equal(t, 1, strings.Count(got, "event: quotaBalanceEvent\n"))
	quotaIndex := strings.Index(got, "event: quotaBalanceEvent\n")
	stopIndex := strings.Index(got, "event: message_stop\n")
	require.GreaterOrEqual(t, quotaIndex, 0)
	require.Greater(t, stopIndex, quotaIndex)
}
