// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package memorylimiter

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-service/config"
	"github.com/open-telemetry/opentelemetry-service/config/configmodels"
	"github.com/open-telemetry/opentelemetry-service/processor"
)

func TestLoadConfig(t *testing.T) {
	receivers, _, exporters, err := config.ExampleComponents()
	require.NoError(t, err)
	processors, err := processor.Build(&Factory{})
	require.NotNil(t, processors)
	require.NoError(t, err)

	config, err := config.LoadConfigFile(
		t,
		path.Join(".", "testdata", "config.yaml"),
		receivers,
		processors,
		exporters)

	require.Nil(t, err)
	require.NotNil(t, config)

	p0 := config.Processors["memory-limiter"]
	assert.Equal(t, p0,
		&Config{
			ProcessorSettings: configmodels.ProcessorSettings{
				TypeVal: "memory-limiter",
				NameVal: "memory-limiter",
			},
		})

	p1 := config.Processors["memory-limiter/with-settings"]
	assert.Equal(t, p1,
		&Config{
			ProcessorSettings: configmodels.ProcessorSettings{
				TypeVal: "memory-limiter",
				NameVal: "memory-limiter/with-settings",
			},
			CheckInterval:       250 * time.Millisecond,
			MemoryLimitMiB:      4000,
			MemorySpikeLimitMiB: 500,
			BallastSizeMiB:      2000,
		})
}
