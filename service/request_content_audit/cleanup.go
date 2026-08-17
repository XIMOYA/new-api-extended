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
	BlobCount        int64
	PackCount        int64
}

func (store *Store) CleanupExpired(ctx context.Context, cutoff time.Time, batchSize int) (CleanupReport, error) {
	requestContentAuditWriteMu.Lock()
	defer requestContentAuditWriteMu.Unlock()

	// 去重记录的正文只是清单，必须在删除前把块引用读出来，否则共享块永远减不掉引用。
	releasing, manifestErrors := store.collectExpiringManifestRefs(ctx, cutoff, batchSize)

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
	cleanupErrors = append(cleanupErrors, manifestErrors...)

	if store.packs != nil && len(releasing) > 0 {
		if err := store.packs.ReleaseItems(ctx, releasing); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("release request content audit blobs: %w", err))
		}
	}
	if store.packs != nil {
		blobReport, err := store.packs.CleanupBlobs(ctx, cutoff, batchSize)
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("cleanup request content audit blobs: %w", err))
		}
		report.BlobCount = blobReport.BlobCount
		report.PackCount = blobReport.PackCount
		if len(blobReport.Paths) > 0 {
			removed, err := store.packs.RemovePackFiles(blobReport.Paths)
			if err != nil {
				cleanupErrors = append(cleanupErrors, fmt.Errorf("remove request content audit pack files: %w", err))
			}
			report.RemovedFileCount += removed
		}
	}
	return report, errors.Join(cleanupErrors...)
}

// collectExpiringManifestRefs 读取即将过期记录的清单，返回其中引用的内容块。
func (store *Store) collectExpiringManifestRefs(ctx context.Context, cutoff time.Time, batchSize int) ([]BlobRef, []error) {
	if store.packs == nil {
		return nil, nil
	}
	audits, err := store.repository.ExpiringAudits(ctx, cutoff.Unix(), batchSize)
	if err != nil {
		return nil, []error{fmt.Errorf("list expiring request content audits: %w", err)}
	}
	var refs []BlobRef
	var failures []error
	for index := range audits {
		audit := &audits[index]
		if audit.IntegrityVersion < manifestIntegrityVersion || audit.ContentPath == "" {
			continue
		}
		manifest, err := store.readAuditManifest(ctx, audit)
		if err != nil {
			// 清单损坏时只记录，不阻塞记录删除；残留块会由过期扫描兜底回收。
			failures = append(failures, fmt.Errorf("read request content audit manifest %d: %w", audit.Id, err))
			continue
		}
		refs = append(refs, manifest.Chunks...)
	}
	return refs, failures
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
	packRemoved, err := store.cleanupOrphanPacks(ctx, olderThan)
	removed += packRemoved
	return removed, err
}

// cleanupOrphanPacks 清掉 packs 目录里没有数据库记录指向的文件，兜住"记录已删、文件删除失败"的残留。
func (store *Store) cleanupOrphanPacks(ctx context.Context, olderThan time.Time) (int64, error) {
	if store.packs == nil {
		return 0, nil
	}
	referenced, err := store.packs.ReferencedPackPaths(ctx)
	if err != nil {
		return 0, err
	}
	packRoot := filepath.Join(store.root, "packs")
	if _, err := os.Stat(packRoot); err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	var removed int64
	err = filepath.WalkDir(packRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || filepath.Ext(path) != ".rap" {
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
	return removed, err
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
