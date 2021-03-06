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

package opencensusexporter

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector/config"
	"github.com/open-telemetry/opentelemetry-collector/config/configgrpc"
	"github.com/open-telemetry/opentelemetry-collector/config/configmodels"
	"github.com/open-telemetry/opentelemetry-collector/exporter/opencensusexporter"
)

func TestLoadConfig(t *testing.T) {
	factories, err := config.ExampleComponents()
	assert.Nil(t, err)

	factory := &Factory{}
	factories.Exporters[typeStr] = factory
	cfg, err := config.LoadConfigFile(
		t, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(t, err)
	require.NotNil(t, cfg)

	e0 := cfg.Exporters["opencensus"]
	assert.Equal(t, e0, factory.CreateDefaultConfig())

	e1 := cfg.Exporters["opencensus/2"]
	assert.Equal(t, e1,
		&Config{
			Config: opencensusexporter.Config{
				ExporterSettings: configmodels.ExporterSettings{
					NameVal: "opencensus/2",
					TypeVal: "opencensus",
				},
				GRPCSettings: configgrpc.GRPCSettings{
					Headers: map[string]string{
						"can you have a . here?": "F0000000-0000-0000-0000-000000000000",
						"header1":                "234",
						"another":                "somevalue",
					},
					Endpoint:    "1.2.3.4:1234",
					Compression: "on",
					CertPemFile: "/var/lib/mycert.pem",
					UseSecure:   true,
					KeepaliveParameters: &configgrpc.KeepaliveConfig{
						Time:                20,
						PermitWithoutStream: true,
						Timeout:             30,
					},
				},
				NumWorkers:        123,
				ReconnectionDelay: 15,
			},
			UseUnaryExporter:     true,
			UnaryExporterTimeout: time.Second * 10,
		})

	e2 := cfg.Exporters["opencensus/unary-disabled"]
	cfg2 := factory.CreateDefaultConfig().(*Config)
	cfg2.ExporterSettings.NameVal = "opencensus/unary-disabled"
	cfg2.UseUnaryExporter = false
	assert.Equal(t, e2, cfg2)
}
