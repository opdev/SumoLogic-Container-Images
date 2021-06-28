// Copyright The OpenTelemetry Authors
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

package datadogexporter

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/DataDog/datadog-agent/pkg/trace/exportable/obfuscate"
	"github.com/DataDog/datadog-agent/pkg/trace/exportable/pb"
	"github.com/DataDog/datadog-agent/pkg/trace/exportable/stats"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/ext"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter/utils"
)

func NewResourceSpansData(mockTraceID [16]byte, mockSpanID [8]byte, mockParentSpanID [8]byte, statusCode pdata.StatusCode, resourceEnvAndService bool, endTime time.Time) pdata.ResourceSpans {
	// The goal of this test is to ensure that each span in
	// pdata.ResourceSpans is transformed to its *trace.SpanData correctly!

	pdataEndTime := pdata.TimestampFromTime(endTime)
	startTime := endTime.Add(-90 * time.Second)
	pdataStartTime := pdata.TimestampFromTime(startTime)

	rs := pdata.NewResourceSpans()
	ilss := rs.InstrumentationLibrarySpans()
	ilss.Resize(1)
	il := ilss.At(0).InstrumentationLibrary()
	il.SetName("test_il_name")
	il.SetVersion("test_il_version")
	spans := ilss.At(0).Spans()
	spans.Resize(1)

	span := spans.At(0)

	traceID := pdata.NewTraceID(mockTraceID)
	spanID := pdata.NewSpanID(mockSpanID)
	parentSpanID := pdata.NewSpanID(mockParentSpanID)
	span.SetTraceID(traceID)
	span.SetSpanID(spanID)
	span.SetParentSpanID(parentSpanID)
	span.SetName("End-To-End Here")
	span.SetKind(pdata.SpanKindSERVER)
	span.SetStartTimestamp(pdataStartTime)
	span.SetEndTimestamp(pdataEndTime)
	span.SetTraceState("tracestatekey=tracestatevalue")

	status := span.Status()
	if statusCode == pdata.StatusCodeError {
		status.SetCode(pdata.StatusCodeError)
		status.SetMessage("This is not a drill!")
	} else {
		status.SetCode(statusCode)
	}

	events := span.Events()
	events.Resize(2)

	events.At(0).SetTimestamp(pdataStartTime)
	events.At(0).SetName("start")

	events.At(1).SetTimestamp(pdataEndTime)
	events.At(1).SetName("end")
	events.At(1).Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"flag": pdata.NewAttributeValueBool(false),
	})

	attribs := map[string]pdata.AttributeValue{
		"cache_hit":  pdata.NewAttributeValueBool(true),
		"timeout_ns": pdata.NewAttributeValueInt(12e9),
		"ping_count": pdata.NewAttributeValueInt(25),
		"agent":      pdata.NewAttributeValueString("ocagent"),
	}

	if statusCode == pdata.StatusCodeError {
		attribs["http.status_code"] = pdata.NewAttributeValueString("501")
	}

	span.Attributes().InitFromMap(attribs)

	resource := rs.Resource()

	if resourceEnvAndService {
		resource.Attributes().InitFromMap(map[string]pdata.AttributeValue{
			conventions.AttributeContainerID: pdata.NewAttributeValueString("3249847017410247"),
			conventions.AttributeK8sPod:      pdata.NewAttributeValueString("example-pod-name"),
			"namespace":                      pdata.NewAttributeValueString("kube-system"),
			"service.name":                   pdata.NewAttributeValueString("test-resource-service-name"),
			"deployment.environment":         pdata.NewAttributeValueString("test-env"),
			"service.version":                pdata.NewAttributeValueString("test-version"),
		})

	} else {
		resource.Attributes().InitFromMap(map[string]pdata.AttributeValue{
			"namespace": pdata.NewAttributeValueString("kube-system"),
		})
	}

	return rs
}

func newSublayerCalculator() *sublayerCalculator {
	return &sublayerCalculator{sc: stats.NewSublayerCalculator()}
}

func TestConvertToDatadogTd(t *testing.T) {
	traces := pdata.NewTraces()
	traces.ResourceSpans().Resize(1)
	calculator := newSublayerCalculator()

	outputTraces, runningMetrics := convertToDatadogTd(traces, calculator, &config.Config{})
	assert.Equal(t, 1, len(outputTraces))
	assert.Equal(t, 1, len(runningMetrics))
}

func TestConvertToDatadogTdNoResourceSpans(t *testing.T) {
	traces := pdata.NewTraces()
	calculator := newSublayerCalculator()

	outputTraces, runningMetrics := convertToDatadogTd(traces, calculator, &config.Config{})
	assert.Equal(t, 0, len(outputTraces))
	assert.Equal(t, 0, len(runningMetrics))
}

