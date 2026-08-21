package idempotency

import (
	"context"
	"testing"
	"time"
)

func TestConcurrentDuplicateWaitsForOwnerTerminalState(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		finish     func(*Store) error
		wantReplay bool
		wantStatus int
	}{
		{
			name: "owner completes",
			finish: func(store *Store) error {
				return store.Complete("tenant-a", "request-7", 201, []byte(`{"shipment_id":"shipment-7"}`))
			},
			wantReplay: true,
			wantStatus: 201,
		},
		{
			name: "owner aborts",
			finish: func(store *Store) error {
				return store.Abort("tenant-a", "request-7")
			},
			wantReplay: false,
			wantStatus: 102,
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			store := New()
			now := time.Unix(1_700_000_000, 0).UTC()
			fingerprint := Fingerprint("POST", "/v1/shipments", []byte(`{"reference":"AWB-7"}`))
			if _, replay, err := store.BeginContext(context.Background(), "tenant-a", "request-7", fingerprint, now); err != nil || replay {
				t.Fatalf("owner begin: replay=%v err=%v", replay, err)
			}

			type result struct {
				record Record
				replay bool
				err    error
			}
			waiter := make(chan result, 1)
			go func() {
				record, replay, err := store.BeginContext(context.Background(), "tenant-a", "request-7", fingerprint, now.Add(time.Second))
				waiter <- result{record: record, replay: replay, err: err}
			}()
			select {
			case early := <-waiter:
				t.Fatalf("duplicate returned before owner terminal state: status=%d replay=%v err=%v", early.record.Status, early.replay, early.err)
			case <-time.After(50 * time.Millisecond):
			}

			if err := testCase.finish(store); err != nil {
				t.Fatalf("finish owner: %v", err)
			}
			select {
			case outcome := <-waiter:
				if outcome.err != nil || outcome.replay != testCase.wantReplay || outcome.record.Status != testCase.wantStatus {
					t.Fatalf("waiter outcome: status=%d replay=%v err=%v", outcome.record.Status, outcome.replay, outcome.err)
				}
			case <-time.After(time.Second):
				t.Fatal("duplicate remained blocked after owner terminal state")
			}
		})
	}
}
