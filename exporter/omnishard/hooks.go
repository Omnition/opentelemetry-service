// Copyright 2019 Omnition Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package omnishard

import (
	"context"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"

	omnishardpb "github.com/Omnition/omnition-opentelemetry-collector/exporter/omnishard/gen"
)

var (
	tagUpsertExportResponseSuccess             = tag.Upsert(tagSendResult, "SUCCESS")
	tagUpsertExportResponseFailedNotRetryable  = tag.Upsert(tagSendResult, "FAILED_NOT_RETRYABLE")
	tagUpsertExportResponseFailedRetryable     = tag.Upsert(tagSendResult, "FAILED_RETRYABLE")
	tagUpsertExportResponseShardConfigMismatch = tag.Upsert(tagSendResult, "SHARD_CONFIG_MISTMATCH")

	tagUpsertDropCodeRetryQueueFull             = tag.Upsert(tagDropReason, "RetryQueueFull")
	tagUpsertDropCodeFatalEncondingError        = tag.Upsert(tagDropReason, "FatalEncodingError")
	tagUpsertDropCodeExportResponseNotRetryable = tag.Upsert(tagDropReason, "ExportResponseNotRetryable")
	tagUpsertDropCodeSendErrNotRetryable        = tag.Upsert(tagDropReason, "SendErrNotRetryable")
)

type telemetryHooks struct {
	exporterName string
	commonTags   []tag.Mutator

	exportSuccessTags             []tag.Mutator
	exportFailedNotRetryableTags  []tag.Mutator
	exportFailedRetryableTags     []tag.Mutator
	exportShardConfigMismatchTags []tag.Mutator

	dropRetryQueueFullTags             []tag.Mutator
	dropFatalEncodingErrorTags         []tag.Mutator
	dropExportResponseNotRetryableTags []tag.Mutator
	dropSendErrNotRetryableTags        []tag.Mutator
}

func newTelemetryHooks(exporterName, shardID string) *telemetryHooks {
	tags := []tag.Mutator{
		tag.Upsert(tagExporterName, exporterName),
	}
	if shardID != "" {
		tags = append(tags, tag.Upsert(tagShardID, shardID))
	}

	h := &telemetryHooks{
		exporterName: exporterName,
		commonTags:   tags,
	}

	// Prepare the pre-defined tags for export metrics.
	h.exportSuccessTags = extendTagMutator(tags, tagUpsertExportResponseSuccess)
	h.exportFailedNotRetryableTags = extendTagMutator(tags, tagUpsertExportResponseFailedNotRetryable)
	h.exportFailedRetryableTags = extendTagMutator(tags, tagUpsertExportResponseFailedRetryable)
	h.exportShardConfigMismatchTags = extendTagMutator(tags, tagUpsertExportResponseShardConfigMismatch)

	// Prepare the pre-defined tags for drop metrics.
	h.dropRetryQueueFullTags = extendTagMutator(tags, tagUpsertDropCodeRetryQueueFull)
	h.dropFatalEncodingErrorTags = extendTagMutator(tags, tagUpsertDropCodeFatalEncondingError)
	h.dropExportResponseNotRetryableTags = extendTagMutator(tags, tagUpsertDropCodeExportResponseNotRetryable)
	h.dropSendErrNotRetryableTags = extendTagMutator(tags, tagUpsertDropCodeSendErrNotRetryable)

	return h
}

func extendTagMutator(baseTags []tag.Mutator, extendedTags ...tag.Mutator) []tag.Mutator {
	newTags := make([]tag.Mutator, 0, len(baseTags)+len(extendedTags))
	newTags = append(newTags, baseTags...)
	return append(newTags, extendedTags...)
}

func (h *telemetryHooks) OnSpansEnqueued(spans int64) {
	_ = stats.RecordWithTags(
		context.Background(),
		h.commonTags,
		statEnqueuedSpans.M(spans),
	)
}

func (h *telemetryHooks) OnSpansDequeued(spans int64) {
	_ = stats.RecordWithTags(
		context.Background(),
		h.commonTags,
		statDequeuedSpans.M(spans),
	)
}

func (h *telemetryHooks) OnXLSpanTruncated(size int) {
	_ = stats.RecordWithTags(
		context.Background(),
		h.commonTags,
		statXLSpansBytes.M(int64(size)),
	)
}

func (h *telemetryHooks) OnSpanListFlushed(spans, bytes int64) {
	_ = stats.RecordWithTags(
		context.Background(),
		h.commonTags,
		statFlushedSpans.M(spans),
		statFlushedSpansBytes.M(bytes),
	)
}

func (h *telemetryHooks) OnCompressed(original, compressed int64) {
	_ = stats.RecordWithTags(
		context.Background(),
		h.commonTags,
		statCompressFactor.M(original/compressed),
	)
}

func (h *telemetryHooks) OnDropSpans(
	dataDropCode DataDropCode,
	spans int64,
	record *omnishardpb.EncodedRecord) {

	var mutators []tag.Mutator
	switch dataDropCode {
	case RetryQueueFull:
		mutators = h.dropRetryQueueFullTags
	case FatalEncodingError:
		mutators = h.dropFatalEncodingErrorTags
	case ExportResponseNotRetryable:
		mutators = h.dropExportResponseNotRetryableTags
	case SendErrNotRetryable:
		mutators = h.dropSendErrNotRetryableTags
	}

	if record == nil {
		_ = stats.RecordWithTags(
			context.Background(),
			mutators,
			statDroppedSpans.M(spans),
		)
		return
	}

	_ = stats.RecordWithTags(
		context.Background(),
		mutators,
		statDroppedSpans.M(spans),
		statDroppedBytes.M(int64(len(record.Data))),
	)
}

func (h *telemetryHooks) OnSendResponse(
	record *omnishardpb.EncodedRecord,
	response *omnishardpb.ExportResponse,
) {
	var mutators []tag.Mutator
	switch response.ResultCode {
	case omnishardpb.ExportResponse_SUCCESS:
		mutators = h.exportSuccessTags
	case omnishardpb.ExportResponse_FAILED_NOT_RETRYABLE:
		mutators = h.exportFailedNotRetryableTags
	case omnishardpb.ExportResponse_FAILED_RETRYABLE:
		mutators = h.exportFailedRetryableTags
	case omnishardpb.ExportResponse_SHARD_CONFIG_MISTMATCH:
		mutators = h.exportShardConfigMismatchTags
	}

	_ = stats.RecordWithTags(
		context.Background(),
		mutators,
		statSentBytes.M(int64(len(record.Data))),
		statSentSpans.M(record.SpanCount))
}
