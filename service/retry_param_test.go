package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 重试循环每失败一次就会登记一个渠道，同一渠道可能因多 key 等原因被重复登记。
// 名单必须去重且忽略非法 id，否则会被无意义地放大。
func TestRetryParamExcludeChannel(t *testing.T) {
	t.Parallel()

	param := &RetryParam{}
	require.Empty(t, param.GetExcludedChannels())

	param.ExcludeChannel(7)
	param.ExcludeChannel(7)
	param.ExcludeChannel(9)
	param.ExcludeChannel(0)
	param.ExcludeChannel(-1)

	require.Equal(t, []int{7, 9}, param.GetExcludedChannels())
}
