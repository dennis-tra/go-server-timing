package servertiming

import (
	"context"
	"testing"
	"testing/synctest"
	"time"
)

func TestFromContext_Missing(t *testing.T) {
	ctx := context.Background()
	if got := FromContext(ctx); got != nil {
		t.Errorf("FromContext on bare context = %v, want nil", got)
	}
}

func TestFromContext_Nil(t *testing.T) {
	if got := FromContext(nil); got != nil {
		t.Errorf("FromContext(nil) = %v, want nil", got)
	}
}

func TestFromContext_RoundTrip(t *testing.T) {
	h := NewHeader()
	ctx := NewContext(context.Background(), h)
	if got := FromContext(ctx); got != h {
		t.Errorf("FromContext returned %v, want %v", got, h)
	}
}

func TestFromContext_SurvivesWithValue(t *testing.T) {
	h := NewHeader()
	type other struct{}
	ctx := NewContext(context.Background(), h)
	ctx = context.WithValue(ctx, other{}, "unrelated")
	if got := FromContext(ctx); got != h {
		t.Errorf("FromContext after nested WithValue = %v, want %v", got, h)
	}
}

func TestFromContext_NilSafeChaining(t *testing.T) {
	// The documented usage pattern: handlers chain .NewMetric().Start()
	// on whatever FromContext returns, even when no middleware is in use.
	synctest.Test(t, func(t *testing.T) {
		ctx := context.Background()
		h := FromContext(ctx)
		m := h.NewMetric("db").Start()
		time.Sleep(time.Millisecond)
		m.Stop()
		if m.Name != "db" {
			t.Errorf("Name = %q, want db", m.Name)
		}
		if m.Duration != time.Millisecond {
			t.Errorf("Duration = %v, want 1ms exactly (synctest)", m.Duration)
		}
	})
}
