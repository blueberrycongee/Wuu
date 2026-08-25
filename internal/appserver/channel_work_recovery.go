package appserver

import (
	"context"
	"strings"

	"github.com/blueberrycongee/wuu/internal/channels"
	"github.com/blueberrycongee/wuu/internal/session"
)

func (s *Server) reconcileChannelWorkRuns(ctx context.Context) error {
	if s == nil || s.channelService == nil || s.rt == nil {
		return nil
	}
	runs, err := s.channelService.ListUnsettledWorkRuns(ctx)
	if err != nil {
		return err
	}
	recoveries := make([]channels.WorkRunRecovery, 0, len(runs))
	for _, run := range runs {
		state := channels.WorkRunRecoveryMissing
		active, activeErr := session.ThreadExecutionActive(s.rt.SessionDir, run.SessionRef)
		if activeErr != nil {
			return activeErr
		}
		if active {
			state = channels.WorkRunRecoveryActive
		} else if stored, found, findErr := session.Find(s.rt.SessionDir, run.SessionRef); findErr != nil {
			return findErr
		} else if found {
			// A persisted but incomplete session remains recoverable by Core's
			// queue/turn recovery. A completed turn is settled here so the room
			// runtime can consume its result and submit the verifier receipt.
			state = channels.WorkRunRecoveryActive
			if strings.TrimSpace(stored.LatestCompletedTurnID) != "" {
				state = channels.WorkRunRecoveryCompleted
			}
		}
		recoveries = append(recoveries, channels.WorkRunRecovery{
			RunID: run.ID, SessionRef: run.SessionRef, State: state,
		})
	}
	return s.channelService.ReconcileWorkRuns(ctx, recoveries)
}
