package app

import "github.com/playdead/koa/internal/pathenv"

// ensurePathOnDisk adds the bin folder to PATH persistently.
func ensurePathOnDisk(binDir string) (PathState, error) {
	st, err := pathenv.Ensure(binDir)
	return toPathState(st), err
}

// checkPath reports the current PATH state without changing anything.
func checkPath(binDir string) PathState {
	return toPathState(pathenv.Check(binDir))
}

func toPathState(st pathenv.Status) PathState {
	return PathState{
		OnPath:       st.OnPath,
		Persisted:    st.Persisted,
		NeedsRestart: st.NeedsRestart,
		Detail:       st.Detail,
	}
}
