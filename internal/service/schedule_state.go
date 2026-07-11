package service

import (
	"encoding/json"
	"os"

	"pbs-win-backup/internal/datalock"
	"pbs-win-backup/internal/paths"
)

type scheduleState struct {
	LastRun map[string]string `json:"last_run"` // jobID -> RFC3339 minute key
}

func scheduleStatePath() (string, error) {
	dir, err := paths.DataDir()
	if err != nil {
		return "", err
	}
	return dir + string(os.PathSeparator) + "schedule_state.json", nil
}

func loadScheduleState() (scheduleState, error) {
	st := scheduleState{LastRun: map[string]string{}}
	err := datalock.With("schedule_state", func() error {
		p, err := scheduleStatePath()
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		return json.Unmarshal(data, &st)
	})
	if st.LastRun == nil {
		st.LastRun = map[string]string{}
	}
	return st, err
}

func saveScheduleState(st scheduleState) error {
	return datalock.With("schedule_state", func() error {
		p, err := scheduleStatePath()
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(st, "", "  ")
		if err != nil {
			return err
		}
		return paths.AtomicWriteSensitive(p, data, 0o600)
	})
}
