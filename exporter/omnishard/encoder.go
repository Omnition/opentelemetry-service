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
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	jaeger "github.com/jaegertracing/jaeger/model"
	"go.opencensus.io/stats/view"
	"go.uber.org/zap"

	omnishardpb "github.com/Omnition/omnition-opentelemetry-collector/exporter/omnishard/gen"
)

// encoder encodes spans into EncodedRecords. Spans are batched according to their
// traceID and current specified shard configuration.
//
// Here is the primary data flow:
//
//                                                +--------+
//                              +------+  Encode  |Shard   |---->onRecordReady
//                            ->|Worker|  ----->  |Encoder |---->onSpanProcessFail
//            +---------+  --/  +------+          +--------+
// EncodeSpans|Span     |-/               ----->
// ---------->|Batches  |         ...                ...
//            |Queue    |-\               ----->
//            +---------+  --\  +------+          +--------+
//                            ->|Worker|  ----->  |Shard   |--->onRecordReady
//                              +------+          |Encoder |--->onSpanProcessFail
//                                                +--------+
//
type encoder struct {
	// User-specified options.
	options *Options

	// Called when encoded record is ready.
	onRecordReady recordConsumeFunc

	// Called when encoding of spans fails. Also called if encoder is stopped while still
	// having pending spans to encode. All pending spans are returned via this function.
	onSpanProcessFail spanProcessFailFunc

	// The very last requested config change.
	configChangeRequest *omnishardpb.ShardingConfig

	// Condition that is signaled when the config change is requested.
	configChangeCond *sync.Cond

	// Indicates that the config change processing is stopped. This happens when the
	// entire encoder is stopped.
	configChangeStopped bool

	// Queue of spans pending processing by workers.
	spanBatchesToProcess chan []*jaeger.Span

	// This WorkGroup counts the number of batches pending in spanBatchesToProcess.
	spansToProcessWg sync.WaitGroup

	// Non-zero value indicates that encoder is stopping.
	isStoppingFlag int32

	// Channel to signal to workers to stop immediately.
	workerHardStopSignal chan interface{}

	// Points to immutable shardEncoders. Unfortunately atomic.Value is not strongly
	// typed, so be careful to always interpret the value as shardEncoders.
	shardEncoders atomic.Value

	// Metrics view that we are registered with.
	metricViews []*view.View

	// Hooks to publish events to.
	hooks *telemetryHooks

	logger *zap.Logger
}

// Immutable list of shard encoders.
type shardEncoders []*shardEncoder

// Options are the options to be used when initializing an encoder.
type Options struct {
	// ExporterName is the name of the exporter using the encoder.
	ExporterName string

	// Number of workers that will feed spans to shard encoders concurrently.
	// Minimum and default value is 1.
	NumWorkers uint

	// MaxRecordSize is the maximum size of one encoded uncompressed record in bytes.
	// When the record reaches this size, it gets sent to the server.
	MaxRecordSize uint

	// BatchFlushInterval defines how often to flush batches of spans if they don't
	// reach the MaxRecordSize. The default value is defBatchFlushInterval.
	BatchFlushInterval time.Duration

	// MaxAllowedSizePerSpan limits the size of an individual span (in uncompressed encoded
	// bytes). If the encoded size is larger than this the encode will attempt to
	// drop some fields from the span and re-encode (see MaxAllowedSizePerSpan usage
	// in the code below for more details on what is dropped). If that doesn't help the
	// span will be dropped altogether. The default value is defMaxAllowedSizePerSpan.
	MaxAllowedSizePerSpan uint
}

// newEncoder returns an encoder implementation that shards, batches and encodes spans.
// onRecordReady is called when encoded records are ready. onSpanProcessFail is called
// if encoding fails for whatever reason. onRecordReady and onSpanProcessFail may be
// called synchronously from Encode() or asynchronously from a different goroutine.
// Must call SetConfig() after this function for encoder to begin processing.
func newEncoder(
	o *Options,
	logger *zap.Logger,
	onRecordReady recordConsumeFunc,
	onSpanProcessFail spanProcessFailFunc,
) (*encoder, error) {

	// Validate options and populate default values if option is unspecified.

	if o.MaxRecordSize == 0 {
		o.MaxRecordSize = defMaxRecordSize
	}

	if o.BatchFlushInterval == 0 {
		o.BatchFlushInterval = defBatchFlushInterval
	}

	if o.MaxAllowedSizePerSpan == 0 {
		o.MaxAllowedSizePerSpan = defMaxAllowedSizePerSpan
	}

	if o.NumWorkers == 0 {
		o.NumWorkers = 1
	}

	// Create the encoder.
	e := &encoder{
		options:              o,
		logger:               logger,
		onRecordReady:        onRecordReady,
		onSpanProcessFail:    onSpanProcessFail,
		hooks:                newTelemetryHooks(o.ExporterName, ""),
		configChangeCond:     sync.NewCond(&sync.Mutex{}),
		workerHardStopSignal: make(chan interface{}),
	}

	// Create a small queue for spans to be accepted via EncodeSpans() call. We
	// don't need a large queue here, the purpose of the channel is to decouple
	// EncodeSpans() from workers, so o.NumWorkers should be a reasonable queue size.
	e.spanBatchesToProcess = make(chan []*jaeger.Span, o.NumWorkers)

	// Register metric views to collect stats.
	e.metricViews = metricViews()
	if err := view.Register(e.metricViews...); err != nil {
		e.logger.Error("Cannot register metric views", zap.Error(err))
		return nil, err
	}

	// Create empty encoders list so that we don't have to do nil checks when we
	// use e.shardEncoders.
	e.shardEncoders.Store(make(shardEncoders, 0))

	// Start config change processor.
	go e.processConfigChanges()

	return e, nil
}

