// service/request_content_audit/manifest.go
// 去重正文的清单格式：正文文件里只存有序块引用，真正的内容按 sha256 存在共享 pack 里，
// 读取时按清单顺序拼回原始字节，因此对上层调用方完全透明。
package request_content_audit

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
)

const (
	// manifestIntegrityVersion 标记这条审计记录的正文是清单而不是完整明文。
	manifestIntegrityVersion = 2
	manifestMarker           = "narrafork-audit-manifest"
	manifestVersion          = 1
	// 超过这个大小就不走去重，直接按原来的整段写入，避免把大包全部读进内存。
	manifestMaxContentSize = 8 << 20
)

var ErrManifestUnsupported = errors.New("request content audit manifest storage is unavailable")

// readForManifest 先尝试把正文全部读进内存；超过上限时返回剩余流，让调用方退回整段写入。
func readForManifest(source io.Reader, limit int64) ([]byte, io.Reader, error) {
	if source == nil {
		return nil, nil, errors.New("request content audit content is nil")
	}
	buffered, err := io.ReadAll(io.LimitReader(source, limit+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(buffered)) > limit {
		return buffered, source, nil
	}
	return buffered, nil, nil
}

// storeDeduplicated 把正文切块写入共享 pack，正文文件只留清单。
func (store *Store) storeDeduplicated(ctx context.Context, input RecordInput, key EncryptionKey, data []byte) (*model.RequestContentAudit, error) {
	if int64(len(data)) > store.maxContentSize {
		return nil, ErrSizeLimitExceeded
	}
	refs, err := store.packs.PutItems(ctx, splitContentChunks(data), input.Audit.ExpiresAt)
	if err != nil {
		return nil, err
	}
	released := false
	defer func() {
		if !released {
			return
		}
		_ = store.packs.ReleaseItems(ctx, refs)
	}()

	manifest := buildContentManifest(data, refs)
	encoded, err := common.Marshal(manifest)
	if err != nil {
		released = true
		return nil, err
	}
	temporaryPath, metadata, err := writeEncryptedContainer(ctx, store.temporaryRoot, bytes.NewReader(encoded), key, store.chunkSize, int64(len(encoded)))
	if err != nil {
		released = true
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()

	createdAt := time.Unix(input.Audit.CreatedAt, 0).UTC()
	relativePath := filepath.ToSlash(filepath.Join(
		"content",
		createdAt.Format("2006"),
		createdAt.Format("01"),
		uuid.NewString()+".rac",
	))
	storedSize, err := store.publishUnique(temporaryPath, relativePath, false)
	if err != nil {
		released = true
		return nil, err
	}
	committed = true
	_ = metadata

	input.Audit.ContentPath = relativePath
	input.Audit.ContentSha256 = manifest.Sha256
	input.Audit.ContentSize = manifest.PlainSize
	input.Audit.StoredSize = storedSize
	input.Audit.IntegrityVersion = manifestIntegrityVersion

	assets, err := store.stageAssets(ctx, input.Assets, key, input.Audit.CreatedAt, input.Audit.ExpiresAt)
	if err != nil {
		released = true
		return nil, errors.Join(err, store.removeRelative(relativePath), store.cleanupStagedAssets(ctx, assets))
	}
	if err := store.repository.Create(ctx, &input.Audit, assets); err != nil {
		released = true
		return nil, errors.Join(err, store.removeRelative(relativePath), store.cleanupStagedAssets(ctx, assets))
	}
	return &input.Audit, nil
}

type contentManifest struct {
	Manifest  string    `json:"manifest"`
	Version   int       `json:"version"`
	PlainSize int64     `json:"plain_size"`
	Sha256    string    `json:"sha256"`
	Chunks    []BlobRef `json:"chunks"`
}

func buildContentManifest(data []byte, refs []BlobRef) contentManifest {
	digest := sha256.Sum256(data)
	return contentManifest{
		Manifest:  manifestMarker,
		Version:   manifestVersion,
		PlainSize: int64(len(data)),
		Sha256:    hex.EncodeToString(digest[:]),
		Chunks:    refs,
	}
}

func decodeContentManifest(data []byte) (contentManifest, error) {
	var manifest contentManifest
	if err := common.Unmarshal(data, &manifest); err != nil {
		return contentManifest{}, err
	}
	if manifest.Manifest != manifestMarker {
		return contentManifest{}, fmt.Errorf("request content audit manifest marker is invalid")
	}
	if manifest.Version != manifestVersion {
		return contentManifest{}, fmt.Errorf("unsupported request content audit manifest version %d", manifest.Version)
	}
	return manifest, nil
}

// readAuditManifest 读出清单本身（不展开内容），供清理流程收集块引用。
func (store *Store) readAuditManifest(ctx context.Context, audit *model.RequestContentAudit) (contentManifest, error) {
	if audit == nil || audit.ContentPath == "" {
		return contentManifest{}, errors.New("request content audit manifest path is empty")
	}
	var buffer bytes.Buffer
	// PlainSize 传 -1 表示不限制读取长度：清单自身的长度与摘要由容器尾记录校验。
	if err := store.readRelative(ctx, audit.ContentPath, &buffer, fileMetadata{
		PlainSize:  -1,
		StoredSize: audit.StoredSize,
	}); err != nil {
		return contentManifest{}, err
	}
	return decodeContentManifest(buffer.Bytes())
}

// readManifestContent 按清单顺序把块拼回明文，并校验总长度与 sha256。
func (store *Store) readManifestContent(ctx context.Context, audit *model.RequestContentAudit, destination io.Writer) error {
	if store.packs == nil {
		return ErrManifestUnsupported
	}
	manifest, err := store.readAuditManifest(ctx, audit)
	if err != nil {
		return err
	}
	chunks, err := store.packs.GetItems(ctx, manifest.Chunks)
	if err != nil {
		return err
	}

	digest := sha256.New()
	var total int64
	for _, chunk := range chunks {
		if err := ctx.Err(); err != nil {
			return err
		}
		if _, err := digest.Write(chunk); err != nil {
			return err
		}
		if _, err := destination.Write(chunk); err != nil {
			return err
		}
		total += int64(len(chunk))
	}
	if total != manifest.PlainSize || hex.EncodeToString(digest.Sum(nil)) != manifest.Sha256 {
		return ErrIntegrityMismatch
	}
	if audit.ContentSha256 != "" && audit.ContentSha256 != manifest.Sha256 {
		return ErrIntegrityMismatch
	}
	if audit.ContentSize > 0 && audit.ContentSize != manifest.PlainSize {
		return ErrIntegrityMismatch
	}
	return nil
}
