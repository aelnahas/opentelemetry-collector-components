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
	"crypto/rand"
	"encoding/hex"
	"io"

	"github.com/elastic/opentelemetry-collector-components/internal/elasticattr"
	"github.com/elastic/opentelemetry-collector-components/processor/elasticapmprocessor/internal/enrichments/attribute"
	"go.opentelemetry.io/collector/pdata/pcommon"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

// TranslateErrorAttributes sets ECS error attributes on the given attributes when they
// represent an error (caller must ensure IsErrorEvent(attributes) is true).
// It only sets attributes that are not already present.
// Resource is reserved for future use (e.g. service language for grouping key).
// logMessage, when non-empty, is used for error.grouping_name; otherwise exception message is used
// (e.g. pass log record body when translating logs, empty for span events).
func TranslateErrorAttributes(attributes pcommon.Map, resource pcommon.Resource, logMessage string) {
	var exceptionType, exceptionMessage, exceptionStacktrace string
	var exceptionEscaped bool

	attributes.Range(func(k string, v pcommon.Value) bool {
		switch k {
		case string(semconv.ExceptionTypeKey):
			exceptionType = v.Str()
		case string(semconv.ExceptionMessageKey):
			exceptionMessage = v.Str()
		case string(semconv.ExceptionStacktraceKey):
			exceptionStacktrace = v.Str()
		case string(semconv.ExceptionEscapedKey):
			exceptionEscaped = v.Bool()
		}
		return true
	})

	if exceptionType == "" && exceptionMessage == "" {
		return
	}

	if _, exists := attributes.Get(elasticattr.ErrorID); !exists {
		if id, err := newUniqueID(); err == nil {
			attributes.PutStr(elasticattr.ErrorID, id)
		}
	}

	attribute.PutBool(attributes, elasticattr.ErrorExceptionHandled, !exceptionEscaped)

	attribute.PutStr(attributes, elasticattr.ErrorGroupingKey, getErrGroupingKey(exceptionType, exceptionStacktrace, exceptionMessage, logMessage))

	errorMessage := logMessage
	if errorMessage == "" {
		errorMessage = exceptionMessage
	}
	if errorMessage != "" {
		attribute.PutStr(attributes, elasticattr.ErrorMessage, errorMessage)
		attribute.PutStr(attributes, elasticattr.ErrorGroupingName, errorMessage)
	}

	if exceptionType != "" {
		attribute.PutStr(attributes, elasticattr.ErrorExceptionTypeKey, exceptionType)
	}
	if exceptionMessage != "" {
		attribute.PutStr(attributes, elasticattr.ErrorExceptionMessageKey, exceptionMessage)
	}
}

// getErrGroupingKey returns an MD5 hash for error grouping, using the same order as apm-data SetGroupingKey:
// type, then stacktrace, then exception message (only if neither type nor stacktrace was hashed), then log message (only if nothing else was hashed).
// https://github.com/elastic/apm-data/blob/main/model/modelprocessor/groupingkey.go
func getErrGroupingKey(exceptionType, exceptionStacktrace, exceptionMessage, logMessage string) string {
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

// newUniqueID returns a 32-character hex string (same algorithm as apm-data).
func newUniqueID() (string, error) {
	var u [16]byte
	if _, err := io.ReadFull(rand.Reader, u[:]); err != nil {
		return "", err
	}
	buf := make([]byte, 32)
	hex.Encode(buf, u[:])
	return string(buf), nil
}
