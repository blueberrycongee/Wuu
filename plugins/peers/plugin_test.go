package peers

import (
	"encoding/json"
	"testing"
)

func TestListPeersPublishesArrayRequiredSchema(t *testing.T) {
	tools := Handler().Definition.Tools
	if len(tools) == 0 || tools[0].ID != "list_peers" {
		t.Fatalf("tools = %+v", tools)
	}
	raw, err := json.Marshal(tools[0].InputSchema)
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	required, ok := schema["required"].([]any)
	if !ok || len(required) != 0 {
		t.Fatalf("list_peers required = %#v, schema = %s", schema["required"], raw)
	}
}
