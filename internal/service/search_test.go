package service_test

import (
	"errors"
	"net/url"
	"slices"
	"testing"
	"time"

	"github.com/regeda/techlog/internal/service"
	"github.com/regeda/techlog/internal/testutil"
)

func TestSearchParamsFromQuery(t *testing.T) {
	v := url.Values{
		"q": {" spin "}, "system": {"alpha"}, "type": {"a.b, c.d", "e.f"},
		"since": {"2026-03-10"}, "until": {"2026-03-10"}, "limit": {"7"},
	}
	p, err := service.SearchParamsFromQuery(v)
	if err != nil {
		t.Fatal(err)
	}
	if p.Query != "spin" || p.System != "alpha" || p.Limit != 7 {
		t.Errorf("p = %+v", p)
	}
	if !slices.Equal(p.Types, []string{"a.b", "c.d", "e.f"}) {
		t.Errorf("types = %v", p.Types)
	}
	if want := time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC); !p.Since.Equal(want) {
		t.Errorf("since = %v", p.Since)
	}
	if want := time.Date(2026, 3, 10, 23, 59, 59, int(999*time.Millisecond), time.UTC); !p.Until.Equal(want) {
		t.Errorf("until = %v, want the end of that day", p.Until)
	}

	p, err = service.SearchParamsFromQuery(url.Values{"since": {"2026-03-11T12:00:00+02:00"}})
	if err != nil || !p.Since.Equal(time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)) {
		t.Errorf("offset since = %v, %v", p.Since, err)
	}
}

func TestSearchParamsFromQueryErrors(t *testing.T) {
	for _, v := range []url.Values{
		{"since": {"yesterday"}}, {"until": {"2026-13-40"}}, {"limit": {"abc"}},
	} {
		_, err := service.SearchParamsFromQuery(v)
		var bad *service.BadRequestError
		if !errors.As(err, &bad) {
			t.Errorf("%v: got %v, want a BadRequestError", v, err)
		}
	}
}

func TestSearchAndHistoryClampLimits(t *testing.T) {
	svc, fs := newService(t)
	for _, tc := range []struct{ in, want int }{{0, 20}, {-3, 20}, {5, 5}, {500, 100}} {
		if _, err := svc.Search(ctx, service.SearchParams{Limit: tc.in}); err != nil {
			t.Fatal(err)
		}
		if fs.lastFilter.Limit != tc.want {
			t.Errorf("Search limit %d -> %d, want %d", tc.in, fs.lastFilter.Limit, tc.want)
		}
	}
	for _, tc := range []struct{ in, want int }{{0, 50}, {5, 5}, {1000, 200}} {
		if _, err := svc.History(ctx, "alpha", time.Time{}, tc.in); err != nil {
			t.Fatal(err)
		}
		if fs.lastFilter.Limit != tc.want {
			t.Errorf("History limit %d -> %d, want %d", tc.in, fs.lastFilter.Limit, tc.want)
		}
	}
}

func TestSearchNeverReturnsNil(t *testing.T) {
	svc, _ := newService(t)
	got, err := svc.Search(ctx, service.SearchParams{Query: "nothing"})
	if err != nil || got == nil {
		t.Fatalf("Search = %v, %v; want an empty non-nil slice", got, err)
	}
}

func TestHistoryLatestNChronological(t *testing.T) {
	svc, _ := newService(t)
	for i, ts := range []string{"2026-03-10T09:00:00Z", "2026-03-10T10:00:00Z", "2026-03-10T11:00:00Z", "2026-03-10T12:00:00Z"} {
		d := testutil.Doc("alpha", "problem.reported", string(rune('a'+i)))
		d["time"] = ts
		record(t, svc, d)
	}
	record(t, svc, testutil.Doc("beta", "problem.reported", "other system"))

	got, err := svc.History(ctx, "alpha", time.Time{}, 3)
	if err != nil {
		t.Fatal(err)
	}
	var summaries []string
	for _, e := range got {
		summaries = append(summaries, e.Summary)
	}
	if !slices.Equal(summaries, []string{"b", "c", "d"}) {
		t.Fatalf("history = %v, want the latest three, oldest first", summaries)
	}

	empty, err := svc.History(ctx, "no-such-system", time.Time{}, 10)
	if err != nil || empty == nil || len(empty) != 0 {
		t.Fatalf("unknown system: %v, %v; want an empty list and no error", empty, err)
	}

	var bad *service.BadRequestError
	if _, err := svc.History(ctx, "", time.Time{}, 10); !errors.As(err, &bad) {
		t.Fatalf("empty system: got %v, want a BadRequestError", err)
	}
}

func TestSearchPassesOffset(t *testing.T) {
	svc, fs := newService(t)
	for _, tc := range []struct{ in, want int }{{0, 0}, {50, 50}, {-5, 0}} {
		if _, err := svc.Search(ctx, service.SearchParams{Offset: tc.in}); err != nil {
			t.Fatal(err)
		}
		if fs.lastFilter.Offset != tc.want {
			t.Errorf("offset %d -> %d, want %d", tc.in, fs.lastFilter.Offset, tc.want)
		}
	}
}
