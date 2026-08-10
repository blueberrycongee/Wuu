package runtime

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/blueberrycongee/wuu/internal/loopdriver"
	"github.com/blueberrycongee/wuu/internal/pluginhost"
	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
	"github.com/blueberrycongee/wuu/plugins/singlepass"
)

func TestSinglepassPluginProcessHelper(t *testing.T) {
	if os.Getenv("WUU_SINGLEPASS_PLUGIN_TEST_HELPER") != "1" {
		return
	}
	if err := pluginapi.Serve(context.Background(), singlepass.Handler()); err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}

// TestRemoteSinglepassDriverAcrossRealProcess installs the single-pass driver
// as a plugin process, selects it by profile through the registry, runs a
// turn whose model loop and checkpoint writes cross back into the kernel
// gateway services, then resumes from the persisted checkpoint.
func TestRemoteSinglepassDriverAcrossRealProcess(t *testing.T) {
	kernel := newKernelHostServices(nil, nil)
	registry, conflicts := pluginhost.BuildServiceRegistry(kernel)
	kernel.bindRegistry(registry)
	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %+v", conflicts)
	}
	// The plugin process routes its service calls into the registry, where
	// the kernel gateway services are already registered.
	client, err := pluginhost.Start(context.Background(), pluginhost.ProcessConfig{
		ID:            "singlepass",
		Command:       os.Args[0],
		Args:          []string{"-test.run=^TestSinglepassPluginProcessHelper$"},
		Env:           map[string]string{"WUU_SINGLEPASS_PLUGIN_TEST_HELPER": "1"},
		Timeout:       5 * time.Second,
		ServiceRouter: registry,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close(context.Background()) })
	if registrationConflicts := registry.RegisterClients(client); len(registrationConflicts) != 0 {
		t.Fatalf("registration conflicts = %+v", registrationConflicts)
	}
	host := pluginhost.New(client)
	host.AttachServiceRegistry(registry, nil)
	if err := host.Activate(context.Background()); err != nil {
		t.Fatal(err)
	}
	registry.Activate()

	driver := resolveLoopDriver("singlepass", host, func() *driverGatewayTable { return kernel.driverGateways })
	if _, ok := driver.(*loopdriver.RemoteDriver); !ok {
		t.Fatalf("driver = %#v, want *RemoteDriver", driver)
	}

	gateway := &recordingKernelGateway{}
	execution := loopdriver.ExecutionContext{SessionID: "s", ExecutionID: "exec-e2e"}
	instance, err := driver.Create(execution, loopdriver.PersistedInput{})
	if err != nil {
		t.Fatalf("Create() = %v", err)
	}
	outcome, err := instance.Run(context.Background(), gateway)
	if err != nil {
		t.Fatalf("Run() = %v", err)
	}
	if outcome.Status != loopdriver.TerminalSucceeded || outcome.ReceiptID != "model-loop-1" {
		t.Fatalf("outcome = %+v", outcome)
	}
	gateway.mu.Lock()
	modelLoops, checkpoints := gateway.modelLoops, gateway.checkpoints
	gateway.mu.Unlock()
	if modelLoops != 1 {
		t.Fatalf("model loop executions = %d, want 1", modelLoops)
	}
	if len(checkpoints) != 1 || checkpoints[0].DriverID != loopdriver.SinglePassDriverID {
		t.Fatalf("checkpoints = %+v", checkpoints)
	}

	// Resume from the persisted checkpoint in a fresh instance: the kernel
	// accepts only the same driver id/version, and the plugin rebuilds its
	// run state from the checkpoint alone.
	resumed, err := driver.Resume(execution, loopdriver.PersistedInput{}, outcome.Checkpoint)
	if err != nil {
		t.Fatalf("Resume() = %v", err)
	}
	resumeOutcome, err := resumed.Run(context.Background(), gateway)
	if err != nil {
		t.Fatalf("resumed Run() = %v", err)
	}
	if resumeOutcome.Status != loopdriver.TerminalSucceeded {
		t.Fatalf("resume outcome = %+v", resumeOutcome)
	}

	// A checkpoint from another driver stays fail-closed.
	foreign := outcome.Checkpoint
	foreign.DriverID = "other-driver"
	if _, err := driver.Resume(execution, loopdriver.PersistedInput{}, foreign); err == nil {
		t.Fatal("resume with a foreign checkpoint must fail")
	}
}

type recordingKernelGateway struct {
	mu          sync.Mutex
	modelLoops  int
	checkpoints []loopdriver.Checkpoint
}

func (g *recordingKernelGateway) ExecuteModelLoop(_ context.Context, _ loopdriver.PersistedInput, policy loopdriver.LoopPolicy) (loopdriver.ModelLoopReceipt, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.modelLoops++
	if !policy.DisableTools || policy.ModelRoundLimit != 1 {
		return loopdriver.ModelLoopReceipt{}, errors.New("single-pass policy was not preserved across the wire")
	}
	return loopdriver.ModelLoopReceipt{ID: "model-loop-1"}, nil
}

func (g *recordingKernelGateway) WriteCheckpoint(_ context.Context, checkpoint loopdriver.Checkpoint) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.checkpoints = append(g.checkpoints, checkpoint)
	return nil
}
