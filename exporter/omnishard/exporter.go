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
	"time"

	jaeger "github.com/jaegertracing/jaeger/model"
	"github.com/open-telemetry/opentelemetry-collector/consumer/consumerdata"
	"github.com/open-telemetry/opentelemetry-collector/exporter"
	jaegertranslator "github.com/open-telemetry/opentelemetry-collector/translator/trace/jaeger"
	"go.uber.org/zap"

	omnishardpb "github.com/Omnition/omnition-opentelemetry-collector/exporter/omnishard/gen"
)

// Maximum number of batches allowed in the retry queue.
// TODO: calculate dynamically based on expected maximum.
const retryQueueSize = 10000

// Exporter implements an OpenTelemetry trace exporter that exports spans via
// OmniShard protocol.
//
// The Exporter is composed of an Encoder that is responsible for sharding
// and encoding spans into records which are ready for sending and a Client
// which is responsible for sending encoded records to the server and for receiving
// sharding configuration from the server.
//                                                +------------------+
//                                                |     Exporter     |<------+
//                                                +------------------+       |
//                                                  |              |         |
//                                                  |              |         |
//                                                  |Stop/         |Stop/    |
//                                                  |Start         |Start    |Config
//                                                  v              |         |
//                +---------+         +----------------+           |         |
//     Export --->|Translate|-------->|                |           v         |
//                +---------+         |                |         +-------------+
//                                    |    Encoder     |-------->|    Client   |---> Server
//                                    |                |         +-------------+
//                    --------------->|                |      Failed |    |Failed
//                    |               +----------------+   Retryable |    |Fatal
//                    |             Failed|       |Failed            |    v
//                    |          Retryable|       |Fatal             |  Drop
//                    |                   |       v                  |
//                    |   +----------+    |      Drop                |
//                    +---|   Retry  |<---+                          |
//                        |   Queue  |<------------------------------+
//                        +----------+
//                             |Queue
//                             |Full
//                             |
//                             v
//                            Drop
//
type Exporter struct {
	// User-specified config.
	cfg *Config

	// Spans that need to be retried processing.
	spansToRetry chan []*jaeger.Span

	// Span Encoder is used for sharding and encoding the spans into records that are ready
	// to be sent via client.
	encoder *encoder

	// Client to connect to server and send encoded records.
	client client

	// Channel for "done" signalling.
	done chan interface{}

	logger *zap.Logger
}

var _ (exporter.TraceExporter) = (*Exporter)(nil)

// NewExporter creates a new Exporter based on user config.
func NewExporter(cfg *Config, logger *zap.Logger, client client) (*Exporter, error) {
	e := &Exporter{
		cfg:          cfg,
		spansToRetry: make(chan []*jaeger.Span, retryQueueSize),
		logger:       logger,
		done:         make(chan interface{}),
		client:       client,
	}

	opt := &Options{
		ExporterName:          cfg.Name(),
		NumWorkers:            cfg.NumWorkers,
		MaxAllowedSizePerSpan: cfg.MaxAllowedSizePerSpan,
		BatchFlushInterval:    cfg.BatchFlushInterval,
		MaxRecordSize:         cfg.MaxRecordSize,
	}

	var err error
	e.encoder, err = newEncoder(opt, logger, e.onRecordReady, e.onSpanProcessFail)
	if err != nil {
		return nil, err
	}
	return e, nil
}

// Start starts the goroutine that connects to endpoint and retrieves the configuration.
func (e *Exporter) Start(host exporter.Host) error {
	go e.connectAndGetConfig()
	return nil
}

// Shutdown the exporter. Exporter cannot be started after it is shutdown. Safe to call
// concurrently with other public functions.
func (e *Exporter) Shutdown() error {
	// Close channels to stop processing.
	close(e.done)

	// Drain and stop the encoder.
	e.encoder.Stop(10 * time.Second)

	e.client.Shutdown()

	return nil
}

func (e *Exporter) connectAndGetConfig() {
	co := ConnectionOptions{
		Endpoint:        e.cfg.Endpoint,
		UseSecure:       e.cfg.UseSecure,
		SendConcurrency: e.cfg.SendConcurrency,
		OnSendFail:      e.OnSendFail,
		OnSendResponse:  e.OnSendResponse,
	}

	// Connect to the server.
	if err := e.client.Connect(co, e.done); err != nil {
		e.logger.Error("Error connecting to the server",
			zap.Error(err))
		return
	}

	// Connected. Now get the sharding config.
	shardingConfig, err := e.client.GetShardingConfig()
	if err != nil {
		e.logger.Error("Error getting sharding config from the server",
			zap.Error(err))
		return
	}

	// Initialize Encoder.
	e.encoder.SetConfig(shardingConfig)

	// Initialization is done. Begin processing spans.
	go e.processRetryQueue()
}

