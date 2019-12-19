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

package telemetryextension

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO: When extensions are able to access the global config and command line parameters,
//  add proper tests.

func Test_parseArgs(t *testing.T) {
	tests := []struct {
		name            string
		args            []string
		wantMetricsPort string
		wantConfigFile  string
	}{
		{
			name: "space_as_separator_only_config",
			args: []string{
				configFileFlag, "cfg.yaml",
			},
			wantConfigFile: "cfg.yaml",
		},
		{
			name: "space_as_separator_all",
			args: []string{
				metricsPortFlag, "9999", configFileFlag, "cfg.yaml",
			},
			wantMetricsPort: "9999",
			wantConfigFile:  "cfg.yaml",
		},
		{
			name: "equal_as_separator_only_config",
			args: []string{
				configFileFlag + "=cfg.yaml",
			},
			wantConfigFile: "cfg.yaml",
		},
		{
			name: "equal_as_separator_all",
			args: []string{
				metricsPortFlag + "=9999", configFileFlag + "=cfg.yaml",
			},
			wantMetricsPort: "9999",
			wantConfigFile:  "cfg.yaml",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMetricsPort, gotConfigFile := parseArgs(tt.args)
			assert.Equal(t, tt.wantMetricsPort, gotMetricsPort)
			assert.Equal(t, tt.wantConfigFile, gotConfigFile)
		})
	}
}