// SetConfig sets the sharding configuration. Normally called first during startup
// and can be called multiple times later when new config is available. Safe to call
// while processing is in progress.
//
// The first time this function is called with a valid config will result in the start
// of encoder. Until that encoder will not process any spans and any calls to
// EncodeSpans will place spans into queue for future processing when encoder starts.
func (e *encoder) SetConfig(config *omnishardpb.ShardingConfig) {
	// Update configChangeRequest and signal to processConfigChanges.
	// Note: we only care about last call to SetConfig, so we happily overwrite
	// the value of configChangeRequest here without worrying about its previous value.
	e.configChangeCond.L.Lock()
	e.configChangeRequest = config
	e.configChangeCond.Signal()
	e.configChangeCond.L.Unlock()
}

// EncodeSpans encodes a span and emits resulting batches as encoded records
// via onRecordReady or onSpanProcessFail callbacks. If EncodeSpans is called
// before SetConfig then the spans will be enqueued and will be processed
// after a valid config provided via SetConfig call.
//
// EncodeSpans puts the spans to a size-limited queue and blocks if there is no
// more room in the queue.
func (e *encoder) EncodeSpans(spans []*jaeger.Span) error {
	if e.isStopping() {
		// We will not accept new spans when we are already stopping.
		return errors.New("encoder is already stopped")
	}

	// Count the addition of a span batch.
	e.spansToProcessWg.Add(1)

	// This will block and apply backpressure to caller if we have no room in
	// spanBatchesToProcess queue.
	e.spanBatchesToProcess <- spans
	e.hooks.OnSpansEnqueued(int64(len(spans)))

	return nil
}

// Stop the encoder gracefully.
//
// Will wait up to maxDrainDuration to encode and drain spans that are already pending
// in the queue. During graceful stopping no new spans will be accepted for encoding via
// EncodeSpans function. Any spans that are still remaining after graceful stopping period
// will be dropped.
func (e *encoder) Stop(maxDrainDuration time.Duration) {
	// Begin graceful shutdown by setting the stopping flag. We will no longer accept new
	// spans for processing but will continue draining spans pending in the queue.
	atomic.StoreInt32(&e.isStoppingFlag, 1)

	// Wait until all pending spans are processed but no more than maxDrainDuration.
	// spansToProcessWg counts the number of span batches that are pending processing.
	if !waitGroupTimeout(&e.spansToProcessWg, maxDrainDuration) {
		e.logger.Warn("timed out waiting for encoder to drain spans")
	}

	// We either successfully drained spans from our queue to shard encoders or timed out.
	// As a result we may have some spans accumulated in shard encoders. Now flush them.

	// TODO: add timeout support to flushShardEncoders() and use here.
	e.flushShardEncoders()

	// Stop shard encoders.
	e.stopShardEncoders()

	// Signal stop to processConfigChanges.
	e.configChangeCond.L.Lock()
	e.configChangeStopped = true
	e.configChangeCond.L.Unlock()
	e.configChangeCond.Signal()

	// Unregister views that are registered during creation.
	view.Unregister(e.metricViews...)

	// Signal hard stop to workers.
	close(e.workerHardStopSignal)
}

func (e *encoder) flushShardEncoders() {
	ses := e.shardEncoders.Load().(shardEncoders)

	var wg sync.WaitGroup
	wg.Add(len(ses))
	for _, se := range ses {
		shardEncoder := se
		go func() {
			// TODO: add timeout support to Flush() and use here.
			shardEncoder.Flush()
			wg.Done()
		}()
	}

	wg.Wait()
}