func TestRunningTraces(t *testing.T) {
	td := pdata.NewTraces()

	rts := td.ResourceSpans()
	rts.Resize(4)

	rt := rts.At(0)
	resAttrs := rt.Resource().Attributes()
	resAttrs.Insert(metadata.AttributeDatadogHostname, pdata.NewAttributeValueString("resource-hostname-1"))

	rt = rts.At(1)
	resAttrs = rt.Resource().Attributes()
	resAttrs.Insert(metadata.AttributeDatadogHostname, pdata.NewAttributeValueString("resource-hostname-1"))

	rt = rts.At(2)
	resAttrs = rt.Resource().Attributes()
	resAttrs.Insert(metadata.AttributeDatadogHostname, pdata.NewAttributeValueString("resource-hostname-2"))

	_, runningMetrics := convertToDatadogTd(td, newSublayerCalculator(), &config.Config{})

	runningHostnames := []string{}
	for _, metric := range runningMetrics {
		require.Equal(t, *metric.Metric, "datadog_exporter.traces.running")
		require.NotNil(t, metric.Host)
		runningHostnames = append(runningHostnames, *metric.Host)
	}

	defaultHostname := *metadata.GetHost(zap.NewNop(), &config.Config{})

	assert.ElementsMatch(t,
		runningHostnames,
		[]string{"resource-hostname-1", "resource-hostname-1", "resource-hostname-2", defaultHostname},
	)
}

func TestObfuscation(t *testing.T) {
	calculator := newSublayerCalculator()

	traces := pdata.NewTraces()
	traces.ResourceSpans().Resize(1)
	rs := traces.ResourceSpans().At(0)
	resource := rs.Resource()
	resource.Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"service.name": pdata.NewAttributeValueString("sure"),
	})
	rs.InstrumentationLibrarySpans().Resize(1)
	ilss := rs.InstrumentationLibrarySpans().At(0)
	instrumentationLibrary := ilss.InstrumentationLibrary()
	instrumentationLibrary.SetName("flash")
	instrumentationLibrary.SetVersion("v1")

	ilss.Spans().Resize(1)
	span := ilss.Spans().At(0)

	// Make this a FaaS span, which will trigger an error, because conversion
	// of them is currently not supported.
	span.Attributes().InsertString("testinfo?=123", "http.route")

	outputTraces, _ := convertToDatadogTd(traces, calculator, &config.Config{})
	aggregatedTraces := aggregateTracePayloadsByEnv(outputTraces)

	obfuscator := obfuscate.NewObfuscator(obfuscatorConfig)

	obfuscatePayload(obfuscator, aggregatedTraces)

	assert.Equal(t, 1, len(aggregatedTraces))
}

func TestBasicTracesTranslation(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	// generate mock trace, span and parent span ids
	mockTraceID := [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	mockSpanID := [8]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}
	mockParentSpanID := [8]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8}
	mockEndTime := time.Now().Round(time.Second)

	// create mock resource span data
	// set shouldError and resourceServiceandEnv to false to test defaut behavior
	rs := NewResourceSpansData(mockTraceID, mockSpanID, mockParentSpanID, pdata.StatusCodeUnset, false, mockEndTime)

	// translate mocks to datadog traces
	datadogPayload := resourceSpansToDatadogSpans(rs, calculator, hostname, &config.Config{})
	// ensure we return the correct type
	assert.IsType(t, pb.TracePayload{}, datadogPayload)

	// ensure hostname arg is respected
	assert.Equal(t, hostname, datadogPayload.HostName)
	assert.Equal(t, 1, len(datadogPayload.Traces))

	// ensure trace id gets translated to uint64 correctly
	assert.Equal(t, decodeAPMTraceID(mockTraceID), datadogPayload.Traces[0].TraceID)

	// ensure the correct number of spans are expected
	assert.Equal(t, 1, len(datadogPayload.Traces[0].Spans))

	// ensure span's trace id matches payload trace id
	assert.Equal(t, datadogPayload.Traces[0].TraceID, datadogPayload.Traces[0].Spans[0].TraceID)

	// ensure span's spanId and parentSpanId are set correctly
	assert.Equal(t, decodeAPMSpanID(mockSpanID), datadogPayload.Traces[0].Spans[0].SpanID)
	assert.Equal(t, decodeAPMSpanID(mockParentSpanID), datadogPayload.Traces[0].Spans[0].ParentID)

	// ensure that span.resource defaults to otlp span.name
	assert.Equal(t, "End-To-End Here", datadogPayload.Traces[0].Spans[0].Resource)

	// ensure that span.name defaults to string representing instrumentation library if present
	assert.Equal(t, strings.ToLower(fmt.Sprintf("%s.%s", datadogPayload.Traces[0].Spans[0].Meta[conventions.InstrumentationLibraryName], strings.TrimPrefix(pdata.SpanKindSERVER.String(), "SPAN_KIND_"))), datadogPayload.Traces[0].Spans[0].Name)

	// ensure that span.type is based on otlp span.kind
	assert.Equal(t, "web", datadogPayload.Traces[0].Spans[0].Type)

	// ensure that span.meta and span.metrics pick up attibutes, instrumentation ibrary and resource attribs
	assert.Equal(t, 10, len(datadogPayload.Traces[0].Spans[0].Meta))
	assert.Equal(t, 1, len(datadogPayload.Traces[0].Spans[0].Metrics))

	// ensure that span error is based on otlp span status
	assert.Equal(t, int32(0), datadogPayload.Traces[0].Spans[0].Error)

	// ensure that span meta also inccludes correctly sets resource attributes
	assert.Equal(t, "kube-system", datadogPayload.Traces[0].Spans[0].Meta["namespace"])

	// ensure that span service name gives resource service.name priority
	assert.Equal(t, "otlpresourcenoservicename", datadogPayload.Traces[0].Spans[0].Service)

	// ensure a duration and start time are calculated
	assert.NotNil(t, datadogPayload.Traces[0].Spans[0].Start)
	assert.NotNil(t, datadogPayload.Traces[0].Spans[0].Duration)

	pdataMockEndTime := pdata.TimestampFromTime(mockEndTime)
	pdataMockStartTime := pdata.TimestampFromTime(mockEndTime.Add(-90 * time.Second))
	mockEventsString := fmt.Sprintf("[{\"attributes\":{},\"name\":\"start\",\"time\":%d},{\"attributes\":{\"flag\":false},\"name\":\"end\",\"time\":%d}]", pdataMockStartTime, pdataMockEndTime)

	// ensure that events tag is set if span events exist and contains structured json fields
	assert.Equal(t, mockEventsString, datadogPayload.Traces[0].Spans[0].Meta["events"])
}

