package server

import (
	"testing"
)

func TestSeriesRegistryAddIfNewDeduplicates(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newSeriesRegistry(dir)

	r.AddIfNew("The Expanse")
	r.AddIfNew("The Expanse")
	r.AddIfNew("Discworld")

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("All() len = %d, want 2 (got %v)", len(all), all)
	}
}

func TestSeriesRegistryIgnoresBlankNames(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newSeriesRegistry(dir)

	r.AddIfNew("")
	r.AddIfNew("   ")
	r.AddIfNew("Dune")

	all := r.All()
	if len(all) != 1 {
		t.Fatalf("All() len = %d, want 1 (got %v)", len(all), all)
	}
	if all[0] != "Dune" {
		t.Fatalf("All()[0] = %q, want %q", all[0], "Dune")
	}
}

func TestSeriesRegistryAllReturnsSorted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	r := newSeriesRegistry(dir)

	for _, name := range []string{"Wheel of Time", "Dune", "Foundation", "Discworld"} {
		r.AddIfNew(name)
	}

	all := r.All()
	want := []string{"Discworld", "Dune", "Foundation", "Wheel of Time"}
	if len(all) != len(want) {
		t.Fatalf("All() len = %d, want %d", len(all), len(want))
	}
	for i, w := range want {
		if all[i] != w {
			t.Fatalf("All()[%d] = %q, want %q", i, all[i], w)
		}
	}
}

func TestSeriesRegistryPersistsAndReloads(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	r1 := newSeriesRegistry(dir)
	r1.AddIfNew("The Expanse")
	r1.AddIfNew("Dune")

	r2 := newSeriesRegistry(dir)
	all := r2.All()
	if len(all) != 2 {
		t.Fatalf("reloaded All() len = %d, want 2 (got %v)", len(all), all)
	}
	want := map[string]bool{"The Expanse": true, "Dune": true}
	for _, name := range all {
		if !want[name] {
			t.Fatalf("unexpected series %q in reloaded registry", name)
		}
	}
}
