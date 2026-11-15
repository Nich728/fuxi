package study

import (
	"fmt"
	"sync"
	"time"
)

// DayBounds returns the study day containing now (SYSTEM_DESIGN §6.4): it
// starts at startHour:00 local time in loc, so with the default 4 am a review
// at 1 am still counts for the evening before (PRD R5). Both bounds are UTC.
//
// On a daylight-saving change the day is 23 or 25 hours long. If startHour
// falls in a skipped hour, time.Date picks one instant for it; both the day
// before and the day after use the same instant, so days never overlap or
// leave a gap.
func DayBounds(now time.Time, loc *time.Location, startHour int) (start, end time.Time) {
	local := now.In(loc)
	start = time.Date(local.Year(), local.Month(), local.Day(), startHour, 0, 0, 0, loc)
	if local.Before(start) {
		start = time.Date(local.Year(), local.Month(), local.Day()-1, startHour, 0, 0, 0, loc)
	}
	end = time.Date(start.Year(), start.Month(), start.Day()+1, startHour, 0, 0, 0, loc)
	return start.UTC(), end.UTC()
}

var (
	locMu    sync.RWMutex
	locCache = map[string]*time.Location{}
)

// location loads and caches an IANA time zone. The binary embeds the zone
// database (time/tzdata in cmd/fuxi), so this works in the distroless image.
func location(name string) (*time.Location, error) {
	locMu.RLock()
	loc, ok := locCache[name]
	locMu.RUnlock()
	if ok {
		return loc, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("load time zone %q: %w", name, err)
	}
	locMu.Lock()
	locCache[name] = loc
	locMu.Unlock()
	return loc, nil
}