func TestTracesTranslationErrorsAndResource(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	// generate mock trace, span and parent span ids
	mockTraceID := [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	mockSpanID := [8]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}
	mockParentSpanID := [8]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8}

	mockEndTime := time.Now().Round(time.Second)

	// create mock resource span data
	// toggle on errors and custom service naming to test edge case code paths
	rs := NewResourceSpansData(mockTraceID, mockSpanID, mockParentSpanID, pdata.StatusCodeError, true, mockEndTime)

	// translate mocks to datadog traces
	cfg := config.Config{
		TagsConfig: config.TagsConfig{
			Version: "v1",
		},
	}

	datadogPayload := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfg)
	// ensure we return the correct type
	assert.IsType(t, pb.TracePayload{}, datadogPayload)

	// ensure hostname arg is respected
	assert.Equal(t, hostname, datadogPayload.HostName)
	assert.Equal(t, 1, len(datadogPayload.Traces))

	// ensure that span error is based on otlp span status
	assert.Equal(t, int32(1), datadogPayload.Traces[0].Spans[0].Error)

	// ensure that span status code is set
	assert.Equal(t, "501", datadogPayload.Traces[0].Spans[0].Meta["http.status_code"])

	// ensure that span service name gives resource service.name priority
	assert.Equal(t, "test-resource-service-name", datadogPayload.Traces[0].Spans[0].Service)

	// ensure that env gives resource deployment.environment priority
	assert.Equal(t, "test-env", datadogPayload.Env)

	// ensure that version gives resource service.version priority
	assert.Equal(t, "test-version", datadogPayload.Traces[0].Spans[0].Meta["version"])

	assert.Equal(t, 18, len(datadogPayload.Traces[0].Spans[0].Meta))

	assert.Contains(t, datadogPayload.Traces[0].Spans[0].Meta[tagContainersTags], "container_id:3249847017410247")
	assert.Contains(t, datadogPayload.Traces[0].Spans[0].Meta[tagContainersTags], "pod_name:example-pod-name")
}

// Ensures that if more than one error event occurs in a span, the last one is used for translation
func TestTracesTranslationErrorsFromEventsUsesLast(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	// generate mock trace, span and parent span ids
	mockTraceID := [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	mockSpanID := [8]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}
	mockParentSpanID := [8]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8}

	mockEndTime := time.Now().Round(time.Second)

	// create mock resource span data
	// toggle on errors and custom service naming to test edge case code paths
	rs := NewResourceSpansData(mockTraceID, mockSpanID, mockParentSpanID, pdata.StatusCodeError, true, mockEndTime)
	span := rs.InstrumentationLibrarySpans().At(0).Spans().At(0)
	events := span.Events()
	events.Resize(4)

	events.At(0).SetName("start")

	events.At(1).SetName(conventions.AttributeExceptionEventName)
	events.At(1).Attributes().InitFromMap(map[string]pdata.AttributeValue{
		conventions.AttributeExceptionType:       pdata.NewAttributeValueString("SomeOtherErr"),
		conventions.AttributeExceptionStacktrace: pdata.NewAttributeValueString("SomeOtherErr at line 67\nthing at line 45"),
		conventions.AttributeExceptionMessage:    pdata.NewAttributeValueString("SomeOtherErr error occurred"),
	})

	attribs := map[string]pdata.AttributeValue{
		conventions.AttributeExceptionType:       pdata.NewAttributeValueString("HttpError"),
		conventions.AttributeExceptionStacktrace: pdata.NewAttributeValueString("HttpError at line 67\nthing at line 45"),
		conventions.AttributeExceptionMessage:    pdata.NewAttributeValueString("HttpError error occurred"),
	}
	events.At(2).SetName(conventions.AttributeExceptionEventName)
	events.At(2).Attributes().InitFromMap(attribs)

	events.At(3).SetName("end")
	events.At(3).Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"flag": pdata.NewAttributeValueBool(false),
	})

	// translate mocks to datadog traces
	cfg := config.Config{
		TagsConfig: config.TagsConfig{
			Version: "v1",
		},
	}

	datadogPayload := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfg)

	// Ensure the error type is copied over from the last error event logged
	assert.Equal(t, attribs[conventions.AttributeExceptionType].StringVal(), datadogPayload.Traces[0].Spans[0].Meta[ext.ErrorType])

	// Ensure the stack trace is copied over from the last error event logged
	assert.Equal(t, attribs[conventions.AttributeExceptionStacktrace].StringVal(), datadogPayload.Traces[0].Spans[0].Meta[ext.ErrorStack])

	// Ensure the error message is copied over from the last error event logged
	assert.Equal(t, attribs[conventions.AttributeExceptionMessage].StringVal(), datadogPayload.Traces[0].Spans[0].Meta[ext.ErrorMsg])
}

