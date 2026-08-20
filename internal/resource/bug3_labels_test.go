package resource

import (
	"path/filepath"
	"task106/internal/model"
	"task106/internal/storage"
	"testing"
)

// newTestManager builds a manager backed by a fresh SQLite DB with one labeled
// resource ("prod") and one child ("prod/child") registered.
func newTestManager(t *testing.T) (*Manager, func()) {
	t.Helper()
	store, err := storage.New(filepath.Join(t.TempDir(), "labels.db"))
	if err != nil {
		t.Fatal(err)
	}
	m := NewManager(store)
	if err := m.Start(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Register(model.ResourceCreateRequest{Path: "prod", Owner: "ops", Labels: map[string]string{"tier": "gold"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Register(model.ResourceCreateRequest{Path: "prod/child", Owner: "ops", Labels: map[string]string{"tier": "gold"}}); err != nil {
		t.Fatal(err)
	}
	return m, func() { _ = store.Close() }
}

// TestResourceGetDoesNotLeakLabels guards the original bug3 fix: mutating the
// labels of a value returned by Get must not affect the internal cache.
func TestResourceGetDoesNotLeakLabels(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	got, err := m.Get("prod")
	if err != nil {
		t.Fatal(err)
	}
	got.Labels["tier"] = "silver"

	again, err := m.Get("prod")
	if err != nil {
		t.Fatal(err)
	}
	if again.Labels["tier"] != "gold" {
		t.Fatalf("Get exposed internal labels map: %#v", again.Labels)
	}
}

// TestResourceListDoesNotLeakLabels guards that List returns resources whose
// labels are independent copies.
func TestResourceListDoesNotLeakLabels(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	list, err := m.List("")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range list {
		if r.Labels != nil {
			r.Labels["tier"] = "silver"
		}
	}

	again, err := m.Get("prod")
	if err != nil {
		t.Fatal(err)
	}
	if again.Labels["tier"] != "gold" {
		t.Fatalf("List exposed internal labels map: %#v", again.Labels)
	}
}

// TestResourceEnsureDoesNotLeakLabels guards the cache-hit branch of Ensure.
func TestResourceEnsureDoesNotLeakLabels(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	got, err := m.Ensure("prod", "ops")
	if err != nil {
		t.Fatal(err)
	}
	got.Labels["tier"] = "silver"

	again, err := m.Get("prod")
	if err != nil {
		t.Fatal(err)
	}
	if again.Labels["tier"] != "gold" {
		t.Fatalf("Ensure exposed internal labels map: %#v", again.Labels)
	}
}

// TestResourceChildrenDoesNotLeakLabels guards Children.
func TestResourceChildrenDoesNotLeakLabels(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	kids := m.Children("prod")
	if len(kids) == 0 {
		t.Fatal("expected at least one child")
	}
	for _, r := range kids {
		r.Labels["tier"] = "silver"
	}

	again, err := m.Get("prod/child")
	if err != nil {
		t.Fatal(err)
	}
	if again.Labels["tier"] != "gold" {
		t.Fatalf("Children exposed internal labels map: %#v", again.Labels)
	}
}

// TestResourceDescendantsDoesNotLeakLabels guards Descendants.
func TestResourceDescendantsDoesNotLeakLabels(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	ds := m.Descendants("prod")
	if len(ds) == 0 {
		t.Fatal("expected at least one descendant")
	}
	for _, r := range ds {
		r.Labels["tier"] = "silver"
	}

	again, err := m.Get("prod/child")
	if err != nil {
		t.Fatal(err)
	}
	if again.Labels["tier"] != "gold" {
		t.Fatalf("Descendants exposed internal labels map: %#v", again.Labels)
	}
}

// TestResourceSetStateDoesNotLeakLabels guards that the resource returned by a
// state transition shares no map with the cache.
func TestResourceSetStateDoesNotLeakLabels(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	got, err := m.SetState("prod", model.ResourceDraining, "planned maintenance")
	if err != nil {
		t.Fatal(err)
	}
	got.Labels["tier"] = "silver"

	again, err := m.Get("prod")
	if err != nil {
		t.Fatal(err)
	}
	if again.Labels["tier"] != "gold" {
		t.Fatalf("SetState exposed internal labels map: %#v", again.Labels)
	}
}

// TestResourceRegisterClonesInputLabels guards that the caller's input label
// map is not aliased into the cache: mutating it after registration must not
// change the stored (or subsequently read) labels.
func TestResourceRegisterClonesInputLabels(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	in := map[string]string{"tier": "gold"}
	if _, err := m.Register(model.ResourceCreateRequest{Path: "prod/payments", Owner: "payments", Labels: in}); err != nil {
		t.Fatal(err)
	}
	in["tier"] = "silver" // caller mutates the input map after registration

	again, err := m.Get("prod/payments")
	if err != nil {
		t.Fatal(err)
	}
	if again.Labels["tier"] != "gold" {
		t.Fatalf("Register aliased caller input labels into the cache: %#v", again.Labels)
	}
}

// TestResourceRegisterReturnsIndependentLabels guards that the resource
// returned by Register itself does not share its labels map with the cache.
func TestResourceRegisterReturnsIndependentLabels(t *testing.T) {
	m, cleanup := newTestManager(t)
	defer cleanup()

	got, err := m.Register(model.ResourceCreateRequest{Path: "prod/billing", Owner: "billing", Labels: map[string]string{"tier": "gold"}})
	if err != nil {
		t.Fatal(err)
	}
	got.Labels["tier"] = "silver"

	again, err := m.Get("prod/billing")
	if err != nil {
		t.Fatal(err)
	}
	if again.Labels["tier"] != "gold" {
		t.Fatalf("Register returned a resource sharing labels with the cache: %#v", again.Labels)
	}
}
