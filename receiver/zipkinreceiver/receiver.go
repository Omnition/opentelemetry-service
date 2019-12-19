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
	"errors"
	"net"
	"net/http"
	"sync"

	"github.com/open-telemetry/opentelemetry-collector/consumer"
	"github.com/open-telemetry/opentelemetry-collector/oterr"
	"github.com/open-telemetry/opentelemetry-collector/receiver"
	"github.com/open-telemetry/opentelemetry-collector/receiver/zipkinreceiver"

	"github.com/Omnition/omnition-opentelemetry-collector/client"
)

const defaultAddress = ":9411"

// ZipkinReceiver wraps zipkin receiver from opentelemetry-collector reop and adds support for detecting IP
// address of the sending process.
// TODO: export HTTP server in upstream receiver and get rid of most of the duplication here
type ZipkinReceiver struct {
	*zipkinreceiver.ZipkinReceiver

	mu sync.Mutex

	// addr is the address onto which the HTTP server will be bound
	addr         string
	host         receiver.Host
	nextConsumer consumer.TraceConsumer

	startOnce sync.Once
	stopOnce  sync.Once
	server    *http.Server
}

// New creates a new ZipkinReceiver reference.
func New(zr *zipkinreceiver.ZipkinReceiver, address string, nextConsumer consumer.TraceConsumer) (*ZipkinReceiver, error) {
	if nextConsumer == nil {
		return nil, oterr.ErrNilNextConsumer
	}

	r := &ZipkinReceiver{
		ZipkinReceiver: zr,
		addr:           address,
		nextConsumer:   nextConsumer,
	}
	return r, nil
}

func (zr *ZipkinReceiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if c, ok := client.FromHTTP(r); ok {
		ctx := client.NewContext(r.Context(), c)
		r = r.WithContext(ctx)
	}
	zr.ZipkinReceiver.ServeHTTP(w, r)
}

// StartTraceReception spins up the receiver's HTTP server and makes the receiver start its processing.
func (zr *ZipkinReceiver) StartTraceReception(host receiver.Host) error {
	if host == nil {
		return errors.New("nil host")
	}

	zr.mu.Lock()
	defer zr.mu.Unlock()

	var err = oterr.ErrAlreadyStarted

	zr.startOnce.Do(func() {
		ln, lerr := net.Listen("tcp", zr.address())
		if lerr != nil {
			err = lerr
			return
		}

		zr.host = host
		server := &http.Server{Handler: zr}
		zr.server = server
		go func() {
			host.ReportFatalError(server.Serve(ln))
		}()

		err = nil
	})

	return err
}

// StopTraceReception tells the receiver that should stop reception,
// giving it a chance to perform any necessary clean-up and shutting down
// its HTTP server.
func (zr *ZipkinReceiver) StopTraceReception() error {
	var err = oterr.ErrAlreadyStopped
	zr.stopOnce.Do(func() {
		err = zr.server.Close()
	})
	return err
}

func (zr *ZipkinReceiver) address() string {
	addr := zr.addr
	if addr == "" {
		addr = defaultAddress
	}
	return addr
}