// ConsumeTraceData receives a span batch and exports it to OmniShard server.
func (e *Exporter) ConsumeTraceData(c context.Context, td consumerdata.TraceData) error {
	// Translate to Jaeger format. This is the only operation that does not depend
	// on sharding and which we can perform regardless of sharding configuration.
	// Subsequent processing operations depend on sharding and may need to be repeated
	// when sharding configuration changes, so they will go through inputSpans pipeline
	// again in that case.
	pBatch, err := jaegertranslator.OCProtoToJaegerProto(td)
	if err != nil {
		e.logger.Error("error translating span batch", zap.Error(err))
		return err
	}

	spans := pBatch.GetSpans()

	for _, span := range spans {
		if span.Process == nil {
			span.Process = pBatch.Process
		}
	}

	// Encode the span. Wait if encoder is busy. This will apply backpressure.
	return e.encoder.EncodeSpans(spans)
}

func (e *Exporter) processRetryQueue() {
	// Read spans from the retry queue and try to encode them again.
	for {
		select {
		case <-e.done:
			return

		case spans := <-e.spansToRetry:
			e.encoder.EncodeSpans(spans)
		}
	}
}

// onRecordReady is called when an encoded record is ready. This function will send it
// to the server.
func (e *Exporter) onRecordReady(record *omnishardpb.EncodedRecord, originalSpans []*jaeger.Span, shard *shardInMemConfig) {
	e.client.Send(record, originalSpans, shard.origin)
}

func (e *Exporter) resendSpans(spans []*jaeger.Span) {
	// Push the failed spans to the retry queue to be picked up by processRetryQueue later.
	select {
	case e.spansToRetry <- spans:
	default:
		// No more room in spansToRetry queue. Drop the failed spans.
		e.encoder.hooks.OnDropSpans(RetryQueueFull, int64(len(spans)), nil)
	}
}

// onSpanProcessFail is called when encoding of spans failed. The function attempt to
// schedule the spans for re-encoding.
func (e *Exporter) onSpanProcessFail(failedSpans []*jaeger.Span, code EncoderErrorCode) {
	switch code {
	case ErrEncoderStopped:
		// If we get ErrEncoderStopped it means encoding was attempted when encoder is
		// stopped it is an indication that re-configuration is in progress. So we need
		// to retry the encoding.
		e.resendSpans(failedSpans)

	case ErrEncodingFailed:
		e.encoder.hooks.OnDropSpans(FatalEncodingError, int64(len(failedSpans)), nil)
	}
}

// OnSendResponse is called when a response is received regarding previously sent records.
// Called asynchronously sometime after Send() successfully sends the record.
func (e *Exporter) OnSendResponse(
	responseToRecords *omnishardpb.EncodedRecord,
	originalSpans []*jaeger.Span,
	response *omnishardpb.ExportResponse,
) {
	switch response.ResultCode {
	case omnishardpb.ExportResponse_SUCCESS:
		// Spans were successfully delivered.

	case omnishardpb.ExportResponse_FAILED_NOT_RETRYABLE:
		// Spans are not accepted by the server.
		e.encoder.hooks.OnDropSpans(
			ExportResponseNotRetryable,
			int64(len(originalSpans)),
			responseToRecords)

	case omnishardpb.ExportResponse_FAILED_RETRYABLE:
		// Spans must be retried.
		e.resendSpans(originalSpans)

	case omnishardpb.ExportResponse_SHARD_CONFIG_MISTMATCH:
		// Server's sharding configuration has changed.
		// Let encoder know about new config.
		e.encoder.SetConfig(response.ShardingConfig)

		// And schedule to resend spans which were previously sent but rejected
		// because they were encoded for wrong sharding config.
		e.resendSpans(originalSpans)
	}

	e.encoder.hooks.OnSendResponse(responseToRecords, response)
}

// OnSendFail is called if the records cannot be sent for whatever reason (e.g. the
// records cannot be serialized).
func (e *Exporter) OnSendFail(
	failedRecords *omnishardpb.EncodedRecord,
	originalSpans []*jaeger.Span,
	code SendErrorCode,
) {
	switch code {
	case ErrFailedNotRetryable:
		e.encoder.hooks.OnDropSpans(
			SendErrNotRetryable,
			int64(len(originalSpans)),
			failedRecords)

	case ErrFailedRetryable:
		e.resendSpans(originalSpans)
	}
}
