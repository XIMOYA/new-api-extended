// service/request_content_audit/crypto_container.go
// 审计文件容器：先以 zstd 流式压缩，再使用 AES-GCM 分块加密，并在尾记录中校验原文大小与 SHA-256。
package request_content_audit

import (
	"bufio"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"os"

	"github.com/klauspost/compress/zstd"
)

const (
	containerVersion       = uint16(1)
	integrityVersion       = 1
	defaultChunkSize       = 64 << 10
	minimumChunkSize       = 4 << 10
	maximumChunkSize       = 16 << 20
	maximumKeyIdLength     = 1024
	containerRecordData    = byte(1)
	containerRecordTrailer = byte(2)
	trailerPlainSize       = 8 + sha256.Size
)

var (
	containerMagic       = [8]byte{'N', 'A', 'R', 'A', 'U', 'D', '0', '1'}
	ErrIntegrityMismatch = errors.New("request content audit integrity check failed")
	ErrSizeLimitExceeded = errors.New("request content audit size limit exceeded")
)

type EncryptionKey struct {
	Id    string
	Value []byte
}

type KeyProvider interface {
	ActiveKey(ctx context.Context) (EncryptionKey, error)
	Key(ctx context.Context, keyId string) ([]byte, error)
}

type StaticKeyProvider struct {
	ActiveKeyId string
	Keys        map[string][]byte
}

func (provider *StaticKeyProvider) ActiveKey(_ context.Context) (EncryptionKey, error) {
	if provider == nil {
		return EncryptionKey{}, errors.New("request content audit key provider is nil")
	}
	key, ok := provider.Keys[provider.ActiveKeyId]
	if !ok {
		return EncryptionKey{}, fmt.Errorf("request content audit active key %q not found", provider.ActiveKeyId)
	}
	return EncryptionKey{Id: provider.ActiveKeyId, Value: append([]byte(nil), key...)}, nil
}

func (provider *StaticKeyProvider) Key(_ context.Context, keyId string) ([]byte, error) {
	if provider == nil {
		return nil, errors.New("request content audit key provider is nil")
	}
	key, ok := provider.Keys[keyId]
	if !ok {
		return nil, fmt.Errorf("request content audit key %q not found", keyId)
	}
	return append([]byte(nil), key...), nil
}

type fileMetadata struct {
	Sha256     string
	PlainSize  int64
	StoredSize int64
}

type encryptedContainerWriter struct {
	writer      io.Writer
	aead        cipher.AEAD
	header      []byte
	noncePrefix [8]byte
	chunkSize   int
	chunkIndex  uint32
	pending     []byte
}

