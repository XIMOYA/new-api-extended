// service/request_content_audit/store.go
// 请求内容审计存储编排：将正文和资源写入加密文件，并把可检索元数据提交到主数据库。
package request_content_audit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/google/uuid"
)

const (
	defaultMaxContentSize = 20 << 20
	defaultMaxAssetSize   = 10 << 20
)

var requestContentAuditWriteMu sync.Mutex

type StoreOption func(*Store) error

type Store struct {
	root           string
	temporaryRoot  string
	repository     *model.RequestContentAuditRepository
	keys           KeyProvider
	chunkSize      int
	maxContentSize int64
	maxAssetSize   int64
	now            func() time.Time
}

func WithChunkSize(chunkSize int) StoreOption {
	return func(store *Store) error {
		if chunkSize < minimumChunkSize || chunkSize > maximumChunkSize {
			return fmt.Errorf("request content audit chunk size must be between %d and %d bytes", minimumChunkSize, maximumChunkSize)
		}
		store.chunkSize = chunkSize
		return nil
	}
}

func WithMaxContentSize(maxContentSize int64) StoreOption {
	return func(store *Store) error {
		if maxContentSize <= 0 {
			return errors.New("request content audit maximum content size must be positive")
		}
		store.maxContentSize = maxContentSize
		return nil
	}
}

func WithMaxAssetSize(maxAssetSize int64) StoreOption {
	return func(store *Store) error {
		if maxAssetSize <= 0 {
			return errors.New("request content audit maximum asset size must be positive")
		}
		store.maxAssetSize = maxAssetSize
		return nil
	}
}

func WithClock(now func() time.Time) StoreOption {
	return func(store *Store) error {
		if now == nil {
			return errors.New("request content audit clock is nil")
		}
		store.now = now
		return nil
	}
}

