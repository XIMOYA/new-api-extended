// controller/narrafork.go
// NarraFork 2.0 策略管理、实时预览和一次性 SSE 测试接口。
package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/narrafork_setting"
	"github.com/gin-gonic/gin"
)

type narraForkPolicyResponse struct {
	ScopeType string                                  `json:"scope_type"`
	ScopeKey  string                                  `json:"scope_key"`
	Config    *narrafork_setting.NarraForkPolicyPatch `json:"config"`
	UpdatedAt int64                                   `json:"updated_at"`
}

type narraForkPolicyUpdateRequest struct {
	ScopeType string                                 `json:"scope_type"`
	ScopeKey  string                                 `json:"scope_key"`
	Config    narrafork_setting.NarraForkPolicyPatch `json:"config"`
}

type narraForkPreviewRequest struct {
	Config narrafork_setting.NarraForkPolicyPatch `json:"config"`
}

func GetNarraForkPolicies(c *gin.Context) {
	scopeType := strings.TrimSpace(c.Query("scope_type"))
	if scopeType != "" {
		if _, err := model.ListNarraForkQuotaEventPolicies(scopeType); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
			return
		}
	}
	scopeTypes := []string{model.NarraForkPolicyScopeGroup, model.NarraForkPolicyScopeUser}
	if scopeType != "" {
		scopeTypes = []string{scopeType}
	}
	responses := make([]narraForkPolicyResponse, 0)
	for _, currentScopeType := range scopeTypes {
		rows, err := model.ListNarraForkQuotaEventPolicies(currentScopeType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
			return
		}
		for _, row := range rows {
			patch, err := model.DecodeNarraForkPolicyPatch(row.Config)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "message": err.Error()})
				return
			}
			responses = append(responses, narraForkPolicyResponse{
				ScopeType: row.ScopeType,
				ScopeKey:  row.ScopeKey,
				Config:    patch,
				UpdatedAt: row.UpdatedAt,
			})
		}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": responses})
}

func UpdateNarraForkPolicy(c *gin.Context) {
	var request narraForkPolicyUpdateRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 NarraFork 策略参数"})
		return
	}
	if err := model.UpsertNarraForkQuotaEventPolicy(request.ScopeType, request.ScopeKey, request.Config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	service.InvalidateNarraForkQuotaPolicyCache(request.ScopeType, request.ScopeKey)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{
		"scope_type": strings.TrimSpace(request.ScopeType),
		"scope_key":  strings.TrimSpace(request.ScopeKey),
		"config":     request.Config,
	}})
}

func DeleteNarraForkPolicy(c *gin.Context) {
	scopeType := c.Param("scope_type")
	scopeKey := c.Param("scope_key")
	if err := model.DeleteNarraForkQuotaEventPolicy(scopeType, scopeKey); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	service.InvalidateNarraForkQuotaPolicyCache(scopeType, scopeKey)
	c.JSON(http.StatusOK, gin.H{"success": true, "message": ""})
}

func buildNarraForkPreviewConfig(patch narrafork_setting.NarraForkPolicyPatch) (relaycommon.NarraForkQuotaEventConfig, error) {
	if err := patch.Validate(); err != nil {
		return relaycommon.NarraForkQuotaEventConfig{}, err
	}
	config := relaycommon.NewNarraForkQuotaEventConfig(narrafork_setting.GetSettings())
	relaycommon.ApplyNarraForkPolicyPatch(&config, &patch)
	return config, nil
}

func PreviewNarraForkEvent(c *gin.Context) {
	var request narraForkPreviewRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 NarraFork 预览参数"})
		return
	}
	config, err := buildNarraForkPreviewConfig(request.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	preview, err := service.BuildNarraForkQuotaEventPreview(config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": preview})
}

func TestNarraForkEvent(c *gin.Context) {
	var request narraForkPreviewRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "无效的 NarraFork 测试参数"})
		return
	}
	config, err := buildNarraForkPreviewConfig(request.Config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	preview, err := service.BuildNarraForkQuotaEventPreview(config)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": err.Error()})
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	if _, err := c.Writer.Write([]byte(preview.SSE)); err != nil {
		return
	}
	if flusher, ok := c.Writer.(http.Flusher); ok {
		flusher.Flush()
	}
}
