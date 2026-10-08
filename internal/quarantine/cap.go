package quarantine

// Cap is how many tests may be quarantined at once.
// The plan's percent truncates. Under 20 tests that truncation is 0, which
// would make the lab unable to quarantine anything. A repo with at least one
// test therefore has a floor of 1. Zero tests stays at 0.
func Cap(testCount, percent, maxN int) int {
	if testCount <= 0 || maxN <= 0 || percent <= 0 {
		return 0
	}
	pct := testCount * percent / 100
	if pct > maxN {
		pct = maxN
	}
	if pct < 1 {
		return 1
	}
	return pct
}