func writeEncryptedContainer(ctx context.Context, tempDirectory string, source io.Reader, key EncryptionKey, chunkSize int, maxPlainSize int64) (string, fileMetadata, error) {
	if source == nil {
		return "", fileMetadata{}, errors.New("request content audit source is nil")
	}
	if len(key.Id) == 0 || len(key.Id) > maximumKeyIdLength {
		return "", fileMetadata{}, errors.New("request content audit key id length is invalid")
	}
	if chunkSize < minimumChunkSize || chunkSize > maximumChunkSize {
		return "", fileMetadata{}, fmt.Errorf("request content audit chunk size must be between %d and %d bytes", minimumChunkSize, maximumChunkSize)
	}
	block, err := aes.NewCipher(key.Value)
	if err != nil {
		return "", fileMetadata{}, fmt.Errorf("create request content audit cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fileMetadata{}, fmt.Errorf("create request content audit gcm: %w", err)
	}

	file, err := os.CreateTemp(tempDirectory, "request-content-audit-*.tmp")
	if err != nil {
		return "", fileMetadata{}, err
	}
	tempPath := file.Name()
	committed := false
	defer func() {
		_ = file.Close()
		if !committed {
			_ = os.Remove(tempPath)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		return "", fileMetadata{}, err
	}

	header, noncePrefix, err := writeContainerHeader(file, key.Id, chunkSize)
	if err != nil {
		return "", fileMetadata{}, err
	}
	containerWriter := &encryptedContainerWriter{
		writer:      file,
		aead:        aead,
		header:      header,
		noncePrefix: noncePrefix,
		chunkSize:   chunkSize,
		pending:     make([]byte, 0, chunkSize),
	}
	encoder, err := zstd.NewWriter(containerWriter,
		zstd.WithEncoderConcurrency(1),
		zstd.WithEncoderCRC(true),
	)
	if err != nil {
		return "", fileMetadata{}, err
	}

	plainHash := sha256.New()
	plainSize, copyErr := copyPlaintext(ctx, encoder, source, plainHash, maxPlainSize)
	closeErr := encoder.Close()
	if copyErr != nil {
		return "", fileMetadata{}, copyErr
	}
	if closeErr != nil {
		return "", fileMetadata{}, closeErr
	}
	if err := containerWriter.flushData(); err != nil {
		return "", fileMetadata{}, err
	}
	if err := containerWriter.writeTrailer(plainSize, plainHash.Sum(nil)); err != nil {
		return "", fileMetadata{}, err
	}
	if err := file.Sync(); err != nil {
		return "", fileMetadata{}, err
	}
	if err := file.Close(); err != nil {
		return "", fileMetadata{}, err
	}
	info, err := os.Stat(tempPath)
	if err != nil {
		return "", fileMetadata{}, err
	}
	committed = true
	return tempPath, fileMetadata{
		Sha256:     fmt.Sprintf("%x", plainHash.Sum(nil)),
		PlainSize:  plainSize,
		StoredSize: info.Size(),
	}, nil
}

func writeContainerHeader(writer io.Writer, keyId string, chunkSize int) ([]byte, [8]byte, error) {
	var noncePrefix [8]byte
	if _, err := io.ReadFull(rand.Reader, noncePrefix[:]); err != nil {
		return nil, noncePrefix, err
	}

	var header bytes.Buffer
	header.Write(containerMagic[:])
	if err := binary.Write(&header, binary.BigEndian, containerVersion); err != nil {
		return nil, noncePrefix, err
	}
	if err := binary.Write(&header, binary.BigEndian, uint32(chunkSize)); err != nil {
		return nil, noncePrefix, err
	}
	header.Write(noncePrefix[:])
	if err := binary.Write(&header, binary.BigEndian, uint16(len(keyId))); err != nil {
		return nil, noncePrefix, err
	}
	header.WriteString(keyId)
	if _, err := writer.Write(header.Bytes()); err != nil {
		return nil, noncePrefix, err
	}
	return append([]byte(nil), header.Bytes()...), noncePrefix, nil
}

func copyPlaintext(ctx context.Context, destination io.Writer, source io.Reader, digest io.Writer, maxPlainSize int64) (int64, error) {
	buffer := make([]byte, 64<<10)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		readCount, readErr := source.Read(buffer)
		if readCount > 0 {
			if maxPlainSize >= 0 && total+int64(readCount) > maxPlainSize {
				return total, ErrSizeLimitExceeded
			}
			if digest != nil {
				if _, err := digest.Write(buffer[:readCount]); err != nil {
					return total, err
				}
			}
			if _, err := destination.Write(buffer[:readCount]); err != nil {
				return total, err
			}
			total += int64(readCount)
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return total, nil
			}
			return total, readErr
		}
		if readCount == 0 {
			return total, io.ErrNoProgress
		}
	}
}

func (writer *encryptedContainerWriter) Write(data []byte) (int, error) {
	written := len(data)
	for len(data) > 0 {
		remaining := writer.chunkSize - len(writer.pending)
		if remaining > len(data) {
			remaining = len(data)
		}
		writer.pending = append(writer.pending, data[:remaining]...)
		data = data[remaining:]
		if len(writer.pending) == writer.chunkSize {
			if err := writer.writeRecord(containerRecordData, writer.pending); err != nil {
				return 0, err
			}
			writer.pending = writer.pending[:0]
		}
	}
	return written, nil
}

func (writer *encryptedContainerWriter) flushData() error {
	if len(writer.pending) == 0 {
		return nil
	}
	if err := writer.writeRecord(containerRecordData, writer.pending); err != nil {
		return err
	}
	writer.pending = writer.pending[:0]
	return nil
}

func (writer *encryptedContainerWriter) writeTrailer(plainSize int64, digest []byte) error {
	if len(digest) != sha256.Size {
		return errors.New("request content audit trailer digest length is invalid")
	}
	trailer := make([]byte, trailerPlainSize)
	binary.BigEndian.PutUint64(trailer[:8], uint64(plainSize))
	copy(trailer[8:], digest)
	return writer.writeRecord(containerRecordTrailer, trailer)
}

