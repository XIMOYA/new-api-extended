// service/request_content_audit_task.go
// 请求内容审计清理任务：按保留期批量删除正文、资源和异常临时文件。
package service

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/model"
)

const requestContentAuditCleanupInterval = 24 * time.Hour

const requestContentAuditCleanupBatchSize = 100

type RequestContentAuditCleanupPayload struct {
	RetentionDays    int   `json:"retention_days"`
	BatchSize        int   `json:"batch_size"`
	OrphanAgeSeconds int64 `json:"orphan_age_seconds"`
}

type RequestContentAuditCleanupResult struct {
	AuditCount         int64 `json:"audit_count"`
	AssetCount         int64 `json:"asset_count"`
	ObjectCount        int64 `json:"object_count"`
	RemovedFileCount   int64 `json:"removed_file_count"`
	OrphanFileCount    int64 `json:"orphan_file_count"`
	TemporaryFileCount int64 `json:"temporary_file_count"`
}

type requestContentAuditCleanupHandler struct{}

func (requestContentAuditCleanupHandler) Type() string {
	return model.SystemTaskTypeRequestContentAudit
}

func (requestContentAuditCleanupHandler) Enabled() bool {
	return true
}

func (requestContentAuditCleanupHandler) Interval() time.Duration {
	return requestContentAuditCleanupInterval
}

func (requestContentAuditCleanupHandler) NewPayload() any {
	settings := RequestContentAuditSettings()
	return RequestContentAuditCleanupPayload{
		RetentionDays:    settings.RetentionDays,
		BatchSize:        requestContentAuditCleanupBatchSize,
		OrphanAgeSeconds: int64((48 * time.Hour).Seconds()),
	}
}

func (requestContentAuditCleanupHandler) Run(ctx context.Context, task *model.SystemTask, runnerID string) {
	payload := RequestContentAuditCleanupPayload{}
	if err := task.DecodePayload(&payload); err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if payload.RetentionDays <= 0 {
		payload.RetentionDays = RequestContentAuditSettings().RetentionDays
	}
	if payload.BatchSize <= 0 {
		payload.BatchSize = requestContentAuditCleanupBatchSize
	}
	if payload.OrphanAgeSeconds <= 0 {
		payload.OrphanAgeSeconds = int64((48 * time.Hour).Seconds())
	}

	store, err := EnsureRequestContentAuditStore(ctx)
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	if store == nil {
		finishRequestContentAuditCleanup(task, runnerID, RequestContentAuditCleanupResult{})
		return
	}

	// ExpiresAt 已在捕获时按 retention_days 计算，这里直接按当前时间清理，避免重复扣除保留期。
	cutoff := time.Now()
	result := RequestContentAuditCleanupResult{}
	for {
		if err := ctx.Err(); err != nil {
			failSystemTask(task, runnerID, err)
			return
		}
		report, cleanupErr := store.CleanupExpired(ctx, cutoff, payload.BatchSize)
		if cleanupErr != nil {
			failSystemTask(task, runnerID, cleanupErr)
			return
		}
		result.AuditCount += report.AuditCount
		result.AssetCount += report.AssetCount
		result.ObjectCount += report.ObjectCount
		result.RemovedFileCount += report.RemovedFileCount
		if report.AuditCount == 0 {
			break
		}
	}

	orphanCount, err := store.CleanupOrphans(ctx, time.Now().Add(-time.Duration(payload.OrphanAgeSeconds)*time.Second))
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	result.OrphanFileCount = orphanCount
	temporaryCount, err := store.CleanupTemporaryFiles(ctx, time.Now().Add(-time.Duration(payload.OrphanAgeSeconds)*time.Second))
	if err != nil {
		failSystemTask(task, runnerID, err)
		return
	}
	result.TemporaryFileCount = temporaryCount
	finishRequestContentAuditCleanup(task, runnerID, result)
}

func finishRequestContentAuditCleanup(task *model.SystemTask, runnerID string, result RequestContentAuditCleanupResult) {
	if err := model.FinishSystemTask(task.TaskID, runnerID, model.SystemTaskStatusSucceeded, result, ""); err != nil {
		logSystemTaskLockError(context.Background(), task, err)
	}
}

func init() {
	RegisterSystemTaskHandler(requestContentAuditCleanupHandler{})
}
