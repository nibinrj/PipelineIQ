package quarantine

// StageRun is one quarantine-stage result used to recompute the release streak.
type StageRun struct {
	BuildID int64
	Outcome string
}

// Streak counts consecutive PASSED quarantine-stage runs after sinceBuildID,
// walking from the newest build backward. Any other outcome breaks the streak.
// The same build uploaded twice does not add a second pass: the caller passes
// the stored rows, not an increment.
func Streak(runs []StageRun, sinceBuildID int64) int {
	latest := map[int64]StageRun{}
	var ids []int64
	for _, run := range runs {
		if run.BuildID <= sinceBuildID {
			continue
		}
		if _, ok := latest[run.BuildID]; !ok {
			ids = append(ids, run.BuildID)
		}
		latest[run.BuildID] = run
	}
	// Newest first. ids were appended in caller order, so sort by id descending.
	for i := 0; i < len(ids); i++ {
		for j := i + 1; j < len(ids); j++ {
			if ids[j] > ids[i] {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
	n := 0
	for _, id := range ids {
		if latest[id].Outcome != "PASSED" {
			break
		}
		n++
	}
	return n
}
