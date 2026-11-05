package db

import (
	"math"
	"testing"
	"time"
)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func TestTPS(t *testing.T) {
	t0 := time.Now()
	prev := Metrics{XactCommit: 100, XactRollback: 10, TakenAt: t0}
	cur := Metrics{XactCommit: 150, XactRollback: 20, TakenAt: t0.Add(2 * time.Second)}

	// (150+20) - (100+10) = 60 over 2s = 30 tps.
	if got := cur.TPS(prev); !approx(got, 30) {
		t.Errorf("TPS = %v, want 30", got)
	}
}

func TestTPSZeroPrevOrInterval(t *testing.T) {
	cur := Metrics{XactCommit: 150, TakenAt: time.Now()}
	// Zero-valued prev (no TakenAt) -> 0, still sampling.
	if got := cur.TPS(Metrics{}); got != 0 {
		t.Errorf("TPS with empty prev = %v, want 0", got)
	}
	// Non-positive interval -> 0.
	t0 := time.Now()
	prev := Metrics{XactCommit: 100, TakenAt: t0}
	same := Metrics{XactCommit: 200, TakenAt: t0}
	if got := same.TPS(prev); got != 0 {
		t.Errorf("TPS with zero interval = %v, want 0", got)
	}
}

func TestTPSCounterReset(t *testing.T) {
	t0 := time.Now()
	prev := Metrics{XactCommit: 500, TakenAt: t0}
	cur := Metrics{XactCommit: 10, TakenAt: t0.Add(time.Second)} // counter reset
	if got := cur.TPS(prev); got != 0 {
		t.Errorf("TPS on counter reset = %v, want 0", got)
	}
}

func TestCacheHitRatioDelta(t *testing.T) {
	prev := Metrics{BlksHit: 900, BlksRead: 100}
	cur := Metrics{BlksHit: 990, BlksRead: 110}
	// delta hit=90, read=10 -> 90/100 = 0.9
	if got := cur.CacheHitRatio(prev); !approx(got, 0.9) {
		t.Errorf("cache hit = %v, want 0.9", got)
	}
}

func TestCacheHitRatioFallsBackToCumulative(t *testing.T) {
	cur := Metrics{BlksHit: 75, BlksRead: 25}
	// Empty prev -> cumulative 75/100 = 0.75
	if got := cur.CacheHitRatio(Metrics{}); !approx(got, 0.75) {
		t.Errorf("cumulative cache hit = %v, want 0.75", got)
	}
}

func TestCacheHitRatioNoReads(t *testing.T) {
	prev := Metrics{BlksHit: 100, BlksRead: 50}
	cur := Metrics{BlksHit: 100, BlksRead: 50} // no change
	if got := cur.CacheHitRatio(prev); !approx(got, 1.0) {
		t.Errorf("cache hit with no delta = %v, want 1.0", got)
	}
}

func TestFormatBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
	}
	for _, c := range cases {
		if got := FormatBytes(c.in); got != c.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}