// processConfigChanges watches for changes in configChangeRequest field and applies them.
func (e *encoder) processConfigChanges() {
	workersStarted := false
	for {
		e.configChangeCond.L.Lock()

		// Wait for config change request or for stop request.
		for e.configChangeRequest == nil && !e.configChangeStopped {
			e.configChangeCond.Wait()
		}

		// Mutex is now locked and one of the 2 conditions is present. Check which one.

		if e.configChangeStopped {
			// encoder is stopped. We are done here.
			e.configChangeCond.L.Unlock()
			return
		}

		if e.configChangeRequest != nil {
			// Config change request is present. Store it in local var.
			newConfig := e.configChangeRequest

			// Reset the request.
			e.configChangeRequest = nil

			// Unlock the mutex as soon as we can, we don't need to hold it anymore.
			e.configChangeCond.L.Unlock()

			// We have a new config to process.
			e.stopShardEncoders()
			cfg, err := newShardingInMemConfig(newConfig)
			if err != nil {
				e.logger.Error("Received invalid sharding config, ignoring.", zap.Error(err))
			} else {
				// Config is good, apply it.
				e.startShardEncoders(cfg)

				if !workersStarted {
					// We have a config and workers are not yet started. Start them now.
					workersStarted = true
					e.startWorkers()
				}

			}
		} else {
			e.configChangeCond.L.Unlock()
		}
	}
}

func (e *encoder) startWorkers() {
	// Start workers to get pending spans and push them to shard encoders.
	for i := 0; i < int(e.options.NumWorkers); i++ {
		go e.processQueuedSpans()
	}
}

// Stops shard encoders. Signals to encoders to stop but does not wait for encoders to
// actually stop, so they may continue running briefly while they get the signal.
// Any currently processing spans that are not yet emitted via onRecordReady will be
// returned by calling onSpanProcessFail func.
func (e *encoder) stopShardEncoders() {
	ses := e.shardEncoders.Load().(shardEncoders)

	// Tell all encoders to stop.
	for _, se := range ses {
		se.Stop()
	}
}

// startShardEncoders starts shard encoders. Safe to call after calling
// stopShardEncoders.
func (e *encoder) startShardEncoders(cfg *shardingInMemConfig) {
	// Create shard encoders based on the config.
	o := e.options
	encoders := make(shardEncoders, 0, len(cfg.shards))
	for _, shard := range cfg.shards {
		// Create hooks to report per-shard operations.
		hooks := newTelemetryHooks(o.ExporterName, shard.shardID)
		encoders = append(encoders, newShardEncoder(
			e.logger,
			shard,
			hooks,
			o.MaxRecordSize,
			o.MaxAllowedSizePerSpan,
			o.BatchFlushInterval,
			e.onRecordReady,
			e.onSpanProcessFail,
		))
	}

	// Start them all.
	for _, se := range encoders {
		se.Start()
	}

	// Now atomically swap old encoders by new ones. In reality at this point
	// we should have empty old encoders list, but we still want to do this swapping
	// atomically so that others can safely read the list via atomic Load().
	e.shardEncoders.Store(encoders)
}

// waitGroupTimeout waits for WaitGroup for up to timeout duration.
// Returns false if waiting timed out.
func waitGroupTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	c := make(chan struct{})
	go func() {
		// Signal that Wait() was completed.
		defer close(c)
		wg.Wait()
	}()

	select {
	case <-c:
		// Wait completed normally.
		return true
	case <-time.After(timeout):
		// Timed out.
		return false
	}
}

func (e *encoder) isStopping() bool {
	return atomic.LoadInt32(&e.isStoppingFlag) != 0
}

// processQueuedSpans is the main routine of the worker. We have NumWorkers of these
// running.
func (e *encoder) processQueuedSpans() {
	// Get spans from spanBatchesToProcess and process them until we get a hard stop
	// signal.
	for {
		select {
		case spans := <-e.spanBatchesToProcess:
			// We have spans to process.
			e.hooks.OnSpansDequeued(int64(len(spans)))
			e.processSpanBatch(spans)
		case <-e.workerHardStopSignal:
			// We got a hard stop signal and must terminate now.
			return
		}
	}
}

// processSpanBatch gets a span that must be processed and pushes it to the appropriate
// shard encoder.
func (e *encoder) processSpanBatch(spans []*jaeger.Span) {

	for _, span := range spans {

		// Find the right shard encoder based on trace id.
		se, err := e.getShardEncoder(span.TraceID.String())
		if err != nil {
			// TODO: consider removing the log if reporting via onSpanProcessFail is
			// sufficient for troubleshooting.
			e.logger.Error("failed to get producer/shard", zap.Error(err),
				zap.String("traceID", span.TraceID.String()))
			e.onSpanProcessFail([]*jaeger.Span{span}, ErrEncodingFailed)
			continue
		}

		se.Encode(span)
	}

	// Count the processing of a span batch.
	e.spansToProcessWg.Done()
}

func (e *encoder) getShardEncoder(partitionKey string) (*shardEncoder, error) {
	// Atomically load the pointer to the list of encoders. Note: if we are stopped
	// the list may be empty (but never nil so it is safe to use).
	ses := e.shardEncoders.Load().(shardEncoders)

	// TODO: Consider using binary search in a future PR. Each belongsToShard may be
	// relatively slow (does md5, than big.Int comparisons) so binary search may be warranted.
	for _, se := range ses {
		if se.shard.belongsToShard(partitionKey) {
			return se, nil
		}
	}
	return nil, fmt.Errorf("no shard found for parition key %s", partitionKey)
}
