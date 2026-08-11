package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/IBM/sarama/mocks"
)

func TestAsyncProducerCallbackFiresOnSuccess(t *testing.T) {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true
	mockAP := mocks.NewAsyncProducer(t, cfg)
	mockAP.ExpectInputAndSucceed()

	var wg sync.WaitGroup
	wg.Add(1)
	var gotVal testEvent
	var gotErr error

	p := &AsyncProducer[testEvent]{
		ap:        mockAP,
		topicName: "t",
		encode:    func(v testEvent) ([]byte, error) { return json.Marshal(v) },
		callback: func(_ context.Context, v testEvent, err error) {
			gotVal = v
			gotErr = err
			wg.Done()
		},
		logger: stderrLogger{},
		closed: make(chan struct{}),
	}
	p.drainWG.Add(2)
	go p.drainSuccesses()
	go p.drainErrors()

	if err := p.Publish(context.Background(), testEvent{ID: "hello"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	waitTimeout(t, &wg, time.Second, "callback never fired")

	if gotErr != nil {
		t.Fatalf("err = %v", gotErr)
	}
	if gotVal.ID != "hello" {
		t.Fatalf("value = %+v", gotVal)
	}

	if err := p.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestAsyncProducerCallbackFiresOnError(t *testing.T) {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true
	mockAP := mocks.NewAsyncProducer(t, cfg)
	mockAP.ExpectInputAndFail(errors.New("broker down"))

	var wg sync.WaitGroup
	wg.Add(1)
	var gotErr error

	p := &AsyncProducer[testEvent]{
		ap:        mockAP,
		topicName: "t",
		encode:    func(v testEvent) ([]byte, error) { return json.Marshal(v) },
		callback: func(_ context.Context, _ testEvent, err error) {
			gotErr = err
			wg.Done()
		},
		logger: stderrLogger{},
		closed: make(chan struct{}),
	}
	p.drainWG.Add(2)
	go p.drainSuccesses()
	go p.drainErrors()

	if err := p.Publish(context.Background(), testEvent{ID: "x"}); err != nil {
		t.Fatalf("publish: %v", err)
	}

	waitTimeout(t, &wg, time.Second, "error callback never fired")

	if gotErr == nil || gotErr.Error() != "broker down" {
		t.Fatalf("err = %v", gotErr)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestAsyncProducerRejectsPublishAfterClose(t *testing.T) {
	cfg := sarama.NewConfig()
	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true
	mockAP := mocks.NewAsyncProducer(t, cfg)

	p := &AsyncProducer[testEvent]{
		ap:        mockAP,
		topicName: "t",
		encode:    func(v testEvent) ([]byte, error) { return json.Marshal(v) },
		logger:    stderrLogger{},
		closed:    make(chan struct{}),
	}
	p.drainWG.Add(2)
	go p.drainSuccesses()
	go p.drainErrors()

	if err := p.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	err := p.Publish(context.Background(), testEvent{ID: "x"})
	if err == nil {
		t.Fatal("publish after close should error")
	}
}

func waitTimeout(t *testing.T, wg *sync.WaitGroup, d time.Duration, msg string) {
	t.Helper()
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatal(msg)
	}
}
