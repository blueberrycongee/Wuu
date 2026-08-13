package pluginhost

import (
	"errors"
	"testing"
)

func TestValidateDataQueryParams(t *testing.T) {
	t.Parallel()

	valid := []DataQueryParams{
		{ThreadID: "thread-1"},
		{ThreadID: "thread_2.sub", Limit: 10},
		{ThreadID: "a-b_c.d", Limit: 0},
	}
	for _, params := range valid {
		if err := ValidateDataQueryParams(params); err != nil {
			t.Fatalf("ValidateDataQueryParams(%+v) = %v", params, err)
		}
	}

	invalid := []DataQueryParams{
		{},
		{ThreadID: "   "},
		{ThreadID: "../thread-1"},
		{ThreadID: "thread/1"},
		{ThreadID: "thread 1"},
		{ThreadID: "thread-1", Limit: -1},
	}
	for _, params := range invalid {
		err := ValidateDataQueryParams(params)
		if err == nil {
			t.Fatalf("ValidateDataQueryParams(%+v) = nil, want error", params)
		}
		var hostErr *HostServiceError
		if !errors.As(err, &hostErr) || hostErr.Code != "invalid_params" {
			t.Fatalf("ValidateDataQueryParams(%+v) error = %v, want invalid_params HostServiceError", params, err)
		}
	}
}

func TestKernelDataQueryDescriptorIsValid(t *testing.T) {
	t.Parallel()

	descriptor := KernelDataQueryDescriptor()
	if err := ValidateServiceDescriptor(descriptor); err != nil {
		t.Fatalf("ValidateServiceDescriptor(%+v) = %v", descriptor, err)
	}
	if descriptor.Name != KernelDataQueryService || descriptor.Version != DataQueryServiceVersion {
		t.Fatalf("descriptor = %+v", descriptor)
	}
}