// Ensures that if the first or last event in the list is the error, that translation still behaves properly
func TestTracesTranslationErrorsFromEventsBounds(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	// generate mock trace, span and parent span ids
	mockTraceID := [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	mockSpanID := [8]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}
	mockParentSpanID := [8]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8}

	mockEndTime := time.Now().Round(time.Second)

	// create mock resource span data
	// toggle on errors and custom service naming to test edge case code paths
	rs := NewResourceSpansData(mockTraceID, mockSpanID, mockParentSpanID, pdata.StatusCodeError, true, mockEndTime)
	span := rs.InstrumentationLibrarySpans().At(0).Spans().At(0)
	events := span.Events()
	events.Resize(3)

	// Start with the error as the first element in the list...
	attribs := map[string]pdata.AttributeValue{
		conventions.AttributeExceptionType:       pdata.NewAttributeValueString("HttpError"),
		conventions.AttributeExceptionStacktrace: pdata.NewAttributeValueString("HttpError at line 67\nthing at line 45"),
		conventions.AttributeExceptionMessage:    pdata.NewAttributeValueString("HttpError error occurred"),
	}
	events.At(0).SetName(conventions.AttributeExceptionEventName)
	events.At(0).Attributes().InitFromMap(attribs)

	events.At(1).SetName("start")

	events.At(2).SetName("end")
	events.At(2).Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"flag": pdata.NewAttributeValueBool(false),
	})

	// translate mocks to datadog traces
	cfg := config.Config{
		TagsConfig: config.TagsConfig{
			Version: "v1",
		},
	}

	datadogPayload := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfg)

	// Ensure the error type is copied over
	assert.Equal(t, attribs[conventions.AttributeExceptionType].StringVal(), datadogPayload.Traces[0].Spans[0].Meta[ext.ErrorType])

	// Ensure the stack trace is copied over
	assert.Equal(t, attribs[conventions.AttributeExceptionStacktrace].StringVal(), datadogPayload.Traces[0].Spans[0].Meta[ext.ErrorStack])

	// Ensure the error message is copied over
	assert.Equal(t, attribs[conventions.AttributeExceptionMessage].StringVal(), datadogPayload.Traces[0].Spans[0].Meta[ext.ErrorMsg])

	// Now with the error event at the end of the list...
	events.At(0).SetName("start")
	// Reset the attributes
	events.At(0).Attributes().InitFromMap(map[string]pdata.AttributeValue{})

	events.At(1).SetName("end")
	events.At(1).Attributes().InitFromMap(map[string]pdata.AttributeValue{
		"flag": pdata.NewAttributeValueBool(false),
	})

	events.At(2).SetName(conventions.AttributeExceptionEventName)
	events.At(2).Attributes().InitFromMap(attribs)

	// Ensure the error type is copied over
	assert.Equal(t, attribs[conventions.AttributeExceptionType].StringVal(), datadogPayload.Traces[0].Spans[0].Meta[ext.ErrorType])

	// Ensure the stack trace is copied over
	assert.Equal(t, attribs[conventions.AttributeExceptionStacktrace].StringVal(), datadogPayload.Traces[0].Spans[0].Meta[ext.ErrorStack])

	// Ensure the error message is copied over
	assert.Equal(t, attribs[conventions.AttributeExceptionMessage].StringVal(), datadogPayload.Traces[0].Spans[0].Meta[ext.ErrorMsg])
}

