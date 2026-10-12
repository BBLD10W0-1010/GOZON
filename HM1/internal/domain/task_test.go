package domain_test

import (
	"hm1/internal/domain"
	"testing"
	"time"
)

func TestDeadlineAndLocalCalendar(t *testing.T) {
	for _, value := range []string{"", "2030-02-30", "10.01.2030", "2030-01-10T12:00:00Z"} {
		if _, err := domain.ParseDeadline(value); err == nil {
			t.Fatalf("accepted invalid date %q", value)
		}
	}
	deadline, err := domain.ParseDeadline("2030-01-10")
	if err != nil {
		t.Fatal(err)
	}
	// Local midnight already belongs to Jan 10 while the UTC date is still Jan 9.
	now := time.Date(2030, 1, 10, 0, 30, 0, 0, time.FixedZone("UTC+5", 5*3600))
	if !domain.Today(now).Equal(deadline) {
		t.Fatal("Today used the UTC calendar instead of the local calendar")
	}
}

func TestStatuses(t *testing.T) {
	for _, status := range []domain.Status{domain.Planned, domain.InProgress, domain.Canceled, domain.Done} {
		if !status.Valid() {
			t.Fatalf("invalid known status %q", status)
		}
		if status.Terminal() != (status == domain.Done || status == domain.Canceled) {
			t.Fatalf("wrong terminal status %q", status)
		}
	}
	if domain.Status("unknown").Valid() {
		t.Fatal("unknown status accepted")
	}
}
