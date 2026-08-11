// relay/helper/narrafork_quota_event.go
// 安全写出 NarraFork quotaBalanceEvent，并处理上游同名 SSE 事件透传。
package helper

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

type narraForkQuotaEventPayload struct {
	QuotaBalance         string                 `json:"quotaBalance"`
	DetailedQuotaBalance string                 `json:"detailedQuotaBalance,omitempty"`
	Extra                map[string]interface{} `json:"extra,omitempty"`
}

func WriteNarraForkQuotaEvent(c *gin.Context, info *relaycommon.RelayInfo, usage *dto.Usage) error {
	if !relaycommon.CanWriteNarraForkQuotaEvent(info) {
		return nil
	}

	balance, err := service.BuildNarraForkQuotaBalance(c, info, usage)
	if err != nil {
		common.SysLog("skip NarraFork quota event: " + err.Error())
		return nil
	}
	payload := narraForkQuotaEventPayload{
		QuotaBalance:         balance.QuotaBalance,
		DetailedQuotaBalance: balance.DetailedQuotaBalance,
		Extra:                balance.Extra,
	}
	data, err := common.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal NarraFork quota event: %w", err)
	}
	if err := WriteSSEEvent(c, relaycommon.NarraForkQuotaEventName, string(data)); err != nil {
		return err
	}
	info.NarraForkQuotaEvent.MarkEventSent()
	return nil
}

func WriteSSEEvent(c *gin.Context, eventName string, data string) error {
	if c == nil || c.Writer == nil {
		return fmt.Errorf("context or writer is nil")
	}
	if requestContextDone(c) {
		return fmt.Errorf("request context done: %w", c.Request.Context().Err())
	}

	eventName = strings.TrimSpace(eventName)
	if eventName == "" {
		return fmt.Errorf("SSE event name is empty")
	}

	var builder strings.Builder
	builder.WriteString("event: ")
	builder.WriteString(eventName)
	builder.WriteString("\n")
	for _, line := range strings.Split(data, "\n") {
		builder.WriteString("data: ")
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	builder.WriteString("\n")
	if _, err := c.Writer.Write([]byte(builder.String())); err != nil {
		return fmt.Errorf("write SSE event failed: %w", err)
	}
	return FlushWriter(c)
}

func ForwardNarraForkQuotaEvent(c *gin.Context, info *relaycommon.RelayInfo, data string) error {
	if info == nil || !info.NarraForkQuotaEvent.ObserveUpstreamEvent() {
		return nil
	}
	return WriteSSEEvent(c, relaycommon.NarraForkQuotaEventName, data)
}
