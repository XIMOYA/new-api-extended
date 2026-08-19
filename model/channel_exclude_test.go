package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// 渠道选择在同优先级内按权重随机，重试时若不剔除已失败渠道就可能反复命中同一个，
// 导致「切换到其它渠道」实际没有发生。同时剔除不能把候选清空，否则本可成功的请求会被打死。
func TestExcludeChannelIdsFromCandidates(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name              string
		channels          []int
		excludeChannelIds []int
		expected          []int
	}{
		{
			name:              "excludes failed channel",
			channels:          []int{1, 2, 3},
			excludeChannelIds: []int{2},
			expected:          []int{1, 3},
		},
		{
			name:              "excludes multiple failed channels",
			channels:          []int{1, 2, 3, 4},
			excludeChannelIds: []int{1, 3},
			expected:          []int{2, 4},
		},
		{
			// 关键回退：所有候选都失败过时必须回退到全量候选，而不是返回空导致无渠道可用。
			name:              "falls back to all candidates when everything excluded",
			channels:          []int{1, 2},
			excludeChannelIds: []int{1, 2},
			expected:          []int{1, 2},
		},
		{
			name:              "no exclusion keeps original order",
			channels:          []int{5, 6, 7},
			excludeChannelIds: nil,
			expected:          []int{5, 6, 7},
		},
		{
			name:              "unknown exclusion ids are harmless",
			channels:          []int{1, 2},
			excludeChannelIds: []int{99},
			expected:          []int{1, 2},
		},
		{
			name:              "empty candidates stay empty",
			channels:          []int{},
			excludeChannelIds: []int{1},
			expected:          []int{},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := excludeChannelIdsFromCandidates(tc.channels, tc.excludeChannelIds)
			require.Equal(t, tc.expected, got)
		})
	}
}

// DB 选择路径必须与内存缓存路径保持一致的剔除与回退语义，否则关闭内存缓存后行为会分叉。
func TestExcludeChannelIdsFromAbilities(t *testing.T) {
	t.Parallel()

	abilities := []Ability{
		{ChannelId: 1},
		{ChannelId: 2},
		{ChannelId: 3},
	}

	t.Run("excludes failed channel", func(t *testing.T) {
		t.Parallel()

		got := excludeChannelIdsFromAbilities(abilities, []int{2})
		require.Len(t, got, 2)
		require.Equal(t, 1, got[0].ChannelId)
		require.Equal(t, 3, got[1].ChannelId)
	})

	t.Run("falls back when everything excluded", func(t *testing.T) {
		t.Parallel()

		got := excludeChannelIdsFromAbilities(abilities, []int{1, 2, 3})
		require.Len(t, got, 3)
	})

	t.Run("no exclusion returns input", func(t *testing.T) {
		t.Parallel()

		got := excludeChannelIdsFromAbilities(abilities, nil)
		require.Len(t, got, 3)
	})
}
