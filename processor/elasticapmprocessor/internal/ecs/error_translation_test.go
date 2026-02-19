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

package ecs

import (
	"crypto/md5"
	"encoding/hex"
	"io"
	"regexp"
	"testing"

	"github.com/elastic/opentelemetry-collector-components/internal/elasticattr"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

// expectedGroupingKey mirrors getErrGroupingKey logic for test assertions.
func expectedGroupingKey(exceptionType, exceptionStacktrace, exceptionMessage, logMessage string) string {
	h := md5.New()
	if exceptionType != "" {
		io.WriteString(h, exceptionType)
	}
	if exceptionStacktrace != "" {
		io.WriteString(h, exceptionStacktrace)
	}
	if exceptionType == "" && exceptionStacktrace == "" && exceptionMessage != "" {
		io.WriteString(h, exceptionMessage)
	}
	if exceptionType == "" && exceptionStacktrace == "" && exceptionMessage == "" && logMessage != "" {
		io.WriteString(h, logMessage)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestTranslateErrorAttributes(t *testing.T) {
	tests := []struct {
		name        string
		logMessage  string
		setupAttrs  func(pcommon.Map)
		wantPresent map[string]func(t *testing.T, v pcommon.Value)
		wantAbsent  []string
	}{
		{
			name:       "empty attributes - no exception type or message",
			logMessage: "",
			setupAttrs: func(attrs pcommon.Map) {
				// no exception.* set
			},
			wantAbsent: []string{
				elasticattr.ErrorID,
				elasticattr.ErrorExceptionHandled,
				elasticattr.ErrorGroupingKey,
				elasticattr.ErrorGroupingName,
			},
		},
		{
			name:       "exception type only",
			logMessage: "",
			setupAttrs: func(attrs pcommon.Map) {
				attrs.PutStr(string(semconv.ExceptionTypeKey), "MyError")
			},
			wantPresent: map[string]func(*testing.T, pcommon.Value){
				elasticattr.ErrorID: func(t *testing.T, v pcommon.Value) {
					s := v.Str()
					if len(s) != 32 {
						t.Errorf("error.id length = %d, want 32", len(s))
						return
					}
					matched, _ := regexp.MatchString(`^[0-9a-f]+$`, s)
					if !matched {
						t.Errorf("error.id %q is not 32-char hex", s)
					}
				},
				elasticattr.ErrorExceptionHandled: func(t *testing.T, v pcommon.Value) {
					if !v.Bool() {
						t.Error("error.exception.handled = false, want true (escaped default false)")
					}
				},
				elasticattr.ErrorGroupingKey: func(t *testing.T, v pcommon.Value) {
					want := expectedGroupingKey("MyError", "", "", "")
					if v.Str() != want {
						t.Errorf("error.grouping_key = %q, want %q", v.Str(), want)
					}
				},
			},
			wantAbsent: []string{elasticattr.ErrorGroupingName},
		},
		{
			name:       "exception message only - grouping key from message",
			logMessage: "",
			setupAttrs: func(attrs pcommon.Map) {
				attrs.PutStr(string(semconv.ExceptionMessageKey), "something broke")
			},
			wantPresent: map[string]func(*testing.T, pcommon.Value){
				elasticattr.ErrorID:               nil,
				elasticattr.ErrorExceptionHandled: nil,
				elasticattr.ErrorGroupingKey: func(t *testing.T, v pcommon.Value) {
					want := expectedGroupingKey("", "", "something broke", "")
					if v.Str() != want {
						t.Errorf("error.grouping_key = %q, want %q", v.Str(), want)
					}
				},
				elasticattr.ErrorGroupingName: func(t *testing.T, v pcommon.Value) {
					if v.Str() != "something broke" {
						t.Errorf("error.grouping_name = %q, want %q", v.Str(), "something broke")
					}
				},
			},
		},
		{
			name:       "exception type and message - grouping key from type, grouping_name from message",
			logMessage: "",
			setupAttrs: func(attrs pcommon.Map) {
				attrs.PutStr(string(semconv.ExceptionTypeKey), "TypeError")
				attrs.PutStr(string(semconv.ExceptionMessageKey), "msg")
			},
			wantPresent: map[string]func(*testing.T, pcommon.Value){
				elasticattr.ErrorGroupingKey: func(t *testing.T, v pcommon.Value) {
					want := expectedGroupingKey("TypeError", "", "", "")
					if v.Str() != want {
						t.Errorf("error.grouping_key = %q, want %q", v.Str(), want)
					}
				},
				elasticattr.ErrorGroupingName: func(t *testing.T, v pcommon.Value) {
					if v.Str() != "msg" {
						t.Errorf("error.grouping_name = %q, want msg", v.Str())
					}
				},
			},
		},
		{
			name:       "exception type, stacktrace, and message - grouping key from type then stacktrace",
			logMessage: "",
			setupAttrs: func(attrs pcommon.Map) {
				attrs.PutStr(string(semconv.ExceptionTypeKey), "Err")
				attrs.PutStr(string(semconv.ExceptionStacktraceKey), "at main.go:1")
				attrs.PutStr(string(semconv.ExceptionMessageKey), "failed")
			},
			wantPresent: map[string]func(*testing.T, pcommon.Value){
				elasticattr.ErrorGroupingKey: func(t *testing.T, v pcommon.Value) {
					want := expectedGroupingKey("Err", "at main.go:1", "", "")
					if v.Str() != want {
						t.Errorf("error.grouping_key = %q, want %q", v.Str(), want)
					}
				},
				elasticattr.ErrorGroupingName: func(t *testing.T, v pcommon.Value) {
					if v.Str() != "failed" {
						t.Errorf("error.grouping_name = %q, want failed", v.Str())
					}
				},
			},
		},
		{
			name:       "exception.escaped true - error.exception.handled false",
			logMessage: "",
			setupAttrs: func(attrs pcommon.Map) {
				attrs.PutStr(string(semconv.ExceptionTypeKey), "E")
				attrs.PutBool(string(semconv.ExceptionEscapedKey), true)
			},
			wantPresent: map[string]func(*testing.T, pcommon.Value){
				elasticattr.ErrorExceptionHandled: func(t *testing.T, v pcommon.Value) {
					if v.Bool() {
						t.Error("error.exception.handled = true, want false when exception.escaped true")
					}
				},
			},
		},
		{
			name:       "exception.escaped false - error.exception.handled true",
			logMessage: "",
			setupAttrs: func(attrs pcommon.Map) {
				attrs.PutStr(string(semconv.ExceptionTypeKey), "E")
				attrs.PutBool(string(semconv.ExceptionEscapedKey), false)
			},
			wantPresent: map[string]func(*testing.T, pcommon.Value){
				elasticattr.ErrorExceptionHandled: func(t *testing.T, v pcommon.Value) {
					if !v.Bool() {
						t.Error("error.exception.handled = false, want true when exception.escaped false")
					}
				},
			},
		},
		{
			name:       "existing error.id not overwritten",
			logMessage: "",
			setupAttrs: func(attrs pcommon.Map) {
				attrs.PutStr(string(semconv.ExceptionTypeKey), "E")
				attrs.PutStr(elasticattr.ErrorID, "existing-id-0000000000000000")
			},
			wantPresent: map[string]func(*testing.T, pcommon.Value){
				elasticattr.ErrorID: func(t *testing.T, v pcommon.Value) {
					if v.Str() != "existing-id-0000000000000000" {
						t.Errorf("error.id = %q, want existing-id-0000000000000000 (not overwritten)", v.Str())
					}
				},
			},
		},
		{
			name:       "existing error.grouping_key not overwritten",
			logMessage: "",
			setupAttrs: func(attrs pcommon.Map) {
				attrs.PutStr(string(semconv.ExceptionTypeKey), "E")
				attrs.PutStr(elasticattr.ErrorGroupingKey, "existing-grouping-key")
			},
			wantPresent: map[string]func(*testing.T, pcommon.Value){
				elasticattr.ErrorGroupingKey: func(t *testing.T, v pcommon.Value) {
					if v.Str() != "existing-grouping-key" {
						t.Errorf("error.grouping_key = %q, want existing-grouping-key (not overwritten)", v.Str())
					}
				},
			},
		},
		{
			name:       "existing error.exception.handled not overwritten",
			logMessage: "",
			setupAttrs: func(attrs pcommon.Map) {
				attrs.PutStr(string(semconv.ExceptionTypeKey), "E")
				attrs.PutBool(elasticattr.ErrorExceptionHandled, false)
			},
			wantPresent: map[string]func(*testing.T, pcommon.Value){
				elasticattr.ErrorExceptionHandled: func(t *testing.T, v pcommon.Value) {
					if v.Bool() {
						t.Error("error.exception.handled = true, want false (existing value preserved)")
					}
				},
			},
		},
		{
			name:       "log message overrides exception message for grouping_name (MIS behavior)",
			logMessage: "log body message",
			setupAttrs: func(attrs pcommon.Map) {
				attrs.PutStr(string(semconv.ExceptionTypeKey), "ErrType")
				attrs.PutStr(string(semconv.ExceptionMessageKey), "exception message")
			},
			wantPresent: map[string]func(*testing.T, pcommon.Value){
				elasticattr.ErrorGroupingName: func(t *testing.T, v pcommon.Value) {
					if v.Str() != "log body message" {
						t.Errorf("error.grouping_name = %q, want log body message (log overrides exception)", v.Str())
					}
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attrs := pcommon.NewMap()
			resource := pcommon.NewResource()
			tt.setupAttrs(attrs)
			TranslateErrorAttributes(attrs, resource, tt.logMessage)
			for key, validate := range tt.wantPresent {
				v, ok := attrs.Get(key)
				if !ok {
					t.Errorf("expected attribute %q to be present", key)
					continue
				}
				if validate != nil {
					validate(t, v)
				}
			}
			for _, key := range tt.wantAbsent {
				if _, ok := attrs.Get(key); ok {
					t.Errorf("expected attribute %q to be absent", key)
				}
			}
		})
	}
}
