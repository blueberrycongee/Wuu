package loopdriver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/blueberrycongee/wuu/internal/providers"
)

func validateCheckpoint(descriptor Descriptor, checkpoint Checkpoint) error {
	if checkpoint.ContractVersion != ContractVersion {
		return fmt.Errorf("unsupported driver checkpoint contract %d", checkpoint.ContractVersion)
	}
	if checkpoint.DriverID != descriptor.ID || checkpoint.DriverVersion != descriptor.Version {
		return fmt.Errorf("driver checkpoint belongs to %s@%s", checkpoint.DriverID, checkpoint.DriverVersion)
	}
	if len(checkpoint.State) > 0 && !json.Valid(checkpoint.State) {
		return errors.New("driver checkpoint state is not valid JSON")
	}
	return nil
}

func checkpointFor(descriptor Descriptor, state any) Checkpoint {
	encoded, _ := json.Marshal(state)
	return Checkpoint{
		ContractVersion: ContractVersion,
		DriverID:        descriptor.ID,
		DriverVersion:   descriptor.Version,
		State:           encoded,
	}
}

func terminalStatus(ctx context.Context, err error) TerminalStatus {
	if err == nil {
		return TerminalSucceeded
	}
	if ctx != nil && ctx.Err() != nil {
		return TerminalCanceled
	}
	return TerminalFailed
}

func cloneInput(input PersistedInput) PersistedInput {
	return PersistedInput{Messages: providers.CloneChatMessages(input.Messages)}
}

func cloneCheckpoint(checkpoint Checkpoint) Checkpoint {
	checkpoint.State = append(json.RawMessage(nil), checkpoint.State...)
	return checkpoint
}
