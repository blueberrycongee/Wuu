// Package singlepass runs the single-pass loop driver inside a plugin
// process: the kernel selects it per session through the versioned
// "driver.singlepass" service, and every model-loop execution or checkpoint
// write calls back into the kernel gateway services. The plugin never sees a
// Go session object; it only exchanges the driver protocol DTOs.
package singlepass

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/blueberrycongee/wuu/internal/loopdriver"
	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

// DriverService is the stable registry name a session's driver profile
// resolves to: profile "singlepass" binds this service at major version 1.
const DriverService = "driver.singlepass"

func driverServiceDescriptor() pluginapi.Service {
	method := func(name string) pluginapi.ServiceMethod {
		return pluginapi.ServiceMethod{
			Name:         name,
			InputSchema:  "driver.singlepass." + name + ".request.v1",
			OutputSchema: "driver.singlepass." + name + ".response.v1",
		}
	}
	return pluginapi.Service{
		Name:    DriverService,
		Version: "1.0.0",
		Methods: []pluginapi.ServiceMethod{
			method(loopdriver.RemoteMethodDescriptor),
			method(loopdriver.RemoteMethodCreate),
			method(loopdriver.RemoteMethodResume),
			method(loopdriver.RemoteMethodRun),
			method(loopdriver.RemoteMethodShutdown),
		},
	}
}

// Handler exposes the single-pass driver as one plugin process.
func Handler() pluginapi.Handler {
	c := &controller{driver: loopdriver.SinglePassDriver{}, instances: make(map[string]driverInstance)}
	return pluginapi.Handler{
		Definition: pluginapi.Definition{
			ProvidedServices: []pluginapi.Service{driverServiceDescriptor()},
			RequiredServices: pluginapi.RequireHostServices(
				loopdriver.DriverModelLoopService,
				loopdriver.DriverCheckpointService,
			),
		},
		Initialize:    c.initialize,
		InvokeService: c.invokeService,
	}
}

type driverInstance struct {
	instance    loopdriver.Instance
	executionID string
}

type controller struct {
	driver loopdriver.SinglePassDriver

	mu        sync.Mutex
	host      pluginapi.Host
	instances map[string]driverInstance
	next      int
}

func (c *controller) initialize(_ context.Context, host pluginapi.Host, _ pluginapi.InitializeParams) error {
	if host == nil {
		return errors.New("singlepass host is required")
	}
	c.mu.Lock()
	c.host = host
	c.mu.Unlock()
	return nil
}

type executionWire struct {
	SessionID   string `json:"session_id"`
	ExecutionID string `json:"execution_id"`
}

type createParams struct {
	Execution executionWire             `json:"execution"`
	Input     loopdriver.PersistedInput `json:"input"`
}

type resumeParams struct {
	Execution  executionWire             `json:"execution"`
	Input      loopdriver.PersistedInput `json:"input"`
	Checkpoint loopdriver.Checkpoint     `json:"checkpoint"`
}

type instanceResult struct {
	InstanceID string                `json:"instance_id"`
	Checkpoint loopdriver.Checkpoint `json:"checkpoint"`
}

type runParams struct {
	InstanceID string `json:"instance_id"`
}

type runResult struct {
	Status     loopdriver.TerminalStatus `json:"status"`
	ReceiptID  string                    `json:"receipt_id,omitempty"`
	Checkpoint loopdriver.Checkpoint     `json:"checkpoint"`
}

