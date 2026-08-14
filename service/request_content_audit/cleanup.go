// service/request_content_audit/cleanup.go
// 审计存储的保留期清理与孤儿文件回收，数据库删除和文件删除可分别重试。
package request_content_audit

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type CleanupReport struct {
	AuditCount       int64
	AssetCount       int64
	ObjectCount      int64
	RemovedFileCount int64
}

func (store *Store) CleanupExpired(ctx context.Context, cutoff time.Time, batchSize int) (CleanupReport, error) {
	requestContentAuditWriteMu.Lock()
	defer requestContentAuditWriteMu.Unlock()
	deleted, err := store.repository.DeleteExpiredBatch(ctx, cutoff.Unix(), batchSize)
	if err != nil {
		return CleanupReport{}, err
	}
	report := CleanupReport{
		AuditCount:  deleted.AuditCount,
		AssetCount:  deleted.AssetCount,
		ObjectCount: deleted.ObjectCount,
	}
	paths := make(map[string]struct{}, len(deleted.Paths))
	for _, path := range deleted.Paths {
		paths[path] = struct{}{}
	}
	referenced, err := store.repository.ReferencedPaths(ctx)
	if err != nil {
		return report, err
	}

	var cleanupErrors []error
	for path := range paths {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if _, stillReferenced := referenced[path]; stillReferenced {
			continue
		}
		if err := store.removeRelative(path); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("remove request content audit file %q: %w", path, err))
			continue
		}
		report.RemovedFileCount++
	}
	return report, errors.Join(cleanupErrors...)
}

func (store *Store) CleanupOrphans(ctx context.Context, olderThan time.Time) (int64, error) {
	requestContentAuditWriteMu.Lock()
	defer requestContentAuditWriteMu.Unlock()
	if err := store.ensureDirectories(); err != nil {
		return 0, err
	}
	referenced, err := store.repository.ReferencedPaths(ctx)
	if err != nil {
		return 0, err
	}
	var removed int64
	for _, directory := range []string{filepath.Join(store.root, "content"), filepath.Join(store.root, "assets")} {
		err := filepath.WalkDir(directory, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(path) != ".rac" {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if !olderThan.IsZero() && !info.ModTime().Before(olderThan) {
				return nil
			}
			relativePath, err := filepath.Rel(store.root, path)
			if err != nil {
				return err
			}
			if _, exists := referenced[filepath.ToSlash(relativePath)]; exists {
				return nil
			}
			if err := os.Remove(path); err != nil {
				return err
			}
			removed++
			return nil
		})
		if err != nil {
			return removed, err
		}
	}
	return removed, nil
}

func (store *Store) CleanupTemporaryFiles(ctx context.Context, olderThan time.Time) (int64, error) {
	requestContentAuditWriteMu.Lock()
	defer requestContentAuditWriteMu.Unlock()
	if err := store.ensureDirectories(); err != nil {
		return 0, err
	}
	var removed int64
	err := filepath.WalkDir(store.temporaryRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasSuffix(entry.Name(), ".tmp") {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !olderThan.IsZero() && !info.ModTime().Before(olderThan) {
			return nil
		}
		if err := os.Remove(path); err != nil {
			return err
		}
		removed++
		return nil
	})
	return removed, err
}
