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

func TestServiceRegistryCloseDrainsRemoteSinglepassExecution(t *testing.T) {
	kernel := newKernelHostServices(nil, nil)
	registry, conflicts := pluginhost.BuildServiceRegistry(kernel)
	kernel.bindRegistry(registry)
	if len(conflicts) != 0 {
		t.Fatalf("conflicts = %+v", conflicts)
	}
	client, err := pluginhost.Start(context.Background(), pluginhost.ProcessConfig{
		ID: "singlepass", Command: os.Args[0],
		Args:    []string{"-test.run=^TestSinglepassPluginProcessHelper$"},
		Env:     map[string]string{"WUU_SINGLEPASS_PLUGIN_TEST_HELPER": "1"},
		Timeout: 5 * time.Second, ServiceRouter: registry,
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
	execution := loopdriver.ExecutionContext{SessionID: "s", ExecutionID: "exec-drain-e2e"}
	instance, err := driver.Create(execution, loopdriver.PersistedInput{})
	if err != nil {
		t.Fatal(err)
	}
	gateway := &blockingKernelGateway{started: make(chan struct{}), release: make(chan struct{})}
	runDone := make(chan error, 1)
	go func() {
		_, runErr := instance.Run(context.Background(), gateway)
		runDone <- runErr
	}()
	<-gateway.started
	closeDone := make(chan struct{})
	go func() {
		registry.Close(context.Background())
		close(closeDone)
	}()
	select {
	case <-closeDone:
		t.Fatal("registry closed while remote driver execution was in flight")
	case <-time.After(20 * time.Millisecond):
	}
	close(gateway.release)
	if runErr := <-runDone; runErr != nil {
		t.Fatalf("remote driver failed while generation drained: %v", runErr)
	}
	select {
	case <-closeDone:
	case <-time.After(time.Second):
		t.Fatal("registry did not close after remote driver completed")
	}
}

type recordingKernelGateway struct {
	mu          sync.Mutex
	modelLoops  int
	checkpoints []loopdriver.Checkpoint
}

type blockingKernelGateway struct {
	started chan struct{}
	release chan struct{}
}

func (g *blockingKernelGateway) ExecuteModelLoop(context.Context, loopdriver.PersistedInput, loopdriver.LoopPolicy) (loopdriver.ModelLoopReceipt, error) {
	close(g.started)
	<-g.release
	return loopdriver.ModelLoopReceipt{ID: "drained"}, nil
}

func (g *blockingKernelGateway) WriteCheckpoint(context.Context, loopdriver.Checkpoint) error {
	return nil
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
