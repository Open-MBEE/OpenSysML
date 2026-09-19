package usage

import (
	"os"
	"strconv"
	"time"
)

// manDate is the date the committed pages carry: a constant, since a moving
// date would fail the check that regenerates them. Bump it when a page changes.
const manDate = "2026-08-30"

// DefaultManMeta is the title line the toolchain's pages share, dated from
// SOURCE_DATE_EPOCH where a reproducible build sets it.
func DefaultManMeta() ManMeta {
	return ManMeta{Date: sourceDate(), Source: "OpenSysML", Manual: "OpenSysML Manual"}
}

// sourceDate reads SOURCE_DATE_EPOCH, ignoring a value that is not the seconds
// the convention asks for, and falls back on the date the pages were written.
func sourceDate() string {
	seconds, err := strconv.ParseInt(os.Getenv("SOURCE_DATE_EPOCH"), 10, 64)
	if err != nil {
		return manDate
	}
	return time.Unix(seconds, 0).UTC().Format(time.DateOnly)
}