func TestTracesTranslationOkStatus(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	// generate mock trace, span and parent span ids
	mockTraceID := [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	mockSpanID := [8]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}
	mockParentSpanID := [8]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8}

	mockEndTime := time.Now().Round(time.Second)

	// create mock resource span data
	// toggle on errors and custom service naming to test edge case code paths
	rs := NewResourceSpansData(mockTraceID, mockSpanID, mockParentSpanID, pdata.StatusCodeError, true, mockEndTime)

	// translate mocks to datadog traces
	cfg := config.Config{
		TagsConfig: config.TagsConfig{
			Version: "v1",
		},
	}

	datadogPayload := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfg)
	// ensure we return the correct type
	assert.IsType(t, pb.TracePayload{}, datadogPayload)

	// ensure hostname arg is respected
	assert.Equal(t, hostname, datadogPayload.HostName)
	assert.Equal(t, 1, len(datadogPayload.Traces))

	// ensure that span error is based on otlp span status
	assert.Equal(t, int32(1), datadogPayload.Traces[0].Spans[0].Error)

	// ensure that span status code is set
	assert.Equal(t, "501", datadogPayload.Traces[0].Spans[0].Meta["http.status_code"])

	// ensure that span service name gives resource service.name priority
	assert.Equal(t, "test-resource-service-name", datadogPayload.Traces[0].Spans[0].Service)

	// ensure that env gives resource deployment.environment priority
	assert.Equal(t, "test-env", datadogPayload.Env)

	// ensure that version gives resource service.version priority
	assert.Equal(t, "test-version", datadogPayload.Traces[0].Spans[0].Meta["version"])

	assert.Equal(t, 18, len(datadogPayload.Traces[0].Spans[0].Meta))
}

// ensure that the datadog span uses the configured unified service tags
func TestTracesTranslationConfig(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	// generate mock trace, span and parent span ids
	mockTraceID := [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	mockSpanID := [8]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}
	mockParentSpanID := [8]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8}

	mockEndTime := time.Now().Round(time.Second)

	// create mock resource span data
	// toggle on errors and custom service naming to test edge case code paths
	rs := NewResourceSpansData(mockTraceID, mockSpanID, mockParentSpanID, pdata.StatusCodeUnset, true, mockEndTime)

	cfg := config.Config{
		TagsConfig: config.TagsConfig{
			Version: "v1",
			Service: "alt-service",
		},
	}

	// translate mocks to datadog traces
	datadogPayload := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfg)
	// ensure we return the correct type
	assert.IsType(t, pb.TracePayload{}, datadogPayload)

	// ensure hostname arg is respected
	assert.Equal(t, hostname, datadogPayload.HostName)
	assert.Equal(t, 1, len(datadogPayload.Traces))

	// ensure that span error is based on otlp span status, unset should not error
	assert.Equal(t, int32(0), datadogPayload.Traces[0].Spans[0].Error)

	// ensure that span service name gives resource service.name priority
	assert.Equal(t, "test-resource-service-name", datadogPayload.Traces[0].Spans[0].Service)

	// ensure that env gives resource deployment.environment priority
	assert.Equal(t, "test-env", datadogPayload.Env)

	// ensure that version gives resource service.version priority
	assert.Equal(t, "test-version", datadogPayload.Traces[0].Spans[0].Meta["version"])

	assert.Equal(t, 15, len(datadogPayload.Traces[0].Spans[0].Meta))
}

// ensure that the translation returns early if no resource instrumentation library spans
func TestTracesTranslationNoIls(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	rs := pdata.NewResourceSpans()

	cfg := config.Config{
		TagsConfig: config.TagsConfig{
			Version: "v1",
			Service: "alt-service",
		},
	}

	// translate mocks to datadog traces
	datadogPayload := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfg)
	// ensure we return the correct type
	assert.IsType(t, pb.TracePayload{}, datadogPayload)

	// ensure hostname arg is respected
	assert.Equal(t, hostname, datadogPayload.HostName)
	assert.Equal(t, 0, len(datadogPayload.Traces))
}

// ensure that the translation returns early if no resource instrumentation library spans
func TestTracesTranslationInvalidService(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	// generate mock trace, span and parent span ids
	mockTraceID := [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	mockSpanID := [8]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}
	mockParentSpanID := [8]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8}

	mockEndTime := time.Now().Round(time.Second)

	// create mock resource span data
	// toggle on errors and custom service naming to test edge case code paths
	rs := NewResourceSpansData(mockTraceID, mockSpanID, mockParentSpanID, pdata.StatusCodeUnset, false, mockEndTime)

	// add a tab and an invalid character to see if it gets normalized
	cfgInvalidService := config.Config{
		TagsConfig: config.TagsConfig{
			Version: "v1",
			Service: "alt-s	ervice",
		},
	}

	// use only an invalid character
	cfgEmptyService := config.Config{
		TagsConfig: config.TagsConfig{
			Version: "v1",
			Service: "	",
		},
	}

	// start with an invalid character
	cfgStartWithInvalidService := config.Config{
		TagsConfig: config.TagsConfig{
			Version: "v1",
			Service: "	alt-service",
		},
	}

	// translate mocks to datadog traces
	datadogPayloadInvalidService := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfgInvalidService)
	datadogPayloadEmptyService := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfgEmptyService)
	datadogPayloadStartWithInvalidService := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfgStartWithInvalidService)

	// ensure we return the correct type
	assert.IsType(t, pb.TracePayload{}, datadogPayloadInvalidService)
	assert.IsType(t, pb.TracePayload{}, datadogPayloadEmptyService)
	assert.IsType(t, pb.TracePayload{}, datadogPayloadStartWithInvalidService)

	// ensure that span service name replaces invalid chars
	assert.Equal(t, "alt-s_ervice", datadogPayloadInvalidService.Traces[0].Spans[0].Service)
	// ensure that span service name has default
	assert.Equal(t, "unnamed-otel-service", datadogPayloadEmptyService.Traces[0].Spans[0].Service)
	// ensure that span service name removes invalid starting chars
	assert.Equal(t, "alt-service", datadogPayloadStartWithInvalidService.Traces[0].Spans[0].Service)
}

