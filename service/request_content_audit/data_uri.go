// service/request_content_audit/data_uri.go
// Data URI 资源输入解析：仅接受 Base64 载荷，并交由加密流式存储层执行大小限制和哈希去重。
package request_content_audit

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
)

func AssetFromDataURI(position int, assetType string, dataURI string, width int, height int) (AssetInput, error) {
	mimeType, content, err := DecodeDataURI(dataURI)
	if err != nil {
		return AssetInput{}, err
	}
	return AssetInput{
		Position:  position,
		AssetType: assetType,
		MimeType:  mimeType,
		Width:     width,
		Height:    height,
		Original:  content,
	}, nil
}

func DecodeDataURI(dataURI string) (string, io.Reader, error) {
	metadata, encodedContent, found := strings.Cut(dataURI, ",")
	if !found || !strings.HasPrefix(strings.ToLower(metadata), "data:") {
		return "", nil, errors.New("request content audit data uri is invalid")
	}
	parts := strings.Split(metadata[len("data:"):], ";")
	if len(parts) < 2 || parts[0] == "" {
		return "", nil, errors.New("request content audit data uri mime type is missing")
	}
	isBase64 := false
	for _, part := range parts[1:] {
		if strings.EqualFold(part, "base64") {
			isBase64 = true
			continue
		}
		if part != "" {
			return "", nil, fmt.Errorf("request content audit data uri parameter %q is unsupported", part)
		}
	}
	if !isBase64 {
		return "", nil, errors.New("request content audit data uri must use base64 encoding")
	}
	return strings.ToLower(parts[0]), base64.NewDecoder(base64.StdEncoding, strings.NewReader(encodedContent)), nil
}
