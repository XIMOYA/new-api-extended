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
	// preludeSent 标记前导事件（Claude 的 message_start/content_block_start 等）是否已发出。
	// 接力时必须抑制第二段流的前导事件，否则客户端会收到重复的消息开始标记。
	preludeSent bool
	// handoffCount 已发生的接力次数，用于日志与防止无限接力。
	handoffCount int
}

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

// MarkPreludeSent 记录前导事件已发出。
func (s *StreamRelayState) MarkPreludeSent() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.preludeSent = true
}

// IsPreludeSent 表示前导事件是否已经发过，接力时据此抑制重复的消息开始事件。
func (s *StreamRelayState) IsPreludeSent() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.preludeSent
}

// BeginHandoff 记录一次接力并返回累计接力次数。
func (s *StreamRelayState) BeginHandoff() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handoffCount++
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