// ensure that datadog span resource naming uses http method+route when available
func TestSpanResourceTranslation(t *testing.T) {
	span := pdata.NewSpan()
	span.SetKind(pdata.SpanKindSERVER)
	span.SetName("Default Name")

	ddHTTPTags := map[string]string{
		"http.method": "GET",
		"http.route":  "/api",
	}

	ddNotHTTPTags := map[string]string{
		"other": "GET",
	}

	resourceNameHTTP := getDatadogResourceName(span, ddHTTPTags)

	resourceNameDefault := getDatadogResourceName(span, ddNotHTTPTags)

	assert.Equal(t, "GET /api", resourceNameHTTP)
	assert.Equal(t, "Default Name", resourceNameDefault)
}

// ensure that datadog span resource naming uses http method+ grpc path when available
func TestSpanResourceTranslationGRPC(t *testing.T) {
	span := pdata.NewSpan()
	span.SetKind(pdata.SpanKindSERVER)
	span.SetName("Default Name")

	ddHTTPTags := map[string]string{
		"http.method": "POST",
		"grpc.path":   "/api",
	}

	ddNotHTTPTags := map[string]string{
		"other": "GET",
	}

	resourceNameHTTP := getDatadogResourceName(span, ddHTTPTags)

	resourceNameDefault := getDatadogResourceName(span, ddNotHTTPTags)

	assert.Equal(t, "POST /api", resourceNameHTTP)
	assert.Equal(t, "Default Name", resourceNameDefault)
}

// ensure that datadog span resource naming uses messaging operation+destination when available
func TestSpanResourceTranslationMessaging(t *testing.T) {
	span := pdata.NewSpan()
	span.SetKind(pdata.SpanKindSERVER)
	span.SetName("Default Name")

	ddHTTPTags := map[string]string{
		"messaging.operation":   "receive",
		"messaging.destination": "example.topic",
	}

	ddNotHTTPTags := map[string]string{
		"other": "GET",
	}

	resourceNameHTTP := getDatadogResourceName(span, ddHTTPTags)

	resourceNameDefault := getDatadogResourceName(span, ddNotHTTPTags)

	assert.Equal(t, "receive example.topic", resourceNameHTTP)
	assert.Equal(t, "Default Name", resourceNameDefault)
}

// ensure that datadog span resource naming uses messaging operation even when destination is not available
func TestSpanResourceTranslationMessagingFallback(t *testing.T) {
	span := pdata.NewSpan()
	span.SetKind(pdata.SpanKindSERVER)
	span.SetName("Default Name")

	ddHTTPTags := map[string]string{
		"messaging.operation": "receive",
	}

	ddNotHTTPTags := map[string]string{
		"other": "GET",
	}

	resourceNameHTTP := getDatadogResourceName(span, ddHTTPTags)

	resourceNameDefault := getDatadogResourceName(span, ddNotHTTPTags)

	assert.Equal(t, "receive", resourceNameHTTP)
	assert.Equal(t, "Default Name", resourceNameDefault)
}

// ensure that datadog span resource naming uses rpc method + rpc service when available
func TestSpanResourceTranslationRpc(t *testing.T) {
	span := pdata.NewSpan()
	span.SetKind(pdata.SpanKindSERVER)
	span.SetName("Default Name")

	ddHTTPTags := map[string]string{
		"rpc.method":  "example_method",
		"rpc.service": "example_service",
	}

	ddNotHTTPTags := map[string]string{
		"other": "GET",
	}

	resourceNameHTTP := getDatadogResourceName(span, ddHTTPTags)

	resourceNameDefault := getDatadogResourceName(span, ddNotHTTPTags)

	assert.Equal(t, "example_method example_service", resourceNameHTTP)
	assert.Equal(t, "Default Name", resourceNameDefault)
}

