package osexec

import "time"

// SetOpenLinkGrace shortens the grace window for a test and restores it on cleanup.
func SetOpenLinkGrace(d time.Duration) (restore func()) {
	old := openLinkGrace
	openLinkGrace = d
	return func() { openLinkGrace = old }
}
