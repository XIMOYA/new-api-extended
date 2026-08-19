package common

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type StreamEndReason string

const (
	StreamEndReasonNone        StreamEndReason = ""
	StreamEndReasonDone        StreamEndReason = "done"
	StreamEndReasonTimeout     StreamEndReason = "timeout"
	StreamEndReasonClientGone  StreamEndReason = "client_gone"
	StreamEndReasonScannerErr  StreamEndReason = "scanner_error"
	StreamEndReasonHandlerStop StreamEndReason = "handler_stop"
	StreamEndReasonEOF         StreamEndReason = "eof"
	StreamEndReasonPanic       StreamEndReason = "panic"
	StreamEndReasonPingFail    StreamEndReason = "ping_fail"
)

const maxStreamErrorEntries = 20

type StreamErrorEntry struct {
	Message   string
	Timestamp time.Time
}

type StreamStatus struct {
	EndReason StreamEndReason
	EndError  error
	endOnce   sync.Once

	mu         sync.Mutex
	Errors     []StreamErrorEntry
	ErrorCount int
}

func NewStreamStatus() *StreamStatus {
	return &StreamStatus{}
}

func (s *StreamStatus) SetEndReason(reason StreamEndReason, err error) {
	if s == nil {
		return
	}
	s.endOnce.Do(func() {
		s.EndReason = reason
		s.EndError = err
	})
}

func (s *StreamStatus) RecordError(msg string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ErrorCount++
	if len(s.Errors) < maxStreamErrorEntries {
		s.Errors = append(s.Errors, StreamErrorEntry{
			Message:   msg,
			Timestamp: time.Now(),
		})
	}
}

func (s *StreamStatus) HasErrors() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ErrorCount > 0
}

func (s *StreamStatus) TotalErrorCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ErrorCount
}

func (s *StreamStatus) IsNormalEnd() bool {
	if s == nil {
		return true
	}
	return s.EndReason == StreamEndReasonDone ||
		s.EndReason == StreamEndReasonEOF ||
		s.EndReason == StreamEndReasonHandlerStop
}

// IsUpstreamInterrupted 判断流是否被上游侧异常打断，而不是正常收尾。
//
// 判据要精准，误判两个方向的代价都很高：漏判会让被截断的回答按成功计费，
// 误判会把本已完整的回答当成失败并触发重试。
//   - client_gone 是下游客户端主动断开，不属于上游故障，必须排除；
//   - EOF 需要结合是否收到过数据来判断：完全没收到任何 chunk 说明上游直接断开，
//     属于异常；已经收到过 chunk 的 EOF 无法区分「上游正常结束但未发 [DONE]」
//     与「中途断流」，为避免误判正常请求，这里不视为中断。
//
// receivedResponseCount 传入本次已从上游接收到的数据块数量。
func (s *StreamStatus) IsUpstreamInterrupted(receivedResponseCount int) bool {
	if s == nil {
		return false
	}
	switch s.EndReason {
	case StreamEndReasonTimeout, StreamEndReasonScannerErr, StreamEndReasonPanic, StreamEndReasonPingFail:
		return true
	case StreamEndReasonEOF:
		return receivedResponseCount == 0
	default:
		return false
	}
}

func (s *StreamStatus) Summary() string {
	if s == nil {
		return "StreamStatus<nil>"
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "reason=%s", s.EndReason)
	if s.EndError != nil {
		fmt.Fprintf(b, " end_error=%q", s.EndError.Error())
	}
	s.mu.Lock()
	if s.ErrorCount > 0 {
		fmt.Fprintf(b, " soft_errors=%d", s.ErrorCount)
	}
	s.mu.Unlock()
	return b.String()
}
