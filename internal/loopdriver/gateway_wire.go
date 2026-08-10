package loopdriver

// Wire DTOs for the kernel-provided gateway services a remote driver calls
// back on: "driver.model_loop" (execute) and "driver.checkpoint" (write).
// Both are scoped by the execution id the kernel assigned to the driver run;
// the kernel routes each call to the gateway registered for that execution
// and rejects calls from any plugin other than the execution's owner.

const (
	DriverModelLoopService  = "driver.model_loop"
	DriverCheckpointService = "driver.checkpoint"

	DriverModelLoopMethod  = "execute"
	DriverCheckpointMethod = "write"
)

type DriverModelLoopParams struct {
	ExecutionID string         `json:"execution_id"`
	Input       PersistedInput `json:"input"`
	Policy      LoopPolicy     `json:"policy"`
}

type DriverModelLoopResult struct {
	ReceiptID string `json:"receipt_id"`
}

type DriverCheckpointParams struct {
	ExecutionID string     `json:"execution_id"`
	Checkpoint  Checkpoint `json:"checkpoint"`
}

type DriverCheckpointResult struct{}
