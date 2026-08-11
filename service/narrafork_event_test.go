// service/narrafork_event_test.go
// 验证 NarraFork 请求指标、模板渲染和兼容 payload 预览。
package service

import (
	"testing"
	"time"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
	"github.com/stretchr/testify/require"
)

func TestBuildNarraForkEventMetrics(t *testing.T) {
	now := time.Date(2026, time.August, 10, 12, 0, 0, 0, time.UTC)
	info := &relaycommon.RelayInfo{
		RequestId:         "req_test",
		RetryIndex:        2,
		StartTime:         now.Add(-2 * time.Second),
		FirstResponseTime: now.Add(-1500 * time.Millisecond),
	}
	metrics := BuildNarraForkEventMetrics(info, now)
	require.Equal(t, int64(2000), metrics.LatencyMs)
	require.NotNil(t, metrics.TTFTMs)
	require.Equal(t, int64(500), *metrics.TTFTMs)
	require.Equal(t, "req_test", metrics.RequestID)
	require.Equal(t, 2, metrics.RetryCount)
}

func TestRenderNarraForkDetailTemplate(t *testing.T) {
	rendered, err := RenderNarraForkDetailTemplate(
		"Balance {{balance}} / {{latency_ms}} ms",
		map[string]string{"balance": "$1.20", "latency_ms": "42"},
	)
	require.NoError(t, err)
	require.Equal(t, "Balance $1.20 / 42 ms", rendered)
	require.Error(t, func() error {
		_, err := RenderNarraForkDetailTemplate("{{missing}}", map[string]string{})
		return err
	}())
}

func TestBuildNarraForkQuotaEventPreviewUsesCompatiblePayload(t *testing.T) {
	config := relaycommon.NewNarraForkQuotaEventConfig(narrafork_setting.GetSettings())
	config.ExposeExtra = true
	config.DetailTemplate = "{{balance}} / {{latency_ms}} ms / {{request_id}}"
	preview, err := BuildNarraForkQuotaEventPreview(config)
	require.NoError(t, err)
	require.Equal(t, relaycommon.NarraForkQuotaEventName, preview.EventName)
	require.NotContains(t, preview.Payload, "protocolVersion")
	require.NotContains(t, preview.Payload, "metrics")
	require.Equal(t, "$12.34 / 1234 ms / narrafork-preview-request", preview.Payload["detailedQuotaBalance"])
	require.Contains(t, preview.SSE, "event: "+relaycommon.NarraForkQuotaEventName)
	require.NotContains(t, preview.SSE, "protocolVersion")
}

func TestBuildNarraForkQuotaEventPreviewIncludesMetricsInDetails(t *testing.T) {
	config := relaycommon.NewNarraForkQuotaEventConfig(narrafork_setting.GetSettings())
	preview, err := BuildNarraForkQuotaEventPreview(config)
	require.NoError(t, err)
	detailed, ok := preview.Payload["detailedQuotaBalance"].(string)
	require.True(t, ok)
	require.Contains(t, detailed, "请求耗时: 1234 ms")
	require.Contains(t, detailed, "首 Token 延迟: 334 ms")
	require.Contains(t, detailed, "请求 ID: narrafork-preview-request")
	require.Contains(t, detailed, "重试次数: 1")
}
