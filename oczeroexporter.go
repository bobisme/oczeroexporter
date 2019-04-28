package oczeroexporter

import (
	"encoding/hex"
	"fmt"
	"regexp"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

// reZero provides a simple way to detect an empty ID
var reZero = regexp.MustCompile(`^0+$`)

// Exporter is a stats and trace exporter that logs
// the exported data using zerolog.
type Exporter struct {
	eventFn func() *zerolog.Event
}

func New(eventFn func() *zerolog.Event) *Exporter {
	return &Exporter{eventFn}
}

func (e *Exporter) log() *zerolog.Event {
	if e.eventFn == nil {
		return log.Debug()
	}
	return e.eventFn()
}

// ExportView logs the view data.
func (e *Exporter) ExportView(vd *view.Data) {
	for _, row := range vd.Rows {
		l := e.log()
		l = l.Str("name", vd.View.Name)
		l = l.Time("end", vd.End)

		switch v := row.Data.(type) {
		case *view.DistributionData:
			l = l.Float64("distributionMin", v.Min).
				Float64("distributionMax", v.Max).
				Float64("distributionMean", v.Mean)
		case *view.CountData:
			l = l.Int64("count", v.Value)
		case *view.SumData:
			l = l.Float64("sum", v.Value)
		case *view.LastValueData:
			l = l.Float64("last", v.Value)
		}

		tags := make([]string, 0, len(row.Tags))
		for _, tag := range row.Tags {
			if tag.Value == "" {
				continue
			}
			t := fmt.Sprintf("%s:%v", tag.Key.Name(), tag.Value)
			tags = append(tags, t)
		}
		l = l.Strs("tags", tags)
		l.Msg("metric")
	}
}

// ExportSpan logs the trace span.
func (e *Exporter) ExportSpan(vd *trace.SpanData) {
	traceID := hex.EncodeToString(vd.SpanContext.TraceID[:])
	spanID := hex.EncodeToString(vd.SpanContext.SpanID[:])
	parentSpanID := hex.EncodeToString(vd.ParentSpanID[:])
	l := e.log()
	l = l.Str("traceId", traceID).Str("spanId", spanID)
	if !reZero.MatchString(parentSpanID) {
		l = l.Str("parentSpanId", parentSpanID)
	}

	l = l.Str("span", vd.Name).
		Str("statusMessage", vd.Status.Message).
		Int32("statusCode", vd.Status.Code).
		Dur("elapsed", vd.EndTime.Sub(vd.StartTime))

	for _, item := range vd.Annotations {
		for k, v := range item.Attributes {
			l = l.Interface(
				fmt.Sprintf("annotations.%s.%s", item.Message, k), v)
		}
	}

	for k, v := range vd.Attributes {
		l = l.Interface(fmt.Sprintf("attributes.%s", k), v)
	}
	l.Msg("trace")
}
