package oczeroexporter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"
)

var (
	MTest       = stats.Int64("test/nothing", "Nothing", "By")
	KeyTag1, _  = tag.NewKey("tag1")
	KeyTag2, _  = tag.NewKey("tag2")
	NothingView = &view.View{
		Name:        "test/nothing_view",
		Measure:     MTest,
		Description: "The number of nothing",
		Aggregation: view.Count(),
		TagKeys:     []tag.Key{KeyTag1, KeyTag2},
	}
)

func expectNo(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func expectEq(t *testing.T, a interface{}, b interface{}) {
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("%#v != %#v", a, b)
	}
}

func expectNotEq(t *testing.T, a interface{}, b interface{}) {
	if reflect.DeepEqual(a, b) {
		t.Fatalf("%+v == %+v", a, b)
	}
}

func TestMetrics(t *testing.T) {
	b := make([]byte, 0)
	buf := bytes.NewBuffer(b)
	logger := zerolog.New(buf)
	e := New(logger.Debug)
	view.Register(NothingView)
	view.RegisterExporter(e)
	view.SetReportingPeriod(100 * time.Millisecond)

	ctx, err := tag.New(
		context.Background(),
		tag.Insert(KeyTag1, "not1"),
		tag.Insert(KeyTag2, "not2"))
	expectNo(t, err)
	stats.Record(ctx, MTest.M(1))
	stats.Record(ctx, MTest.M(1))

	time.Sleep(150 * time.Millisecond)

	r := bufio.NewReader(buf)
	line, _, err := r.ReadLine()
	expectNo(t, err)
	var logged struct {
		Level   string
		Message string
		Name    string
		Count   float64
		Tags    []string
	}
	expected := logged
	err = json.Unmarshal(line, &logged)
	expectNo(t, err)

	expected.Level = "debug"
	expected.Message = "metric"
	expected.Name = "test/nothing_view"
	expected.Count = 2.0
	expected.Tags = []string{"tag1:not1", "tag2:not2"}

	expectEq(t, logged, expected)
}

func TestTrace(t *testing.T) {
	b := make([]byte, 0)
	buf := bytes.NewBuffer(b)
	logger := zerolog.New(buf)
	e := New(logger.Debug)
	trace.RegisterExporter(e)
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})

	// work
	ctx := context.Background()
	_, span := trace.StartSpan(ctx, "/foo")
	span.AddAttributes(
		trace.BoolAttribute("key1", false),
		trace.BoolAttribute("key2", true),
		trace.StringAttribute("key3", "hello"),
		trace.StringAttribute("key4", "hello"),
	)
	span.End()

	// assert
	logged := make(map[string]interface{})
	err := json.Unmarshal(buf.Bytes(), &logged)
	expectNo(t, err)
	expectEq(t, logged["level"], "debug")
	expectEq(t, logged["message"], "trace")
	expectEq(t, logged["span"], "/foo")
	expectEq(t, len(logged["traceId"].(string)), 32)
	expectEq(t, len(logged["spanId"].(string)), 16)
	expectEq(t, logged["attributes.key1"], false)
	expectEq(t, logged["attributes.key2"], true)
	if logged["elapsed"].(float64) <= 0 {
		t.Fatalf("expected %+v to be greater than 0", logged["elapsed"])
	}
}
