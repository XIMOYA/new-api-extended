package common

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 接力状态由流式主干的 goroutine 写入、重试循环在主协程读取，必须并发安全。
func TestStreamRelayState_ConcurrentAppendIsSafe(t *testing.T) {
	t.Parallel()
	s := NewStreamRelayState()

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.AppendDeliveredText("x")
			_ = s.DeliveredText()
			_ = s.IsMessageStarted()
		}()
	}
	wg.Wait()

	assert.Len(t, s.DeliveredText(), 50)
	assert.True(t, s.HasDeliveredText())
}

func TestStreamRelayState_MessageStartAndHandoffTracking(t *testing.T) {
	t.Parallel()
	s := NewStreamRelayState()

	assert.False(t, s.IsMessageStarted())
	assert.False(t, s.HasDeliveredText())
	assert.Equal(t, 0, s.HandoffCount())

	s.MarkMessageStarted()
	s.AppendDeliveredText("前半段")
	s.AppendDeliveredText("后半段")

	assert.True(t, s.IsMessageStarted())
	assert.Equal(t, "前半段后半段", s.DeliveredText())
	assert.Equal(t, 1, s.BeginHandoff())
	assert.Equal(t, 2, s.BeginHandoff())
	assert.Equal(t, 2, s.HandoffCount())
}

func TestStreamRelayState_NilSafe(t *testing.T) {
	t.Parallel()
	var s *StreamRelayState

	s.AppendDeliveredText("ignored")
	s.MarkMessageStarted()

	assert.Equal(t, "", s.DeliveredText())
	assert.False(t, s.HasDeliveredText())
	assert.False(t, s.IsMessageStarted())
	assert.Equal(t, 0, s.BeginHandoff())
	assert.Equal(t, 0, s.HandoffCount())
}

// 空字符串增量不应污染已输出内容，否则会误判"已有内容发出"而允许接力。
func TestStreamRelayState_EmptyTextIsIgnored(t *testing.T) {
	t.Parallel()
	s := NewStreamRelayState()

	s.AppendDeliveredText("")

	assert.False(t, s.HasDeliveredText())
	assert.Equal(t, "", s.DeliveredText())
}
