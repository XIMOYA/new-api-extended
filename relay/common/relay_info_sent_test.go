// relay/common/relay_info_sent_test.go
// 覆盖 HasSentToClient：判断「响应是否已经发给客户端」。
// 该判据同时被重试守卫和失败落账依赖，两条路径都不能误判。
package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newSentTestContext(t *testing.T) *gin.Context {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	return c
}

func TestHasSentToClient_ConvertCountPath(t *testing.T) {
	c := newSentTestContext(t)

	// 跨格式转换路径：靠 SendResponseCount 识别，此时 writer 还没写
	info := &RelayInfo{SendResponseCount: 1}
	require.True(t, info.HasSentToClient(c))
}

func TestHasSentToClient_NativePassthroughPath(t *testing.T) {
	c := newSentTestContext(t)

	info := &RelayInfo{}
	require.False(t, info.HasSentToClient(c), "尚未写出任何内容")

	// 原生透传：不递增计数，只有 writer 有痕迹
	c.Writer.WriteHeader(http.StatusOK)
	_, err := c.Writer.Write([]byte("event: message_start\ndata: {}\n\n"))
	require.NoError(t, err)
	c.Writer.Flush()

	require.Zero(t, info.GetSendResponseCount(), "原生路径不递增转换计数")
	require.True(t, info.HasSentToClient(c), "必须靠 Writer.Written() 兜住")
}

func TestHasSentToClient_NilSafe(t *testing.T) {
	c := newSentTestContext(t)

	var info *RelayInfo
	require.False(t, info.HasSentToClient(c), "nil info 不应 panic")

	info = &RelayInfo{}
	require.False(t, info.HasSentToClient(nil), "nil context 不应 panic")

	info = &RelayInfo{SendResponseCount: 2}
	require.True(t, info.HasSentToClient(nil), "计数已表明发出过，无需 context")
}

// gin 的 WriteHeader 只记录状态码，size 仍是 noWritten，Written() 为 false；
// 真正发出要等 Write/Flush 触发 WriteHeaderNow。
// 这个边界对我们有利：只设了状态码还没发数据时，重试仍然是安全的。
func TestHasSentToClient_HeaderOnlyIsNotSentYet(t *testing.T) {
	c := newSentTestContext(t)

	info := &RelayInfo{}
	c.Writer.WriteHeader(http.StatusOK)
	require.False(t, info.HasSentToClient(c), "仅记录状态码尚未真正发出")

	// Flush 会触发 WriteHeaderNow，此时才算真的发了出去
	c.Writer.Flush()
	require.True(t, info.HasSentToClient(c), "Flush 后状态码已发出，不可撤回")
}