func NewStore(root string, repository *model.RequestContentAuditRepository, keys KeyProvider, options ...StoreOption) (*Store, error) {
	if root == "" {
		return nil, errors.New("request content audit storage root is empty")
	}
	if repository == nil {
		return nil, errors.New("request content audit repository is nil")
	}
	if keys == nil {
		return nil, errors.New("request content audit key provider is nil")
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	store := &Store{
		root:           absoluteRoot,
		temporaryRoot:  filepath.Join(absoluteRoot, ".tmp"),
		repository:     repository,
		keys:           keys,
		chunkSize:      defaultChunkSize,
		maxContentSize: defaultMaxContentSize,
		maxAssetSize:   defaultMaxAssetSize,
		now:            time.Now,
	}
	for _, option := range options {
		if err := option(store); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (store *Store) Migrate(ctx context.Context) error {
	if err := store.ensureDirectories(); err != nil {
		return err
	}
	return store.repository.Migrate(ctx)
}

func (store *Store) ListAudits(ctx context.Context, filter model.RequestContentAuditListFilter) ([]model.RequestContentAudit, int64, error) {
	return store.repository.List(ctx, filter)
}

func (store *Store) GetAuditById(ctx context.Context, id int64) (*model.RequestContentAudit, error) {
	return store.repository.GetAuditById(ctx, id)
}

func (store *Store) GetAuditByRequestId(ctx context.Context, requestId string) (*model.RequestContentAudit, error) {
	return store.repository.GetAuditByRequestId(ctx, requestId)
}

func (store *Store) GetAssets(ctx context.Context, auditId int64) ([]model.RequestContentAsset, error) {
	return store.repository.GetAssets(ctx, auditId)
}

func (store *Store) GetAssetByKey(ctx context.Context, auditId int64, assetKey string) (*model.RequestContentAsset, error) {
	return store.repository.GetAssetByKey(ctx, auditId, assetKey)
}

func (store *Store) GetObjectById(ctx context.Context, id int64) (*model.RequestContentObject, error) {
	return store.repository.GetObjectById(ctx, id)
}

type RecordInput struct {
	Audit   model.RequestContentAudit
	Content io.Reader
	Assets  []AssetInput
}

type AssetInput struct {
	Position  int
	AssetKey  string
	FileName  string
	AssetType string
	MimeType  string
	Width     int
	Height    int
	Original  io.Reader
	Thumbnail *AssetVariantInput
}

type AssetVariantInput struct {
	MimeType string
	Width    int
	Height   int
	Content  io.Reader
}

func (store *Store) Store(ctx context.Context, input RecordInput) (*model.RequestContentAudit, error) {
	requestContentAuditWriteMu.Lock()
	defer requestContentAuditWriteMu.Unlock()
	if input.Content == nil {
		return nil, errors.New("request content audit content is nil")
	}
	if input.Audit.RequestId == "" {
		return nil, errors.New("request content audit request id is empty")
	}
	if err := store.ensureDirectories(); err != nil {
		return nil, err
	}
	if input.Audit.CreatedAt == 0 {
		input.Audit.CreatedAt = store.now().Unix()
	}
	if input.Audit.CaptureStatus == "" {
		input.Audit.CaptureStatus = model.RequestContentCaptureStatusComplete
	}
	input.Audit.IntegrityVersion = integrityVersion

	key, err := store.keys.ActiveKey(ctx)
	if err != nil {
		return nil, err
	}
	bodyTemporaryPath, bodyMetadata, err := writeEncryptedContainer(
		ctx,
		store.temporaryRoot,
		input.Content,
		key,
		store.chunkSize,
		store.maxContentSize,
	)
	if err != nil {
		return nil, err
	}
	bodyCommitted := false
	defer func() {
		if !bodyCommitted {
			_ = os.Remove(bodyTemporaryPath)
		}
	}()

	createdAt := time.Unix(input.Audit.CreatedAt, 0).UTC()
	bodyRelativePath := filepath.ToSlash(filepath.Join(
		"content",
		createdAt.Format("2006"),
		createdAt.Format("01"),
		uuid.NewString()+".rac",
	))
	bodyStoredSize, err := store.publishUnique(bodyTemporaryPath, bodyRelativePath, false)
	if err != nil {
		return nil, err
	}
	bodyCommitted = true
	input.Audit.ContentPath = bodyRelativePath
	input.Audit.ContentSha256 = bodyMetadata.Sha256
	input.Audit.ContentSize = bodyMetadata.PlainSize
	input.Audit.StoredSize = bodyStoredSize

	assets, err := store.stageAssets(ctx, input.Assets, key, input.Audit.CreatedAt, input.Audit.ExpiresAt)
	if err != nil {
		return nil, errors.Join(err, store.removeRelative(bodyRelativePath), store.cleanupStagedAssets(ctx, assets))
	}
	if err := store.repository.Create(ctx, &input.Audit, assets); err != nil {
		return nil, errors.Join(err, store.removeRelative(bodyRelativePath), store.cleanupStagedAssets(ctx, assets))
	}
	return &input.Audit, nil
}

func (store *Store) StoreJSON(ctx context.Context, input RecordInput, value any) (*model.RequestContentAudit, error) {
	data, err := common.Marshal(value)
	if err != nil {
		return nil, err
	}
	input.Content = bytes.NewReader(data)
	return store.Store(ctx, input)
}

func (store *Store) ReadAuditContent(ctx context.Context, audit *model.RequestContentAudit, destination io.Writer) error {
	if audit == nil {
		return errors.New("request content audit is nil")
	}
	if destination == nil {
		return errors.New("request content audit destination is nil")
	}
	return store.readRelative(ctx, audit.ContentPath, destination, fileMetadata{
		Sha256:     audit.ContentSha256,
		PlainSize:  audit.ContentSize,
		StoredSize: audit.StoredSize,
	})
}

func (store *Store) ReadAuditJSON(ctx context.Context, audit *model.RequestContentAudit, value any) error {
	var content bytes.Buffer
	if err := store.ReadAuditContent(ctx, audit, &content); err != nil {
		return err
	}
	return common.Unmarshal(content.Bytes(), value)
}

func (store *Store) ReadAsset(ctx context.Context, object *model.RequestContentObject, destination io.Writer) error {
	if object == nil {
		return errors.New("request content audit object is nil")
	}
	if destination == nil {
		return errors.New("request content audit destination is nil")
	}
	return store.readRelative(ctx, object.StoragePath, destination, fileMetadata{
		Sha256:     object.Sha256,
		PlainSize:  object.PlainSize,
		StoredSize: object.StoredSize,
	})
}

func (store *Store) OpenAuditContent(ctx context.Context, audit *model.RequestContentAudit) (io.ReadCloser, error) {
	if audit == nil {
		return nil, errors.New("request content audit is nil")
	}
	reader, writer := io.Pipe()
	go func() {
		err := store.ReadAuditContent(ctx, audit, writer)
		_ = writer.CloseWithError(err)
	}()
	return reader, nil
}

func (store *Store) OpenValidatedAuditContent(ctx context.Context, audit *model.RequestContentAudit) (io.ReadCloser, error) {
	if audit == nil {
		return nil, errors.New("request content audit is nil")
	}
	return store.openValidated(ctx, func(destination io.Writer) error {
		return store.ReadAuditContent(ctx, audit, destination)
	})
}

func (store *Store) OpenValidatedAsset(ctx context.Context, object *model.RequestContentObject) (io.ReadCloser, error) {
	if object == nil {
		return nil, errors.New("request content audit object is nil")
	}
	return store.openValidated(ctx, func(destination io.Writer) error {
		return store.ReadAsset(ctx, object, destination)
	})
}

type validatedReadCloser struct {
	file *os.File
	path string
	once sync.Once
	err  error
}

func (reader *validatedReadCloser) Read(data []byte) (int, error) {
	return reader.file.Read(data)
}

func (reader *validatedReadCloser) Close() error {
	reader.once.Do(func() {
		reader.err = errors.Join(reader.file.Close(), os.Remove(reader.path))
	})
	return reader.err
}

func (store *Store) openValidated(ctx context.Context, write func(io.Writer) error) (io.ReadCloser, error) {
	if err := store.ensureDirectories(); err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(store.temporaryRoot, "request-content-audit-read-*.tmp")
	if err != nil {
		return nil, err
	}
	path := file.Name()
	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(path)
	}
	if err := file.Chmod(0o600); err != nil {
		cleanup()
		return nil, err
	}
	if err := write(file); err != nil {
		cleanup()
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		cleanup()
		return nil, err
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return nil, err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	reader, err := os.Open(path)
	if err != nil {
		_ = os.Remove(path)
		return nil, err
	}
	return &validatedReadCloser{file: reader, path: path}, nil
}

func (store *Store) stageAssets(ctx context.Context, inputs []AssetInput, key EncryptionKey, createdAt int64, expiresAt int64) ([]model.RequestContentAssetCreate, error) {
	assets := make([]model.RequestContentAssetCreate, 0, len(inputs))
	positions := make(map[int]struct{}, len(inputs))
	for index := range inputs {
		input := &inputs[index]
		if input.Original == nil {
			return assets, fmt.Errorf("request content audit asset %d original content is nil", index)
		}
		if input.AssetType == "" {
			return assets, fmt.Errorf("request content audit asset %d type is empty", index)
		}
		if input.Position < 0 {
			return assets, fmt.Errorf("request content audit asset %d position is invalid", index)
		}
		if input.Width < 0 || input.Height < 0 {
			return assets, fmt.Errorf("request content audit asset %d dimensions are invalid", index)
		}
		if input.Thumbnail != nil && (input.Thumbnail.Width < 0 || input.Thumbnail.Height < 0) {
			return assets, fmt.Errorf("request content audit asset %d thumbnail dimensions are invalid", index)
		}
		if _, exists := positions[input.Position]; exists {
			return assets, fmt.Errorf("request content audit asset position %d is duplicated", input.Position)
		}
		positions[input.Position] = struct{}{}

		original, err := store.stageObject(ctx, key, model.RequestContentObjectKindOriginal, input.MimeType, input.Width, input.Height, input.Original)
		if err != nil {
			return assets, err
		}
		asset := model.RequestContentAsset{
			Position:  input.Position,
			AssetKey:  input.AssetKey,
			FileName:  input.FileName,
			AssetType: input.AssetType,
			MimeType:  input.MimeType,
			Width:     input.Width,
			Height:    input.Height,
			CreatedAt: createdAt,
			ExpiresAt: expiresAt,
		}
		item := model.RequestContentAssetCreate{Asset: asset, Original: original}
		assets = append(assets, item)
		assetIndex := len(assets) - 1
		if input.Thumbnail != nil {
			thumbnail, err := store.stageObject(
				ctx,
				key,
				model.RequestContentObjectKindThumbnail,
				input.Thumbnail.MimeType,
				input.Thumbnail.Width,
				input.Thumbnail.Height,
				input.Thumbnail.Content,
			)
			if err != nil {
				return assets, err
			}
			assets[assetIndex].Thumbnail = &thumbnail
		}
	}
	return assets, nil
}

func (store *Store) cleanupStagedAssets(ctx context.Context, assets []model.RequestContentAssetCreate) error {
	if len(assets) == 0 {
		return nil
	}
	referenced, err := store.repository.ReferencedPaths(ctx)
	if err != nil {
		return err
	}
	paths := make(map[string]struct{}, len(assets)*2)
	for index := range assets {
		if assets[index].Original.StoragePath != "" {
			paths[assets[index].Original.StoragePath] = struct{}{}
		}
		if assets[index].Thumbnail != nil && assets[index].Thumbnail.StoragePath != "" {
			paths[assets[index].Thumbnail.StoragePath] = struct{}{}
		}
	}
	var cleanupErr error
	for path := range paths {
		if _, exists := referenced[path]; exists {
			continue
		}
		cleanupErr = errors.Join(cleanupErr, store.removeRelative(path))
	}
	return cleanupErr
}

func (store *Store) stageObject(ctx context.Context, key EncryptionKey, objectKind string, mimeType string, width int, height int, content io.Reader) (model.RequestContentObject, error) {
	if content == nil {
		return model.RequestContentObject{}, errors.New("request content audit object content is nil")
	}
	temporaryPath, metadata, err := writeEncryptedContainer(
		ctx,
		store.temporaryRoot,
		content,
		key,
		store.chunkSize,
		store.maxAssetSize,
	)
	if err != nil {
		return model.RequestContentObject{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(temporaryPath)
		}
	}()
	relativePath := filepath.ToSlash(filepath.Join(
		"assets",
		objectKind,
		metadata.Sha256[:2],
		metadata.Sha256+".rac",
	))
	storedSize, err := store.publishUnique(temporaryPath, relativePath, true)
	if err != nil {
		return model.RequestContentObject{}, err
	}
	committed = true
	return model.RequestContentObject{
		ObjectKind:  objectKind,
		Sha256:      metadata.Sha256,
		StoragePath: relativePath,
		MimeType:    mimeType,
		PlainSize:   metadata.PlainSize,
		StoredSize:  storedSize,
		Width:       width,
		Height:      height,
		CreatedAt:   store.now().Unix(),
	}, nil
}

func (store *Store) readRelative(ctx context.Context, relativePath string, destination io.Writer, expected fileMetadata) error {
	path, err := store.resolve(relativePath)
	if err != nil {
		return err
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if expected.StoredSize > 0 {
		info, err := file.Stat()
		if err != nil {
			return err
		}
		if info.Size() != expected.StoredSize {
			return ErrIntegrityMismatch
		}
	}
	_, err = readEncryptedContainer(ctx, file, destination, store.keys, expected)
	return err
}

func (store *Store) publishUnique(temporaryPath string, relativePath string, allowExisting bool) (int64, error) {
	target, err := store.resolve(relativePath)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return 0, err
	}
	if err := os.Link(temporaryPath, target); err != nil {
		if errors.Is(err, fs.ErrExist) && allowExisting {
			entry, statErr := os.Lstat(target)
			if statErr != nil {
				return 0, statErr
			}
			if !entry.Mode().IsRegular() {
				return 0, errors.New("request content audit deduplicated path is not a regular file")
			}
			if removeErr := os.Remove(temporaryPath); removeErr != nil {
				return 0, removeErr
			}
			return entry.Size(), nil
		}
		return 0, err
	}
	if err := os.Remove(temporaryPath); err != nil {
		return 0, errors.Join(err, os.Remove(target))
	}
	if err := syncDirectory(filepath.Dir(target)); err != nil {
		return 0, errors.Join(err, os.Remove(target))
	}
	info, err := os.Stat(target)
	if err != nil {
		return 0, errors.Join(err, os.Remove(target))
	}
	return info.Size(), nil
}

func (store *Store) removeRelative(relativePath string) error {
	path, err := store.resolve(relativePath)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (store *Store) resolve(relativePath string) (string, error) {
	if relativePath == "" {
		return "", errors.New("request content audit storage path is empty")
	}
	cleanRelativePath := filepath.Clean(filepath.FromSlash(relativePath))
	if filepath.IsAbs(cleanRelativePath) || cleanRelativePath == "." || cleanRelativePath == ".." || strings.HasPrefix(cleanRelativePath, ".."+string(filepath.Separator)) {
		return "", errors.New("request content audit storage path escapes root")
	}
	path := filepath.Join(store.root, cleanRelativePath)
	relativeToRoot, err := filepath.Rel(store.root, path)
	if err != nil || relativeToRoot == ".." || strings.HasPrefix(relativeToRoot, ".."+string(filepath.Separator)) {
		return "", errors.New("request content audit storage path escapes root")
	}
	return path, nil
}

func (store *Store) ensureDirectories() error {
	for _, directory := range []string{
		store.root,
		store.temporaryRoot,
		filepath.Join(store.root, "content"),
		filepath.Join(store.root, "assets"),
	} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return err
		}
	}
	return nil
}
