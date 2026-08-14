// service/request_content_audit_capture_test.go
// 验证请求内容规范化不会把图片 Base64 留在正文中，并会生成可访问的原件与缩略图。
package service

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBuildRequestContentEnvelopeExtractsBareBase64Fields(t *testing.T) {
	imageData := []byte("bare-base64-image")
	encoded := base64.StdEncoding.EncodeToString(imageData)
	body := []byte(`{"inline_data":{"mime_type":"image/png","data":"` + encoded + `"},"b64_json":"` + encoded + `"}`)

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("POST", "/v1/gemini", bytes.NewReader(body))

	envelope, assets, err := buildRequestContentEnvelope(context, "req-bare-b64", types.RelayFormatGemini, "application/json", body, 8<<20)
	require.NoError(t, err)
	require.Len(t, assets, 2)
	encodedEnvelope, err := common.Marshal(envelope)
	require.NoError(t, err)
	require.NotContains(t, string(encodedEnvelope), encoded)
	for _, asset := range assets {
		original, readErr := io.ReadAll(asset.Original)
		require.NoError(t, readErr)
		require.Equal(t, imageData, original)
	}
}

func TestBuildMultipartEnvelopeExtractsJSONFieldBase64(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("multipart-image"))
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("payload", `{"inline_data":{"mime_type":"image/png","data":"`+encoded+`"}}`))
	require.NoError(t, writer.Close())

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("POST", "/v1/upload", &body)
	context.Request.Header.Set("Content-Type", writer.FormDataContentType())

	envelope, assets, err := buildRequestContentEnvelope(context, "req-multipart-b64", types.RelayFormatOpenAI, writer.FormDataContentType(), body.Bytes(), 8<<20)
	require.NoError(t, err)
	require.Len(t, assets, 1)
	encodedEnvelope, err := common.Marshal(envelope)
	require.NoError(t, err)
	require.NotContains(t, string(encodedEnvelope), encoded)
}

func TestBuildRequestContentEnvelopeExtractsBase64Image(t *testing.T) {
	var imageData bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 8, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{R: 32, G: 128, B: 240, A: 255})
		}
	}
	require.NoError(t, png.Encode(&imageData, img))
	dataURI := "data:image/png;base64," + base64.StdEncoding.EncodeToString(imageData.Bytes())
	body := []byte(`{"model":"vision-model","messages":[{"role":"user","content":[{"type":"text","text":"inspect"},{"type":"image_url","image_url":{"url":"` + dataURI + `"}}]}]}`)

	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(body))

	envelope, assets, err := buildRequestContentEnvelope(context, "req-1", types.RelayFormatOpenAI, "application/json", body, 8<<20)
	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.NotNil(t, assets[0].Thumbnail)
	require.Equal(t, "asset-0", assets[0].AssetKey)
	require.Equal(t, 8, assets[0].Width)
	require.Equal(t, 4, assets[0].Height)

	encodedEnvelope, err := common.Marshal(envelope)
	require.NoError(t, err)
	require.NotContains(t, string(encodedEnvelope), dataURI)
	require.NotContains(t, string(encodedEnvelope), strings.TrimPrefix(dataURI, "data:image/png;base64,"))
	require.Contains(t, string(encodedEnvelope), "asset-0")

	original, err := io.ReadAll(assets[0].Original)
	require.NoError(t, err)
	require.Equal(t, imageData.Bytes(), original)
	thumbnail, err := io.ReadAll(assets[0].Thumbnail.Content)
	require.NoError(t, err)
	require.NotEmpty(t, thumbnail)
}
