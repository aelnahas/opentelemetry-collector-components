// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package config

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnabled(t *testing.T) {
	config := Enabled()
	assertAttributeConfigDefaults(t, reflect.ValueOf(config.Resource), nil)
	assertAttributeConfigDefaults(t, reflect.ValueOf(config.Scope), nil)
	assertAttributeConfigDefaults(t, reflect.ValueOf(config.Transaction), nil)
	assertAttributeConfigDefaults(t, reflect.ValueOf(config.Span), nil)
	assertAttributeConfigDefaults(t, reflect.ValueOf(config.SpanEvent), nil)
	assertAttributeConfigDefaults(t, reflect.ValueOf(config.Log), []string{"ProcessorEvent"})
}

func assertAttributeConfigDefaults(t *testing.T, cfg reflect.Value, expectDisabled []string) {
	t.Helper()

	disabled := make(map[string]bool)
	for _, name := range expectDisabled {
		disabled[name] = true
	}

	// Fields that are intentionally disabled by default
	disabledByDefault := map[string]bool{
		"ClearSpanID":   true,
		"ClearSpanName": true,
	}

	for i := 0; i < cfg.NumField(); i++ {
		fieldName := cfg.Type().Field(i).Name
		rAttrCfg := cfg.Field(i).Interface()
		attrCfg, ok := rAttrCfg.(AttributeConfig)
		require.True(t, ok, "must be a type of AttributeConfig")

		if disabled[fieldName] {
			require.False(t, attrCfg.Enabled, "%s must be disabled", fieldName)
			continue
		}
		if disabledByDefault[fieldName] {
			require.False(t, attrCfg.Enabled, "%s must be disabled by default", fieldName)
		} else {
			require.True(t, attrCfg.Enabled, "%s must be enabled", fieldName)
		}
	}
}