// ensure that datadog span resource naming uses rpc method even when rpc service is not available
func TestSpanResourceTranslationRpcFallback(t *testing.T) {
	span := pdata.NewSpan()
	span.SetKind(pdata.SpanKindSERVER)
	span.SetName("Default Name")

	ddHTTPTags := map[string]string{
		"rpc.method": "example_method",
	}

	ddNotHTTPTags := map[string]string{
		"other": "GET",
	}

	resourceNameHTTP := getDatadogResourceName(span, ddHTTPTags)

	resourceNameDefault := getDatadogResourceName(span, ddNotHTTPTags)

	assert.Equal(t, "example_method", resourceNameHTTP)
	assert.Equal(t, "Default Name", resourceNameDefault)
}

// ensure that the datadog span name uses IL name +kind when available and falls back to opetelemetry + kind
func TestSpanNameTranslation(t *testing.T) {
	span := pdata.NewSpan()
	span.SetName("Default Name")
	span.SetKind(pdata.SpanKindSERVER)

	ddIlTags := map[string]string{
		fmt.Sprintf(conventions.InstrumentationLibraryName): "il_name",
	}

	ddNoIlTags := map[string]string{
		"other": "other_value",
	}

	ddIlTagsOld := map[string]string{
		"otel.instrumentation_library.name": "old_value",
	}

	ddIlTagsCur := map[string]string{
		"otel.library.name": "current_value",
	}

	ddIlTagsUnusual := map[string]string{
		"otel.library.name": "@unusual/\\::value",
	}

	ddIlTagsHyphen := map[string]string{
		"otel.library.name": "hyphenated-value",
	}

	spanNameIl := getDatadogSpanName(span, ddIlTags)
	spanNameDefault := getDatadogSpanName(span, ddNoIlTags)
	spanNameOld := getDatadogSpanName(span, ddIlTagsOld)
	spanNameCur := getDatadogSpanName(span, ddIlTagsCur)
	spanNameUnusual := getDatadogSpanName(span, ddIlTagsUnusual)
	spanNameHyphen := getDatadogSpanName(span, ddIlTagsHyphen)

	assert.Equal(t, strings.ToLower(fmt.Sprintf("%s.%s", "il_name", strings.TrimPrefix(pdata.SpanKindSERVER.String(), "SPAN_KIND_"))), spanNameIl)
	assert.Equal(t, strings.ToLower(fmt.Sprintf("%s.%s", "opentelemetry", strings.TrimPrefix(pdata.SpanKindSERVER.String(), "SPAN_KIND_"))), spanNameDefault)
	assert.Equal(t, strings.ToLower(fmt.Sprintf("%s.%s", "old_value", strings.TrimPrefix(pdata.SpanKindSERVER.String(), "SPAN_KIND_"))), spanNameOld)
	assert.Equal(t, strings.ToLower(fmt.Sprintf("%s.%s", "current_value", strings.TrimPrefix(pdata.SpanKindSERVER.String(), "SPAN_KIND_"))), spanNameCur)
	assert.Equal(t, strings.ToLower(fmt.Sprintf("%s.%s", "unusual_value", strings.TrimPrefix(pdata.SpanKindSERVER.String(), "SPAN_KIND_"))), spanNameUnusual)
	assert.Equal(t, strings.ToLower(fmt.Sprintf("%s.%s", "hyphenated_value", strings.TrimPrefix(pdata.SpanKindSERVER.String(), "SPAN_KIND_"))), spanNameHyphen)
}

// ensure that the datadog span name uses IL name +kind when available and falls back to opetelemetry + kind
func TestSpanNameNormalization(t *testing.T) {
	emptyName := ""
	dashName := "k-e-b-a-b"
	camelCaseName := "camelCase"
	periodName := "first.second"
	removeSpacesName := "removes Spaces"
	numericsName := "num3r1cs_OK"
	underscoresName := "permits_underscores"
	tabName := "\t"
	junkName := "\tgetsRidOf\x1c\x1c\x18Junk"
	onlyJunkName := "\x02\x1c\x18\x08_only_junk_"
	onlyBadCharsName := "\x02\x1c\x18\x08"

	assert.Equal(t, utils.NormalizeSpanName(emptyName, false), "")
	assert.Equal(t, utils.NormalizeSpanName(dashName, false), "k_e_b_a_b")
	assert.Equal(t, utils.NormalizeSpanName(camelCaseName, false), "camelcase")
	assert.Equal(t, utils.NormalizeSpanName(periodName, false), "first.second")
	assert.Equal(t, utils.NormalizeSpanName(removeSpacesName, false), "removes_spaces")
	assert.Equal(t, utils.NormalizeSpanName(numericsName, false), "num3r1cs_ok")
	assert.Equal(t, utils.NormalizeSpanName(underscoresName, false), "permits_underscores")
	assert.Equal(t, utils.NormalizeSpanName(tabName, false), "")
	assert.Equal(t, utils.NormalizeSpanName(junkName, false), "getsridof_junk")
	assert.Equal(t, utils.NormalizeSpanName(onlyJunkName, false), "only_junk")
	assert.Equal(t, utils.NormalizeServiceName(dashName), "k-e-b-a-b")
	assert.Equal(t, utils.NormalizeServiceName(onlyBadCharsName), utils.DefaultServiceName)
	assert.Equal(t, utils.NormalizeServiceName(emptyName), utils.DefaultServiceName)
}

