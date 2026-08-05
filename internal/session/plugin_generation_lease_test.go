package session

import "testing"

func TestPluginGenerationLeaseExcludesMutationFromExecutions(t *testing.T) {
	home := t.TempDir()
	first, acquired, err := TryAcquirePluginGenerationExecutionLease(home)
	if err != nil || !acquired {
		t.Fatalf("acquire first execution lease: acquired=%v err=%v", acquired, err)
	}
	defer first.Release()
	second, acquired, err := TryAcquirePluginGenerationExecutionLease(home)
	if err != nil || !acquired {
		t.Fatalf("acquire second execution lease: acquired=%v err=%v", acquired, err)
	}
	defer second.Release()
	mutation, acquired, err := TryAcquirePluginGenerationMutationLease(home)
	if err != nil {
		t.Fatal(err)
	}
	if acquired || mutation != nil {
		t.Fatal("mutation lease acquired while executions were active")
	}
	if err := first.Release(); err != nil {
		t.Fatal(err)
	}
	if err := second.Release(); err != nil {
		t.Fatal(err)
	}
	mutation, acquired, err = TryAcquirePluginGenerationMutationLease(home)
	if err != nil || !acquired {
		t.Fatalf("acquire mutation lease: acquired=%v err=%v", acquired, err)
	}
	defer mutation.Release()
	epoch, err := mutation.Advance()
	if err != nil {
		t.Fatal(err)
	}
	if epoch != 1 {
		t.Fatalf("epoch = %d, want 1", epoch)
	}
	blocked, acquired, err := TryAcquirePluginGenerationExecutionLease(home)
	if err != nil {
		t.Fatal(err)
	}
	if acquired || blocked != nil {
		t.Fatal("execution lease acquired while mutation was active")
	}
	if err := mutation.Release(); err != nil {
		t.Fatal(err)
	}
	next, acquired, err := TryAcquirePluginGenerationExecutionLease(home)
	if err != nil || !acquired {
		t.Fatalf("acquire execution after mutation: acquired=%v err=%v", acquired, err)
	}
	defer next.Release()
	if next.Epoch() != 1 {
		t.Fatalf("persisted epoch = %d, want 1", next.Epoch())
	}
}
