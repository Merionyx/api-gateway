package permissions

import "testing"

func TestDescribe(t *testing.T) {
	t.Parallel()

	if got := Describe(Wildcard); got == "" {
		t.Fatalf("built-in permission %q has empty description", Wildcard)
	}
	if got := Describe("custom.permission"); got == "" {
		t.Fatal("custom permission description is empty")
	}
}

func TestListDescriptors_sortedUnique(t *testing.T) {
	t.Parallel()

	got := ListDescriptors()
	if len(got) == 0 {
		t.Fatal("empty descriptors")
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].ID >= got[i].ID {
			t.Fatalf("descriptors are not strictly sorted: %q then %q", got[i-1].ID, got[i].ID)
		}
	}
}
