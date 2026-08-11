package kafka

import (
	"context"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/IBM/sarama"
)

// mockClaim implements sarama.ConsumerGroupClaim with a preloaded message stream.
type mockClaim struct {
	msgs chan *sarama.ConsumerMessage
}

func (c *mockClaim) Topic() string                            { return "t" }
func (c *mockClaim) Partition() int32                         { return 0 }
func (c *mockClaim) InitialOffset() int64                     { return 0 }
func (c *mockClaim) HighWaterMarkOffset() int64               { return 0 }
func (c *mockClaim) Messages() <-chan *sarama.ConsumerMessage { return c.msgs }

func TestPoolContiguousCommit(t *testing.T) {
	// Setup: 10 messages, 4 workers, out-of-order completion via variable handler latency.
	const N = 10
	const workers = 4

	var latencyByOffset = map[int64]time.Duration{
		0: 40 * time.Millisecond,
		1: 5 * time.Millisecond,
		2: 20 * time.Millisecond,
		3: 5 * time.Millisecond,
		4: 5 * time.Millisecond,
		5: 15 * time.Millisecond,
		6: 5 * time.Millisecond,
		7: 5 * time.Millisecond,
		8: 5 * time.Millisecond,
		9: 5 * time.Millisecond,
	}
	var processedOrder []int64
	var mu sync.Mutex
	h := &recordingHandler{
		name: "pool",
		fn: func(evt testEvent) Progress {
			// simulate work using the offset embedded in id
			return Progress{Status: ProgressSuccess}
		},
	}
	// Wrap adapter so we can observe order + inject sleep by offset:
	// use a custom messageHandler directly.
	adapter := adaptEventHandler(h, nil, withJSONDecoder[testEvent]())
	timed := func(ctx context.Context, msg *sarama.ConsumerMessage) messageResult {
		time.Sleep(latencyByOffset[msg.Offset])
		mu.Lock()
		processedOrder = append(processedOrder, msg.Offset)
		mu.Unlock()
		return adapter(ctx, msg)
	}

	proc := &cgHandler{
		handler:     timed,
		logger:      stderrLogger{},
		strategy:    ErrorSkip,
		concurrency: workers,
	}
	sess := &mockSession{ctx: context.Background()}
	claim := &mockClaim{msgs: make(chan *sarama.ConsumerMessage, N)}
	for i := 0; i < N; i++ {
		claim.msgs <- newMsg(int64(i), "e")
	}
	close(claim.msgs)

	proc.consumeClaimPool(sess, claim)

	marked := sess.markedOffsets()
	if len(marked) == 0 {
		t.Fatal("no offsets marked")
	}
	// Highest mark should be N-1 (all contiguous).
	if marked[len(marked)-1] != int64(N-1) {
		t.Fatalf("last mark = %d, want %d", marked[len(marked)-1], N-1)
	}
	// Every marked offset must be monotonically non-decreasing (contiguous invariant).
	for i := 1; i < len(marked); i++ {
		if marked[i] <= marked[i-1] {
			t.Fatalf("marks not strictly increasing: %v", marked)
		}
	}
	// Sanity: processing did occur out of order (worker pool was actually concurrent).
	sorted := make([]int64, len(processedOrder))
	copy(sorted, processedOrder)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	inOrder := true
	for i := range sorted {
		if sorted[i] != processedOrder[i] {
			inOrder = false
			break
		}
	}
	if inOrder {
		t.Log("warning: all messages processed in offset order — pool concurrency not observed (test still valid)")
	}
}

func TestPoolWithSingleWorkerMatchesSerialPath(t *testing.T) {
	// concurrency=1 must take the serial path; regression guard.
	h := &recordingHandler{name: "single"}
	proc := &cgHandler{
		handler:     adaptEventHandler(h, nil, withJSONDecoder[testEvent]()),
		logger:      stderrLogger{},
		strategy:    ErrorSkip,
		concurrency: 1,
	}
	sess := &mockSession{ctx: context.Background()}
	claim := &mockClaim{msgs: make(chan *sarama.ConsumerMessage, 3)}
	claim.msgs <- newMsg(1, "a")
	claim.msgs <- newMsg(2, "b")
	claim.msgs <- newMsg(3, "c")
	close(claim.msgs)

	// Direct call to the exported dispatch as ConsumeClaim would.
	_ = proc.ConsumeClaim(sess, claim)

	marked := sess.markedOffsets()
	if len(marked) != 3 || marked[2] != 3 {
		t.Fatalf("marked = %v, want [1 2 3]", marked)
	}
}
