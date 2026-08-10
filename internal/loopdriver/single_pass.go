package loopdriver

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
)

const (
	SinglePassDriverID      = "wuu.single-pass"
	SinglePassDriverVersion = "0.1.0"
)

// SinglePassDriver is an experimental non-agentic driver. It performs exactly
// one model round with no tool surface or compaction retry.
type SinglePassDriver struct{}

func (SinglePassDriver) Descriptor() Descriptor {
	return Descriptor{
		ID:           SinglePassDriverID,
		Version:      SinglePassDriverVersion,
		Capabilities: []string{"single_round"},
	}
}

func (driver SinglePassDriver) Create(execution ExecutionContext, input PersistedInput) (Instance, error) {
	return newSinglePassInstance(driver.Descriptor(), execution, input, nil)
}

func (driver SinglePassDriver) Resume(execution ExecutionContext, input PersistedInput, checkpoint Checkpoint) (Instance, error) {
	if err := validateCheckpoint(driver.Descriptor(), checkpoint); err != nil {
		return nil, err
	}
	return newSinglePassInstance(driver.Descriptor(), execution, input, checkpoint.State)
}

type singlePassInstance struct {
	descriptor Descriptor
	execution  ExecutionContext
	input      PersistedInput

	mu         sync.Mutex
	checkpoint Checkpoint
	cancel     context.CancelCauseFunc
}

type singlePassCheckpointState struct {
	Phase         string `json:"phase"`
	ExecutionID   string `json:"execution_id,omitempty"`
	LastReceiptID string `json:"last_receipt_id,omitempty"`
}

func newSinglePassInstance(descriptor Descriptor, execution ExecutionContext, input PersistedInput, state json.RawMessage) (*singlePassInstance, error) {
	phase := "ready"
	if len(state) > 0 {
		var restored singlePassCheckpointState
		if err := json.Unmarshal(state, &restored); err != nil {
			return nil, errors.New("decode single-pass driver checkpoint: " + err.Error())
		}
		if restored.Phase != "" {
			phase = restored.Phase
		}
	}
	instance := &singlePassInstance{descriptor: descriptor, execution: execution, input: cloneInput(input)}
	instance.checkpoint = checkpointFor(descriptor, singlePassCheckpointState{Phase: phase, ExecutionID: execution.ExecutionID})
	return instance, nil
}

func (instance *singlePassInstance) Run(ctx context.Context, gateway KernelGateway) (TerminalOutcome, error) {
	runCtx, cancel := context.WithCancelCause(ctx)
	instance.mu.Lock()
	instance.cancel = cancel
	instance.mu.Unlock()
	defer cancel(nil)

	running := checkpointFor(instance.descriptor, singlePassCheckpointState{Phase: "running", ExecutionID: instance.execution.ExecutionID})
	instance.setCheckpoint(running)
	if err := gateway.WriteCheckpoint(runCtx, running); err != nil {
		return TerminalOutcome{Status: TerminalFailed, Checkpoint: running}, err
	}
	receipt, runErr := gateway.ExecuteModelLoop(runCtx, instance.input, LoopPolicy{
		ModelRoundLimit:   1,
		DisableTools:      true,
		DisableCompaction: true,
	})
	status := terminalStatus(runCtx, runErr)
	terminal := checkpointFor(instance.descriptor, singlePassCheckpointState{
		Phase:         string(status),
		ExecutionID:   instance.execution.ExecutionID,
		LastReceiptID: receipt.ID,
	})
	instance.setCheckpoint(terminal)
	return TerminalOutcome{Status: status, ReceiptID: receipt.ID, Checkpoint: terminal}, runErr
}

func (instance *singlePassInstance) Checkpoint() Checkpoint {
	instance.mu.Lock()
	defer instance.mu.Unlock()
	return cloneCheckpoint(instance.checkpoint)
}

func (instance *singlePassInstance) Cancel(reason string) {
	instance.mu.Lock()
	cancel := instance.cancel
	instance.mu.Unlock()
	if cancel != nil {
		cancel(errors.New(reason))
	}
}

func (instance *singlePassInstance) Shutdown() {
	instance.Cancel("driver shutdown")
}

func (instance *singlePassInstance) setCheckpoint(checkpoint Checkpoint) {
	instance.mu.Lock()
	instance.checkpoint = cloneCheckpoint(checkpoint)
	instance.mu.Unlock()
}