// ensure that the datadog span type gets mapped from span kind
func TestSpanTypeTranslation(t *testing.T) {
	spanTypeClient := spanKindToDatadogType(pdata.SpanKindCLIENT)
	spanTypeServer := spanKindToDatadogType(pdata.SpanKindSERVER)
	spanTypeCustom := spanKindToDatadogType(pdata.SpanKindUNSPECIFIED)

	assert.Equal(t, "http", spanTypeClient)
	assert.Equal(t, "web", spanTypeServer)
	assert.Equal(t, "custom", spanTypeCustom)

}

// ensure that the IL Tags extraction handles nil case
func TestILTagsExctraction(t *testing.T) {
	il := pdata.NewInstrumentationLibrary()

	tags := map[string]string{}

	extractInstrumentationLibraryTags(il, tags)

	assert.Equal(t, "", tags[conventions.InstrumentationLibraryName])

}

func TestHttpResourceTag(t *testing.T) {
	span := pdata.NewSpan()
	span.SetName("Default Name")
	span.SetKind(pdata.SpanKindSERVER)

	ddTags := map[string]string{
		"http.method": "POST",
	}

	resourceName := getDatadogResourceName(pdata.Span{}, ddTags)

	assert.Equal(t, "POST", resourceName)
}

// ensure that payloads get aggregated by env to reduce number of flushes
func TestTracePayloadAggr(t *testing.T) {

	// ensure that tracepayloads are aggregated by matchig hosts if envs match
	env := "testenv"
	payloadOne := pb.TracePayload{
		HostName:     "hostnameTestOne",
		Env:          env,
		Traces:       []*pb.APITrace{},
		Transactions: []*pb.Span{},
	}

	payloadTwo := pb.TracePayload{
		HostName:     "hostnameTestOne",
		Env:          env,
		Traces:       []*pb.APITrace{},
		Transactions: []*pb.Span{},
	}

	var originalPayload []*pb.TracePayload
	originalPayload = append(originalPayload, &payloadOne)
	originalPayload = append(originalPayload, &payloadTwo)

	updatedPayloads := aggregateTracePayloadsByEnv(originalPayload)

	assert.Equal(t, 2, len(originalPayload))
	assert.Equal(t, 1, len(updatedPayloads))

	// ensure that trace playloads are not aggregated by matching hosts if different envs
	envTwo := "testenvtwo"

	payloadThree := pb.TracePayload{
		HostName:     "hostnameTestOne",
		Env:          env,
		Traces:       []*pb.APITrace{},
		Transactions: []*pb.Span{},
	}

	payloadFour := pb.TracePayload{
		HostName:     "hostnameTestOne",
		Env:          envTwo,
		Traces:       []*pb.APITrace{},
		Transactions: []*pb.Span{},
	}

	var originalPayloadDifferentEnv []*pb.TracePayload
	originalPayloadDifferentEnv = append(originalPayloadDifferentEnv, &payloadThree)
	originalPayloadDifferentEnv = append(originalPayloadDifferentEnv, &payloadFour)

	updatedPayloadsDifferentEnv := aggregateTracePayloadsByEnv(originalPayloadDifferentEnv)

	assert.Equal(t, 2, len(originalPayloadDifferentEnv))
	assert.Equal(t, 2, len(updatedPayloadsDifferentEnv))
}

// ensure that stats payloads get tagged with version tag
func TestStatsAggregations(t *testing.T) {
	hostname := "testhostname"
	calculator := newSublayerCalculator()

	// generate mock trace, span and parent span ids
	mockTraceID := [16]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F}
	mockSpanID := [8]byte{0xF1, 0xF2, 0xF3, 0xF4, 0xF5, 0xF6, 0xF7, 0xF8}
	mockParentSpanID := [8]byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8}

	mockEndTime := time.Now().Round(time.Second)

	// create mock resource span data
	// toggle on errors and custom service naming to test edge case code paths
	rs := NewResourceSpansData(mockTraceID, mockSpanID, mockParentSpanID, pdata.StatusCodeError, true, mockEndTime)

	// translate mocks to datadog traces
	cfg := config.Config{}

	datadogPayload := resourceSpansToDatadogSpans(rs, calculator, hostname, &cfg)

	statsOutput := computeAPMStats(&datadogPayload, calculator, time.Now().UTC().UnixNano())

	var statsVersionTag stats.Tag

	// extract the first stats.TagSet containing a stats.Tag of "version"
	for _, countVal := range statsOutput.Stats[0].Counts {
		for _, tagVal := range countVal.TagSet {
			if tagVal.Name == versionAggregationTag {
				statsVersionTag = tagVal
			}
		}
	}

	assert.Equal(t, "test-version", statsVersionTag.Value)
}
