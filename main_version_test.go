// main_version_test.go
// 应用版本：验证链接注入、嵌入 VERSION 文件与默认版本之间的回退优先级。
package main

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestResolveBuildVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		linkedVersion   string
		embeddedVersion string
		want            string
	}{
		{
			name:            "uses embedded VERSION when linker version is default",
			linkedVersion:   common.DefaultVersion,
			embeddedVersion: " v1.0.0-rc.24-ximoya.1\n",
			want:            "v1.0.0-rc.24-ximoya.1",
		},
		{
			name:            "preserves linker injected version",
			linkedVersion:   "v1.0.0-rc.24-ximoya.2",
			embeddedVersion: "v1.0.0-rc.24-ximoya.1",
			want:            "v1.0.0-rc.24-ximoya.2",
		},
		{
			name:            "falls back to default when no build version is available",
			linkedVersion:   "",
			embeddedVersion: " \n\t",
			want:            common.DefaultVersion,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			actual := resolveBuildVersion(testCase.linkedVersion, testCase.embeddedVersion)

			require.Equal(t, testCase.want, actual)
		})
	}
}
