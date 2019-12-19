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
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// Keys and stats for telemetry.
var (
	tagExporterName, _ = tag.NewKey("exporter")
	tagShardID, _      = tag.NewKey("shard_id")
	tagSendResult, _   = tag.NewKey("send_result")
	tagDropReason, _   = tag.NewKey("drop_reason")

	statXLSpansBytes = stats.Int64("omnishard_xl_span_size", "size of spans bigger than max support size", stats.UnitBytes)

	statFlushedSpans      = stats.Int64("omnishard_flushed_spans", "number of spans flushed to omnishard exporter", stats.UnitDimensionless)
	statFlushedSpansBytes = stats.Int64("omnishard_flushed_bytes", "total size in bytes of spans flushed to omnishard exporter", stats.UnitBytes)

	statEnqueuedSpans = stats.Int64("omnishard_enqueued_spans", "spans received and put in a queue to be processed by omnishard exporter", stats.UnitDimensionless)
	statDequeuedSpans = stats.Int64("omnishard_dequeued_spans", "spans taken out of queue and processed by omnishard exporter", stats.UnitDimensionless)

	statCompressFactor = stats.Int64("omnishard_compress_factor", "compression factor achieved by spanlists", stats.UnitDimensionless)

	statDroppedSpans = stats.Int64("omnishard_dropped_spans", "dropped span count", stats.UnitDimensionless)
	statDroppedBytes = stats.Int64("omnishard_dropped_bytes", "dropped bytes count", stats.UnitBytes)

	statSentSpans = stats.Int64("omnishard_sent_spans", "number of spans sent", stats.UnitDimensionless)
	statSentBytes = stats.Int64("omnishard_sent_bytes", "number of bytes sent", stats.UnitBytes)
)

// TODO: support telemetry level

// MetricViews return the metrics views according to given telemetry level.
func metricViews() []*view.View {
	exporterTags := []tag.Key{tagExporterName}
	exporterShardIDTags := []tag.Key{tagExporterName, tagShardID}

	// There are some metrics enabled, return the views.

	flushedSpansView := &view.View{
		Name:        statFlushedSpans.Name(),
		Measure:     statFlushedSpans,
		Description: statFlushedSpans.Description(),
		TagKeys:     exporterShardIDTags,
		Aggregation: view.Sum(),
	}

	flushedBatchesView := &view.View{
		Name:        "omnishard_flushed_batches",
		Measure:     statFlushedSpans,
		Description: "number of batches flushed to omnishard exporter",
		TagKeys:     exporterShardIDTags,
		Aggregation: view.Count(),
	}

	flushedBatchBytesView := &view.View{
		Name:        statFlushedSpansBytes.Name(),
		Measure:     statFlushedSpansBytes,
		Description: statFlushedSpansBytes.Description(),
		TagKeys:     exporterShardIDTags,
		Aggregation: view.LastValue(),
	}

	xlSpansBytesView := &view.View{
		Name:        statXLSpansBytes.Name(),
		Measure:     statXLSpansBytes,
		Description: statXLSpansBytes.Description(),
		TagKeys:     exporterShardIDTags,
		Aggregation: view.Sum(),
	}

	xlSpansView := &view.View{
		Name:        "omnishard_xl_spans",
		Measure:     statXLSpansBytes,
		Description: "number of spans found that were larger than max supported size",
		TagKeys:     exporterShardIDTags,
		Aggregation: view.Count(),
	}

	enqueuedSpansView := &view.View{
		Name:        statEnqueuedSpans.Name(),
		Measure:     statEnqueuedSpans,
		Description: statEnqueuedSpans.Description(),
		TagKeys:     exporterTags,
		Aggregation: view.Sum(),
	}

	enqueuedBatchesView := &view.View{
		Name:        "omnishard_enqueued_batches",
		Measure:     statEnqueuedSpans,
		Description: "batches received and put in a queue to be processed by omnishard exporter",
		TagKeys:     exporterTags,
		Aggregation: view.Count(),
	}

	dequeuedSpansView := &view.View{
		Name:        statDequeuedSpans.Name(),
		Measure:     statDequeuedSpans,
		Description: statDequeuedSpans.Description(),
		TagKeys:     exporterTags,
		Aggregation: view.Sum(),
	}

	dequeuedBatchesView := &view.View{
		Name:        "omnishard_dequeued_batches",
		Measure:     statDequeuedSpans,
		Description: "batches taken out of queue and processed by omnishard exporter",
		TagKeys:     exporterTags,
		Aggregation: view.Count(),
	}

	compressFactorView := &view.View{
		Name:        statCompressFactor.Name(),
		Measure:     statCompressFactor,
		Description: statCompressFactor.Description(),
		TagKeys:     exporterShardIDTags,
		Aggregation: view.LastValue(),
	}

	droppedTagKeys := make([]tag.Key, len(exporterTags)+1)
	copy(droppedTagKeys, exporterTags)
	droppedTagKeys[len(exporterTags)] = tagDropReason

	droppedSpansView := &view.View{
		Name:        statDroppedSpans.Name(),
		Measure:     statDroppedSpans,
		Description: statDroppedSpans.Description(),
		TagKeys:     droppedTagKeys,
		Aggregation: view.Sum(),
	}

	droppedBatchesView := &view.View{
		Name:        "omnishard_dropped_batches",
		Measure:     statDroppedSpans,
		Description: "dropped batches count",
		TagKeys:     droppedTagKeys,
		Aggregation: view.Count(),
	}

	droppedBytesView := &view.View{
		Name:        statDroppedBytes.Name(),
		Measure:     statDroppedBytes,
		Description: statDroppedBytes.Description(),
		TagKeys:     droppedTagKeys,
		Aggregation: view.Sum(),
	}

	sentTagKeys := make([]tag.Key, len(exporterTags)+1)
	copy(sentTagKeys, exporterTags)
	sentTagKeys[len(exporterTags)] = tagSendResult

	sentSpansView := &view.View{
		Name:        statSentSpans.Name(),
		Measure:     statSentSpans,
		Description: statSentSpans.Description(),
		TagKeys:     sentTagKeys,
		Aggregation: view.Sum(),
	}

	sentBatchesView := &view.View{
		Name:        "omnishard_sent_batches",
		Measure:     statSentSpans,
		Description: "number of batches sent",
		TagKeys:     sentTagKeys,
		Aggregation: view.Count(),
	}

	sentBytesView := &view.View{
		Name:        statSentBytes.Name(),
		Measure:     statSentBytes,
		Description: statSentBytes.Description(),
		TagKeys:     sentTagKeys,
		Aggregation: view.Sum(),
	}

	return []*view.View{
		compressFactorView,
		flushedSpansView,
		flushedBatchesView,
		flushedBatchBytesView,
		xlSpansBytesView,
		xlSpansView,
		enqueuedSpansView,
		enqueuedBatchesView,
		dequeuedSpansView,
		dequeuedBatchesView,
		droppedSpansView,
		droppedBatchesView,
		droppedBytesView,
		sentSpansView,
		sentBatchesView,
		sentBytesView,
	}
}
