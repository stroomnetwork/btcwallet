package chain

import (
	"testing"
	"time"
)

func TestTopicQueue(t *testing.T) {
	tests := []struct {
		name             string
		subscribersCount int
	}{
		{
			name:             "single subscriber",
			subscribersCount: 1,
		},

		{
			name:             "multiple subscribers",
			subscribersCount: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tq := NewTopicQueue()
			tq.Start()
			defer tq.Stop()

			outs := make([]<-chan interface{}, tt.subscribersCount)
			for i := 0; i < tt.subscribersCount; i++ {
				outs[i] = tq.ChanOut()
			}
			// send 1 message
			go func() {
				tq.ChanIn() <- "hello"
			}()

			// verify it's broadcast to all
			for i := 0; i < tt.subscribersCount; i++ {
				select {
				case <-outs[i]:
					// Ok
				case <-time.After(100 * time.Millisecond):
					t.Fatal("message not received within timeout")
				}
			}
		})
	}
}
