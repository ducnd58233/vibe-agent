package sse

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBeginSetsSSEHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	conn, err := Begin(rec)
	if err != nil {
		t.Fatal(err)
	}
	if conn == nil {
		t.Fatal("expected conn")
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("content-type = %q", ct)
	}
	if rec.Header().Get(headerAccelBuffering) != "no" {
		t.Fatalf("missing %s header", headerAccelBuffering)
	}
}

func TestWriteEventMultilineData(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := WriteEvent(rec, Event{Data: "line1\nline2"}); err != nil {
		t.Fatal(err)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "data: line1\n") || !strings.Contains(body, "data: line2\n") {
		t.Fatalf("unexpected body: %q", body)
	}
}

func TestPollRunsUntilContextDone(t *testing.T) {
	rec := httptest.NewRecorder()
	conn, err := Begin(rec)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	calls := 0
	err = Poll(ctx, conn, 50*time.Millisecond, func(context.Context) ([]Event, error) {
		calls++
		return []Event{{Data: "tick"}}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls < 2 {
		t.Fatalf("expected multiple poll ticks, got %d", calls)
	}
}