func (writer *encryptedContainerWriter) writeRecord(recordType byte, plaintext []byte) error {
	if writer.chunkIndex == ^uint32(0) {
		return errors.New("request content audit file has too many encrypted chunks")
	}
	nonce := makeRecordNonce(writer.noncePrefix, writer.chunkIndex)
	aad := makeRecordAdditionalData(writer.header, recordType, writer.chunkIndex)
	sealed := writer.aead.Seal(nil, nonce, plaintext, aad)
	if err := binary.Write(writer.writer, binary.BigEndian, recordType); err != nil {
		return err
	}
	if err := binary.Write(writer.writer, binary.BigEndian, uint32(len(sealed))); err != nil {
		return err
	}
	if _, err := writer.writer.Write(sealed); err != nil {
		return err
	}
	writer.chunkIndex++
	return nil
}

type encryptedPayloadReader struct {
	reader       *bufio.Reader
	aead         cipher.AEAD
	header       []byte
	noncePrefix  [8]byte
	chunkSize    int
	chunkIndex   uint32
	pending      []byte
	trailerSeen  bool
	expectedSize int64
	expectedHash [sha256.Size]byte
}

func readEncryptedContainer(ctx context.Context, source io.Reader, destination io.Writer, keys KeyProvider, expected fileMetadata) (fileMetadata, error) {
	if keys == nil {
		return fileMetadata{}, errors.New("request content audit key provider is nil")
	}
	buffered := bufio.NewReader(source)
	header, keyId, chunkSize, noncePrefix, err := readContainerHeader(buffered)
	if err != nil {
		return fileMetadata{}, err
	}
	key, err := keys.Key(ctx, keyId)
	if err != nil {
		return fileMetadata{}, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return fileMetadata{}, fmt.Errorf("create request content audit cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return fileMetadata{}, fmt.Errorf("create request content audit gcm: %w", err)
	}
	payload := &encryptedPayloadReader{
		reader:      buffered,
		aead:        aead,
		header:      header,
		noncePrefix: noncePrefix,
		chunkSize:   chunkSize,
	}
	decoder, err := zstd.NewReader(payload, zstd.WithDecoderConcurrency(1))
	if err != nil {
		return fileMetadata{}, err
	}
	defer decoder.Close()

	digest := sha256.New()
	plainSize, err := copyPlaintext(ctx, io.MultiWriter(destination, digest), decoder, nil, expected.PlainSize)
	if err != nil {
		if errors.Is(err, ErrSizeLimitExceeded) {
			return fileMetadata{}, ErrIntegrityMismatch
		}
		return fileMetadata{}, err
	}
	if err := payload.finish(); err != nil {
		return fileMetadata{}, err
	}
	actualHash := digest.Sum(nil)
	if plainSize != payload.expectedSize || !bytes.Equal(actualHash, payload.expectedHash[:]) {
		return fileMetadata{}, ErrIntegrityMismatch
	}
	actual := fileMetadata{Sha256: fmt.Sprintf("%x", actualHash), PlainSize: plainSize}
	if expected.PlainSize > 0 && expected.PlainSize != actual.PlainSize {
		return fileMetadata{}, ErrIntegrityMismatch
	}
	if expected.Sha256 != "" && expected.Sha256 != actual.Sha256 {
		return fileMetadata{}, ErrIntegrityMismatch
	}
	return actual, nil
}

func readContainerHeader(reader io.Reader) ([]byte, string, int, [8]byte, error) {
	var noncePrefix [8]byte
	fixed := make([]byte, len(containerMagic)+2+4+len(noncePrefix)+2)
	if _, err := io.ReadFull(reader, fixed); err != nil {
		return nil, "", 0, noncePrefix, err
	}
	if !bytes.Equal(fixed[:len(containerMagic)], containerMagic[:]) {
		return nil, "", 0, noncePrefix, errors.New("request content audit file magic is invalid")
	}
	offset := len(containerMagic)
	version := binary.BigEndian.Uint16(fixed[offset : offset+2])
	if version != containerVersion {
		return nil, "", 0, noncePrefix, fmt.Errorf("unsupported request content audit file version %d", version)
	}
	offset += 2
	chunkSize := int(binary.BigEndian.Uint32(fixed[offset : offset+4]))
	if chunkSize < minimumChunkSize || chunkSize > maximumChunkSize {
		return nil, "", 0, noncePrefix, errors.New("request content audit file chunk size is invalid")
	}
	offset += 4
	copy(noncePrefix[:], fixed[offset:offset+len(noncePrefix)])
	offset += len(noncePrefix)
	keyIdLength := int(binary.BigEndian.Uint16(fixed[offset : offset+2]))
	if keyIdLength <= 0 || keyIdLength > maximumKeyIdLength {
		return nil, "", 0, noncePrefix, errors.New("request content audit file key id length is invalid")
	}
	keyIdBytes := make([]byte, keyIdLength)
	if _, err := io.ReadFull(reader, keyIdBytes); err != nil {
		return nil, "", 0, noncePrefix, err
	}
	header := append(append([]byte(nil), fixed...), keyIdBytes...)
	return header, string(keyIdBytes), chunkSize, noncePrefix, nil
}

func (reader *encryptedPayloadReader) Read(destination []byte) (int, error) {
	for len(reader.pending) == 0 {
		if reader.trailerSeen {
			return 0, io.EOF
		}
		if err := reader.readRecord(); err != nil {
			return 0, err
		}
	}
	count := copy(destination, reader.pending)
	reader.pending = reader.pending[count:]
	return count, nil
}

func (reader *encryptedPayloadReader) readRecord() error {
	recordType, err := reader.reader.ReadByte()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return io.ErrUnexpectedEOF
		}
		return err
	}
	var sealedSize uint32
	if err := binary.Read(reader.reader, binary.BigEndian, &sealedSize); err != nil {
		return err
	}
	maximumSealedSize := reader.chunkSize + reader.aead.Overhead()
	if recordType == containerRecordTrailer {
		maximumSealedSize = trailerPlainSize + reader.aead.Overhead()
	}
	if sealedSize < uint32(reader.aead.Overhead()) || uint64(sealedSize) > uint64(maximumSealedSize) {
		return errors.New("request content audit encrypted record size is invalid")
	}
	sealed := make([]byte, int(sealedSize))
	if _, err := io.ReadFull(reader.reader, sealed); err != nil {
		return err
	}
	nonce := makeRecordNonce(reader.noncePrefix, reader.chunkIndex)
	aad := makeRecordAdditionalData(reader.header, recordType, reader.chunkIndex)
	plaintext, err := reader.aead.Open(nil, nonce, sealed, aad)
	if err != nil {
		return fmt.Errorf("decrypt request content audit record: %w", ErrIntegrityMismatch)
	}
	reader.chunkIndex++

	switch recordType {
	case containerRecordData:
		if reader.trailerSeen || len(plaintext) == 0 {
			return errors.New("request content audit data record is invalid")
		}
		reader.pending = plaintext
		return nil
	case containerRecordTrailer:
		if len(plaintext) != trailerPlainSize {
			return errors.New("request content audit trailer is invalid")
		}
		plainSize := binary.BigEndian.Uint64(plaintext[:8])
		if plainSize > math.MaxInt64 {
			return ErrIntegrityMismatch
		}
		reader.expectedSize = int64(plainSize)
		copy(reader.expectedHash[:], plaintext[8:])
		reader.trailerSeen = true
		if _, err := reader.reader.Peek(1); err == nil {
			return errors.New("request content audit file has trailing data")
		} else if !errors.Is(err, io.EOF) {
			return err
		}
		return nil
	default:
		return errors.New("request content audit record type is invalid")
	}
}

func (reader *encryptedPayloadReader) finish() error {
	if len(reader.pending) != 0 {
		return errors.New("request content audit compressed payload was not fully consumed")
	}
	for !reader.trailerSeen {
		if err := reader.readRecord(); err != nil {
			return err
		}
		if len(reader.pending) != 0 {
			return errors.New("request content audit file contains trailing compressed data")
		}
	}
	return nil
}

func makeRecordNonce(prefix [8]byte, chunkIndex uint32) []byte {
	nonce := make([]byte, 12)
	copy(nonce, prefix[:])
	binary.BigEndian.PutUint32(nonce[8:], chunkIndex)
	return nonce
}

func makeRecordAdditionalData(header []byte, recordType byte, chunkIndex uint32) []byte {
	aad := make([]byte, 0, len(header)+5)
	aad = append(aad, header...)
	aad = append(aad, recordType)
	index := make([]byte, 4)
	binary.BigEndian.PutUint32(index, chunkIndex)
	return append(aad, index...)
}