func (c *controller) invokeService(ctx context.Context, _ pluginapi.Host, call pluginapi.ServiceCall) (json.RawMessage, error) {
	if call.Service != DriverService {
		return nil, fmt.Errorf("singlepass plugin does not provide service %q", call.Service)
	}
	switch call.Method {
	case loopdriver.RemoteMethodDescriptor:
		return json.Marshal(c.driver.Descriptor())
	case loopdriver.RemoteMethodCreate:
		var params createParams
		if err := json.Unmarshal(call.Params, &params); err != nil {
			return nil, fmt.Errorf("driver.singlepass create params: %w", err)
		}
		return c.openInstance(params.Execution, func(execution loopdriver.ExecutionContext) (loopdriver.Instance, error) {
			return c.driver.Create(execution, params.Input)
		})
	case loopdriver.RemoteMethodResume:
		var params resumeParams
		if err := json.Unmarshal(call.Params, &params); err != nil {
			return nil, fmt.Errorf("driver.singlepass resume params: %w", err)
		}
		return c.openInstance(params.Execution, func(execution loopdriver.ExecutionContext) (loopdriver.Instance, error) {
			return c.driver.Resume(execution, params.Input, params.Checkpoint)
		})
	case loopdriver.RemoteMethodRun:
		var params runParams
		if err := json.Unmarshal(call.Params, &params); err != nil {
			return nil, fmt.Errorf("driver.singlepass run params: %w", err)
		}
		return c.runInstance(ctx, params.InstanceID)
	case loopdriver.RemoteMethodShutdown:
		var params runParams
		if err := json.Unmarshal(call.Params, &params); err != nil {
			return nil, fmt.Errorf("driver.singlepass shutdown params: %w", err)
		}
		c.dropInstance(params.InstanceID)
		return json.RawMessage(`{}`), nil
	}
	return nil, fmt.Errorf("driver.singlepass does not declare method %q", call.Method)
}

func (c *controller) openInstance(wire executionWire, open func(loopdriver.ExecutionContext) (loopdriver.Instance, error)) (json.RawMessage, error) {
	execution := loopdriver.ExecutionContext{SessionID: wire.SessionID, ExecutionID: wire.ExecutionID}
	if execution.ExecutionID == "" {
		return nil, errors.New("driver.singlepass requires a non-empty execution id")
	}
	instance, err := open(execution)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.next++
	instanceID := fmt.Sprintf("%s-%d", execution.ExecutionID, c.next)
	c.instances[instanceID] = driverInstance{instance: instance, executionID: execution.ExecutionID}
	c.mu.Unlock()
	return json.Marshal(instanceResult{InstanceID: instanceID, Checkpoint: instance.Checkpoint()})
}

func (c *controller) runInstance(ctx context.Context, instanceID string) (json.RawMessage, error) {
	c.mu.Lock()
	entry, ok := c.instances[instanceID]
	host := c.host
	c.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("driver.singlepass has no instance %q", instanceID)
	}
	outcome, err := entry.instance.Run(ctx, &serviceGateway{host: host, executionID: entry.executionID})
	result := runResult{Status: outcome.Status, ReceiptID: outcome.ReceiptID, Checkpoint: outcome.Checkpoint}
	encoded, encodeErr := json.Marshal(result)
	if encodeErr != nil {
		return nil, encodeErr
	}
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func (c *controller) dropInstance(instanceID string) {
	c.mu.Lock()
	entry, ok := c.instances[instanceID]
	delete(c.instances, instanceID)
	c.mu.Unlock()
	if ok {
		entry.instance.Shutdown()
	}
}

// serviceGateway adapts the plugin host's kernel service calls to the
// loopdriver.KernelGateway the in-process driver logic already works against.
type serviceGateway struct {
	host        pluginapi.Host
	executionID string
}

func (g *serviceGateway) ExecuteModelLoop(ctx context.Context, input loopdriver.PersistedInput, policy loopdriver.LoopPolicy) (loopdriver.ModelLoopReceipt, error) {
	var result loopdriver.DriverModelLoopResult
	if err := pluginapi.CallHostService(ctx, g.host, loopdriver.DriverModelLoopService, loopdriver.DriverModelLoopParams{
		ExecutionID: g.executionID,
		Input:       input,
		Policy:      policy,
	}, &result); err != nil {
		return loopdriver.ModelLoopReceipt{}, err
	}
	return loopdriver.ModelLoopReceipt{ID: result.ReceiptID}, nil
}

func (g *serviceGateway) WriteCheckpoint(ctx context.Context, checkpoint loopdriver.Checkpoint) error {
	return pluginapi.CallHostService(ctx, g.host, loopdriver.DriverCheckpointService, loopdriver.DriverCheckpointParams{
		ExecutionID: g.executionID,
		Checkpoint:  checkpoint,
	}, nil)
}
