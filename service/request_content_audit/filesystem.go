// service/request_content_audit/filesystem.go
// 文件系统提交辅助：在支持目录同步的平台上同步目录项，保证已发布审计文件的持久性边界。
package request_content_audit

import (
	"os"
	"runtime"
)

func syncDirectory(directory string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	file, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer file.Close()
	return file.Sync()
}
