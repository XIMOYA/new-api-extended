package common

import (
	"strings"
	"sync"
)

// StreamRelayState 承载跨渠道流式接力续写的状态。
//
// 背景：SSE 响应一旦发出 200 状态码并 flush 过任意 chunk，就无法回滚重来。
// 上游在流中途失败（典型是余额耗尽）时，整轮重发会让客户端收到两段拼接的流，
// 因此只能让接替的渠道在同一条连接里"接着写"，前导事件不再重复发送。
//
// 该状态跨重试复用，因此所有字段都通过锁保护：流式主干在 goroutine 中写入，
// 重试循环在主协程读取。
type StreamRelayState struct {
	mu sync.Mutex

	// deliveredText 已经真实发给客户端的回答文本，用于构造续写请求的 assistant 前缀。
	deliveredText strings.Builder
	// messageStarted 标记消息级前导事件 message_start 是否已发出。
	// 它是消息级的：一条 SSE 连接里只应出现一次，接力时第二段必须抑制。
	//
	// 注意不要把 content_block_start 并入这个语义。那是块级事件，
	// 一条消息里 text / thinking / tool_use 各自都会发一次，
	// 若一并抑制，普通流式请求都会丢失 block 起始事件。
	messageStarted bool
	// blockIndexOffset 接力后为第二段流的 content block index 附加的偏移量。
	// 第一段用掉了 [0, maxBlockIndex]，第二段必须从 maxBlockIndex+1 continue，
	// 否则客户端会看到"index 0 已在进行中却被重新 start"。
	blockIndexOffset int
	// maxBlockIndex 本段流中出现过的最大 content block index（已含偏移）。
	maxBlockIndex int
	// blockOpen 标记当前是否有未收到 content_block_stop 的 block。
	// 接力前需要据此补发 stop，保证客户端的 block 状态是闭合的。
	blockOpen bool
	// handoffCount 已发生的接力次数，用于日志与防止无限接力。
	handoffCount int
}

// maxDeliveredTextBytes 限制累积的续写前缀大小。
//
// 前缀会作为 assistant prompt 重新发给新上游，无上限累积会撞上游的上下文窗口
// 或 max_tokens，生成一个必然被拒绝的请求。超过该阈值就放弃接力，
// 退回普通失败处理（保留已输出内容并在流内收尾），比发一个注定失败的请求更好。
const maxDeliveredTextBytes = 32 * 1024


func NewStreamRelayState() *StreamRelayState {
	return &StreamRelayState{}
}

// AppendDeliveredText 累积一段已经发给客户端的回答文本。
func (s *StreamRelayState) AppendDeliveredText(text string) {
	if s == nil || text == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveredText.WriteString(text)
}

// DeliveredText 返回目前为止已发给客户端的全部回答文本。
func (s *StreamRelayState) DeliveredText() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deliveredText.String()
}

// HasDeliveredText 表示是否已经有回答内容落到客户端。
func (s *StreamRelayState) HasDeliveredText() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deliveredText.Len() > 0
}

// MarkMessageStarted 记录消息级前导事件 message_start 已发出。
func (s *StreamRelayState) MarkMessageStarted() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messageStarted = true
}

// IsMessageStarted 表示 message_start 是否已经发过。
// 接力时据此抑制第二段流重复的消息起始事件。
func (s *StreamRelayState) IsMessageStarted() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.messageStarted
}

// ObserveBlockIndex 记录一个已转发的 content block index（应传入偏移后的值），
// 并标记该 block 处于打开状态。
func (s *StreamRelayState) ObserveBlockIndex(index int) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if index > s.maxBlockIndex {
		s.maxBlockIndex = index
	}
	s.blockOpen = true
}

// CloseBlock 标记当前 block 已经收到 content_block_stop。
func (s *StreamRelayState) CloseBlock() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blockOpen = false
}

// HasOpenBlock 表示是否存在尚未闭合的 content block。
// 接力前若为真，需要补发一个 content_block_stop 让客户端状态闭合。
func (s *StreamRelayState) HasOpenBlock() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.blockOpen
}

// MaxBlockIndex 返回本条消息中出现过的最大 content block index（已含偏移）。
func (s *StreamRelayState) MaxBlockIndex() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxBlockIndex
}

// BlockIndexOffset 返回当前应加到上游 block index 上的偏移量。
func (s *StreamRelayState) BlockIndexOffset() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.blockIndexOffset
}

// DeliveredTextExceedsLimit 表示续写前缀是否已超出可安全重发的大小。
func (s *StreamRelayState) DeliveredTextExceedsLimit() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deliveredText.Len() > maxDeliveredTextBytes
}

// BeginHandoff 记录一次接力并返回累计接力次数。
//
// 同时把 block index 偏移推进到第一段之后：第二段上游会重新从 index 0 开始编号，
// 加上偏移才能延续同一条消息的 block 序列。
func (s *StreamRelayState) BeginHandoff() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handoffCount++
	s.blockIndexOffset = s.maxBlockIndex + 1
	s.blockOpen = false
	return s.handoffCount
}

// HandoffCount 返回已发生的接力次数。
func (s *StreamRelayState) HandoffCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.handoffCount
}
