package pluginhost

import (
	"encoding/json"
	"reflect"
	"testing"

	pluginapi "github.com/blueberrycongee/wuu/packages/plugin-go"
)

func TestPublicSecurityServiceDescriptorsMatchKernelContract(t *testing.T) {
	for _, tc := range []struct {
		internal ServiceDescriptor
		public   pluginapi.Service
	}{
		{internal: SecurityAuthorizeDescriptor(), public: pluginapi.AuthorizationService()},
		{internal: ProcessSandboxDescriptor(), public: pluginapi.ProcessSandboxProviderService()},
	} {
		internalJSON, err := json.Marshal(tc.internal)
		if err != nil {
			t.Fatal(err)
		}
		publicJSON, err := json.Marshal(tc.public)
		if err != nil {
			t.Fatal(err)
		}
		var internalValue, publicValue any
		if err := json.Unmarshal(internalJSON, &internalValue); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(publicJSON, &publicValue); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(internalValue, publicValue) {
			t.Fatalf("descriptor mismatch:\ninternal=%s\npublic=%s", internalJSON, publicJSON)
		}
	}
}
