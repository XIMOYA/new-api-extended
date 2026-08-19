// relay/channel/claude/stream_prelude_test.go
// 钉住 Claude 流式事件的层级语义。
//
// 这里防的是一个已经发生过的真实缺陷：曾把 message_start 与 content_block_start
// 放进同一个「前导事件」集合，导致任一事件都会置位标记、且都会被抑制。
// 后果是所有 Claude 原生流式请求（哪怕接力从未发生、开关关闭）都收不到
// content_block_start，客户端拿到没有 start 打头的 delta，破坏 Messages API 契约。
package claude

import (
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/require"
)

func blockIndex(i int) *int { return &i }

// message_start 是消息级事件：一条 SSE 连接里只应出现一次，重复的才抑制。
// content_block_start 是块级事件：一条消息里 text / thinking / tool_use 各发一次，
// 永远不能因为「前导已发过」而被抑制。
func TestClaudeStreamOnlyMessageStartIsSuppressed(t *testing.T) {
	info := &relaycommon.RelayInfo{StreamRelay: relaycommon.NewStreamRelayState()}

	require.False(t, shouldSuppressClaudeHandoffEvent(info, "message_start"),
		"首个 message_start 必须转发")

	trackClaudeDeliveredContent(info, &dto.ClaudeResponse{Type: "message_start"})

	require.True(t, shouldSuppressClaudeHandoffEvent(info, "message_start"),
		"重复的 message_start 才抑制")
	require.False(t, shouldSuppressClaudeHandoffEvent(info, "content_block_start"),
		"content_block_start 是块级事件，绝不能被抑制")
	require.False(t, shouldSuppressClaudeHandoffEvent(info, "content_block_delta"))
	require.False(t, shouldSuppressClaudeHandoffEvent(info, "content_block_stop"))
	require.False(t, shouldSuppressClaudeHandoffEvent(info, "message_delta"))
	require.False(t, shouldSuppressClaudeHandoffEvent(info, "message_stop"))
}

// 多 block 消息（thinking + text，或并发 tool_use）里，
// 每个 block 的 start 都必须原样转发。
func TestClaudeStreamMultipleBlockStartsAllPass(t *testing.T) {
	info := &relaycommon.RelayInfo{StreamRelay: relaycommon.NewStreamRelayState()}
	trackClaudeDeliveredContent(info, &dto.ClaudeResponse{Type: "message_start"})

	for _, idx := range []int{0, 1, 2} {
		require.False(t, shouldSuppressClaudeHandoffEvent(info, "content_block_start"),
			"第 %d 个 block 的 start 必须转发", idx)
		trackClaudeDeliveredContent(info, &dto.ClaudeResponse{
			Type:  "content_block_start",
			Index: blockIndex(idx),
		})
		trackClaudeDeliveredContent(info, &dto.ClaudeResponse{
			Type:  "content_block_stop",
			Index: blockIndex(idx),
		})
	}
	require.Equal(t, 2, info.StreamRelay.MaxBlockIndex())
	require.False(t, info.StreamRelay.HasOpenBlock(), "每个 block 都已闭合")
}

// 接力后第二段上游会重新从 index 0 编号，必须平移到第一段之后。
func TestClaudeStreamHandoffShiftsBlockIndex(t *testing.T) {
	info := &relaycommon.RelayInfo{StreamRelay: relaycommon.NewStreamRelayState()}
	trackClaudeDeliveredContent(info, &dto.ClaudeResponse{Type: "message_start"})

	// 第一段：用掉 index 0，中断时没有收到 stop
	first := &dto.ClaudeResponse{Type: "content_block_start", Index: blockIndex(0)}
	_, shifted := rewriteClaudeHandoffBlockIndex(info, first, `{"type":"content_block_start","index":0}`)
	require.False(t, shifted, "第一段不该有偏移")
	trackClaudeDeliveredContent(info, first)

	delta := &dto.ClaudeResponse{Type: "content_block_delta", Index: blockIndex(0),
		Delta: &dto.ClaudeMediaMessage{Type: "text_delta"}}
	delta.Delta.SetText("前半段")
	trackClaudeDeliveredContent(info, delta)
	require.True(t, info.StreamRelay.HasOpenBlock(), "中断时 block 仍打开")

	// 接力
	require.Equal(t, 1, info.StreamRelay.BeginHandoff())
	require.Equal(t, 1, info.StreamRelay.BlockIndexOffset())
	require.False(t, info.StreamRelay.HasOpenBlock(), "接力时重置打开标记")

	// 第二段：上游又从 0 开始
	second := &dto.ClaudeResponse{Type: "content_block_start", Index: blockIndex(0)}
	data, ok := rewriteClaudeHandoffBlockIndex(info, second, `{"type":"content_block_start","index":0}`)
	require.True(t, ok)
	require.Contains(t, data, `"index":1`, "透传报文里的 index 必须同步改写")
	require.Equal(t, 1, *second.Index)

	// thinking 先行的情况：上游 index 1 应落到 2
	third := &dto.ClaudeResponse{Type: "content_block_delta", Index: blockIndex(1)}
	data3, ok3 := rewriteClaudeHandoffBlockIndex(info, third, `{"type":"content_block_delta","index":1}`)
	require.True(t, ok3)
	require.Contains(t, data3, `"index":2`)
}

// 不带 index 的事件不参与平移，且不能 panic。
func TestClaudeStreamHandoffIgnoresIndexlessEvents(t *testing.T) {
	info := &relaycommon.RelayInfo{StreamRelay: relaycommon.NewStreamRelayState()}
	info.StreamRelay.ObserveBlockIndex(0)
	info.StreamRelay.BeginHandoff()

	for _, typ := range []string{"message_start", "message_delta", "message_stop", "ping"} {
		data := `{"type":"` + typ + `"}`
		out, ok := rewriteClaudeHandoffBlockIndex(info, &dto.ClaudeResponse{Type: typ}, data)
		require.False(t, ok, "%s 不携带 block index", typ)
		require.Equal(t, data, out)
	}

	// content_block_delta 但 index 缺省
	out, ok := rewriteClaudeHandoffBlockIndex(info,
		&dto.ClaudeResponse{Type: "content_block_delta"}, `{"type":"content_block_delta"}`)
	require.False(t, ok)
	require.Equal(t, `{"type":"content_block_delta"}`, out)
}
