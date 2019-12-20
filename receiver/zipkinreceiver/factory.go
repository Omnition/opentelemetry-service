// Copyright 2019 OpenTelemetry Authors
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

package zipkinreceiver

import (
	"context"
	"net/http"

	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/receiver"
	"github.com/open-telemetry/opentelemetry-collector/receiver/zipkinreceiver"
	"go.uber.org/zap"
)

// Factory implements a trace receiver factory that creates a ZipkinReceiver
type Factory struct {
	zipkinreceiver.Factory
}

// CreateTraceReceiver wraps zipkinreceiver.ZipkinReceiver with a local ZipkinReceiver that implements IP address
// extraction on to the context.
func (f *Factory) CreateTraceReceiver(ctx context.Context, logger *zap.Logger, config configmodels.Receiver, nextConsumer consumer.TraceConsumer) (receiver.TraceReceiver, error) {
	tr, err := f.Factory.CreateTraceReceiver(ctx, logger, config, nextConsumer)
	if err != nil {
		return nil, err
	}
	zr := tr.(*zipkinreceiver.ZipkinReceiver)
	zr = zr.WithHTTPServer(&http.Server{Handler: httpHanlder(zr)})
	return zr, nil

}
