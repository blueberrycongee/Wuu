package pluginhost

import "testing"

func TestKernelDataSubscribeDescriptorIsValid(t *testing.T) {
	descriptor := KernelDataSubscribeDescriptor()
	if descriptor.Name != KernelDataSubscribeService || descriptor.Version != DataSubscribeServiceVersion {
		t.Fatalf("descriptor = %s@%s", descriptor.Name, descriptor.Version)
	}
	if len(descriptor.Methods) != 1 || descriptor.Methods[0].Name != KernelServiceMethod {
		t.Fatalf("methods = %+v", descriptor.Methods)
	}
}

func TestValidateDataSubscribeParams(t *testing.T) {
	if err := ValidateDataSubscribeParams(DataSubscribeParams{ThreadID: "t1", Limit: 2}); err != nil {
		t.Fatalf("valid params rejected: %v", err)
	}
	if err := ValidateDataSubscribeParams(DataSubscribeParams{ThreadID: ""}); err == nil {
		t.Fatal("missing thread_id accepted")
	}
	if err := ValidateDataSubscribeParams(DataSubscribeParams{ThreadID: "t1", Limit: -1}); err == nil {
		t.Fatal("negative limit accepted")
	}
}

func TestFilterDataEventsForSubscription(t *testing.T) {
	params := DataSubscribeParams{ThreadID: "t1", Types: []string{DataEventTypeToolCall}, TurnID: "turn-2"}
	cases := []struct {
		event DataEvent
		want  bool
	}{
		{DataEvent{Type: DataEventTypeToolCall, TurnID: "turn-2"}, true},
		{DataEvent{Type: DataEventTypeToolCall, TurnID: "turn-1"}, false},
		{DataEvent{Type: DataEventTypeTurn, TurnID: "turn-2"}, false},
	}
	for _, tc := range cases {
		if got := FilterDataEventsForSubscription(tc.event, params); got != tc.want {
			t.Fatalf("filter(%+v) = %v, want %v", tc.event, got, tc.want)
		}
	}
}
