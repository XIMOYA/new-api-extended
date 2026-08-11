// relay/common/narrafork_quota_test.go
// 验证 NarraFork 额度事件的请求激活条件与重复事件策略。
package common

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveNarraForkQuotaEvent_HeaderAndUserAgentActivation(t *testing.T) {
	t.Parallel()

	boolPtr := func(value bool) *bool { return &value }
	tests := []struct {
		name      string
		mode      string
		header    string
		userAgent string
		want      bool
	}{
		{name: "header", mode: narrafork_setting.ActivationModeHeaderOnly, header: "true", want: true},
		{name: "header requires true", mode: narrafork_setting.ActivationModeHeaderOnly, header: "false", userAgent: "NarraFork", want: false},
		{name: "user agent", mode: narrafork_setting.ActivationModeUserAgentOnly, userAgent: "NarraFork Client", want: true},
		{name: "or", mode: narrafork_setting.ActivationModeHeaderOrUserAgent, header: "true", want: true},
		{name: "always", mode: narrafork_setting.ActivationModeAlways, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
			c.Request.Header.Set(NarraForkQuotaEventHeader, tt.header)
			c.Request.Header.Set("User-Agent", tt.userAgent)

			info := &RelayInfo{
				IsStream:    true,
				RelayFormat: types.RelayFormatOpenAI,
				ChannelMeta: &ChannelMeta{
					ChannelOtherSettings: dto.ChannelOtherSettings{
						NarraFork: &dto.NarraForkQuotaEventSettings{
							Enabled:        boolPtr(true),
							ActivationMode: tt.mode,
						},
					},
				},
			}

			resolved := ResolveNarraForkQuotaEvent(c, info)
			assert.Equal(t, tt.want, resolved.Activated)
		})
	}
}

func TestResolveNarraForkQuotaEvent_UnsupportedFormatIsDisabled(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	info := &RelayInfo{
		IsStream:    true,
		RelayFormat: types.RelayFormatGemini,
		ChannelMeta: &ChannelMeta{
			ChannelOtherSettings: dto.ChannelOtherSettings{
				NarraFork: &dto.NarraForkQuotaEventSettings{
					Enabled:        func() *bool { value := true; return &value }(),
					ActivationMode: narrafork_setting.ActivationModeAlways,
				},
			},
		},
	}

	resolved := ResolveNarraForkQuotaEvent(c, info)
	require.False(t, resolved.Activated)
}

func TestApplyNarraForkUserDisplayPreference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mode    string
		allowed bool
		want    bool
	}{
		{name: "hide when allowed", mode: narrafork_setting.UserDisplayModeHide, allowed: true, want: false},
		{name: "hide ignored when disallowed", mode: narrafork_setting.UserDisplayModeHide, allowed: false, want: true},
		{name: "inherit keeps enabled", mode: narrafork_setting.UserDisplayModeInherit, allowed: true, want: true},
		{name: "show keeps enabled", mode: narrafork_setting.UserDisplayModeShow, allowed: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &NarraForkQuotaEventConfig{Enabled: true}
			ApplyNarraForkUserDisplayPreference(config, tt.mode, tt.allowed)
			assert.Equal(t, tt.want, config.Enabled)
		})
	}
}

func TestResolveNarraForkQuotaEvent_IgnoresUserHideWhenGlobalOverrideIsDisabled(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	c.Request.Header.Set(NarraForkQuotaEventHeader, "true")
	info := &RelayInfo{
		IsStream:    true,
		RelayFormat: types.RelayFormatOpenAI,
		UserSetting: dto.UserSetting{NarraForkDisplayMode: narrafork_setting.UserDisplayModeHide},
		ChannelMeta: &ChannelMeta{
			ChannelOtherSettings: dto.ChannelOtherSettings{
				NarraFork: &dto.NarraForkQuotaEventSettings{
					Enabled:        func() *bool { value := true; return &value }(),
					ActivationMode: narrafork_setting.ActivationModeAlways,
				},
			},
		},
	}

	resolved := ResolveNarraForkQuotaEvent(c, info)
	require.True(t, resolved.Activated)
}

func TestCanWriteNarraForkQuotaEventRejectsUnsuccessfulStreamEnd(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		reason StreamEndReason
		want   bool
	}{
		{name: "before terminal event", reason: StreamEndReasonNone, want: true},
		{name: "done", reason: StreamEndReasonDone, want: true},
		{name: "eof", reason: StreamEndReasonEOF, want: true},
		{name: "timeout", reason: StreamEndReasonTimeout, want: false},
		{name: "client gone", reason: StreamEndReasonClientGone, want: false},
		{name: "handler error", reason: StreamEndReasonHandlerStop, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := NewStreamStatus()
			status.SetEndReason(tt.reason, nil)
			info := &RelayInfo{
				StreamStatus: status,
				NarraForkQuotaEvent: NarraForkQuotaEventConfig{
					Activated: true,
				},
			}
			assert.Equal(t, tt.want, CanWriteNarraForkQuotaEvent(info))
		})
	}
}

func TestNarraForkQuotaEventConfig_DuplicatePolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		policy       string
		wantUpstream bool
		wantLocal    bool
	}{
		{name: "skip", policy: narrafork_setting.DuplicatePolicySkip, wantUpstream: true, wantLocal: false},
		{name: "replace", policy: narrafork_setting.DuplicatePolicyReplace, wantUpstream: false, wantLocal: true},
		{name: "always", policy: narrafork_setting.DuplicatePolicyAlways, wantUpstream: true, wantLocal: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &NarraForkQuotaEventConfig{
				Activated:       true,
				DuplicatePolicy: tt.policy,
			}
			assert.Equal(t, tt.wantUpstream, config.ObserveUpstreamEvent())
			assert.Equal(t, tt.wantLocal, config.ShouldSendLocalEvent())
		})
	}
}
