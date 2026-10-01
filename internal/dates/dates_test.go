package dates

import (
	"testing"
	"time"
)

func TestParseInstant(t *testing.T) {
	now := time.Date(2026, 10, 1, 15, 30, 0, 0, time.UTC)
	cases := []struct {
		expr string
		end  bool
		want time.Time
	}{
		{"today", false, time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		{"today", true, time.Date(2026, 10, 1, 23, 59, 59, 0, time.UTC)},
		{"yesterday", false, time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)},
		{"2026-09-20", false, time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)},
		{"3 days ago", false, now.Add(-72 * time.Hour)},
		{"2 hours ago", false, now.Add(-2 * time.Hour)},
		{"now", false, now},
	}
	for _, c := range cases {
		got, err := ParseInstant(c.expr, now, c.end)
		if err != nil {
			t.Fatalf("%s: %v", c.expr, err)
		}
		if !got.Equal(c.want) {
			t.Errorf("%q end=%v: got %s want %s", c.expr, c.end, got, c.want)
		}
	}

	if _, err := ParseInstant("banana", now, false); err == nil {
		t.Error("expected error for unparseable input")
	}
}

func TestRange(t *testing.T) {
	now := time.Date(2026, 10, 1, 15, 30, 0, 0, time.UTC)
	from, to, err := Range(now, "week", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if from.Weekday() != time.Monday {
		t.Errorf("week should start Monday, got %s", from.Weekday())
	}
	if !to.After(from) {
		t.Error("to should be after from")
	}

	from, to, err = Range(now, "", "2026-09-01", "2026-09-30")
	if err != nil {
		t.Fatal(err)
	}
	if from.Month() != time.September || to.Month() != time.September {
		t.Errorf("bad range: %s .. %s", from, to)
	}
}
