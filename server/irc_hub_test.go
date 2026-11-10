package server

import (
	"context"
	"testing"
	"time"
)

func newTestHub(t *testing.T) *ircHub {
	t.Helper()
	srv := New(Config{
		SearchBot:             "search",
		SearchBotFallback:     "searchook",
		SearchResponseTimeout: time.Second,
		SearchResultTimeout:   2 * time.Second,
	})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return newIrcHub(srv, ctx)
}

// expireCooldown moves the degraded window into the past so the next
// prepareSearch probes the primary bot.
func expireCooldown(h *ircHub) {
	h.currentMu.Lock()
	h.degradedUntil = time.Now().Add(-time.Minute)
	h.currentMu.Unlock()
}

// resolveCurrent pops the in-flight search and records its outcome, mirroring
// what the terminal event handlers do. Returns the recovered flag.
func resolveCurrent(h *ircHub) bool {
	_, st := h.popCurrentSearch()
	if st == nil {
		return false
	}
	return h.recordSearchOutcome(st)
}

func TestPrepareSearchUsesPrimaryWhenHealthy(t *testing.T) {
	h := newTestHub(t)
	sess := newSession("test")

	st, superseded := h.prepareSearch(sess, "dune")
	if len(superseded) != 0 {
		t.Fatalf("expected no superseded searches, got %d", len(superseded))
	}
	if st.bot != "search" || st.probe {
		t.Fatalf("expected primary bot 'search' non-probe, got bot=%q probe=%v", st.bot, st.probe)
	}
	if st.phase != searchAwaitingResponse {
		t.Fatalf("expected phase awaitingResponse, got %v", st.phase)
	}
}

func TestSearchTimeoutDegradesToFallback(t *testing.T) {
	h := newTestHub(t)
	sess := newSession("test")

	for i := 0; i < searchDegradedThreshold; i++ {
		st, _ := h.prepareSearch(sess, "q")
		if st.bot != "search" {
			t.Fatalf("search %d: expected primary bot before degradation, got %q", i, st.bot)
		}
		h.searchTimedOut(st) // clears h.current itself
	}

	if !h.searchDegraded {
		t.Fatal("expected hub to be degraded after consecutive timeouts")
	}

	st, _ := h.prepareSearch(sess, "q")
	if st.bot != "searchook" {
		t.Fatalf("expected fallback bot 'searchook' while degraded, got %q", st.bot)
	}
	if st.probe {
		t.Fatal("fallback search must not be marked as a primary probe")
	}
}

func TestDegradedCooldownProbesPrimaryAndRecovers(t *testing.T) {
	h := newTestHub(t)
	sess := newSession("test")

	// Drive into degraded state.
	for i := 0; i < searchDegradedThreshold; i++ {
		st, _ := h.prepareSearch(sess, "q")
		h.searchTimedOut(st)
	}

	// During cooldown → fallback.
	st, _ := h.prepareSearch(sess, "q")
	if st.bot != "searchook" {
		t.Fatalf("expected fallback during cooldown, got %q", st.bot)
	}

	// Expire the cooldown → next search probes primary.
	expireCooldown(h)
	probe, _ := h.prepareSearch(sess, "q")
	if probe.bot != "search" || !probe.probe {
		t.Fatalf("expected primary probe after cooldown, got bot=%q probe=%v", probe.bot, probe.probe)
	}

	// Probe times out → immediately re-degraded (no need for 3 more strikes).
	h.searchTimedOut(probe)
	if !h.searchDegraded {
		t.Fatal("expected re-degrade after probe timeout")
	}

	// Fallback search succeeds → stays degraded.
	h.prepareSearch(sess, "q")
	if resolveCurrent(h) {
		t.Fatal("fallback success must not report recovery")
	}
	if !h.searchDegraded {
		t.Fatal("fallback success should not clear degraded state")
	}

	// Next probe succeeds → recovery.
	expireCooldown(h)
	probe2, _ := h.prepareSearch(sess, "q")
	if probe2.bot != "search" || !probe2.probe {
		t.Fatalf("expected second primary probe, got bot=%q probe=%v", probe2.bot, probe2.probe)
	}
	if !resolveCurrent(h) {
		t.Fatal("expected recovery=true when primary probe resolves")
	}
	if h.searchDegraded {
		t.Fatal("expected degraded state cleared after primary recovery")
	}
}

func TestSearchProgressAdvancesPhase(t *testing.T) {
	h := newTestHub(t)
	sess := newSession("test")

	st, _ := h.prepareSearch(sess, "q")
	h.searchProgress()
	if st.phase != searchAwaitingResult {
		t.Fatalf("expected phase awaitingResult after progress, got %v", st.phase)
	}
	select {
	case <-st.rearm:
	default:
		t.Fatal("expected rearm signal after phase advance")
	}
	// Second progress is a no-op.
	h.searchProgress()
}

func TestSupersededSearchIsClosedOut(t *testing.T) {
	h := newTestHub(t)
	sess := newSession("test")

	st1, _ := h.prepareSearch(sess, "first")
	st2, superseded := h.prepareSearch(sess, "second")
	if len(superseded) != 1 || superseded[0] != st1 {
		t.Fatalf("expected st1 to be superseded, got %v", superseded)
	}
	select {
	case <-st1.done:
	default:
		t.Fatal("expected superseded search done channel to be closed")
	}
	if h.current != st2 {
		t.Fatal("expected st2 to be current")
	}
	// A late timeout for the superseded search must not clear the new one.
	h.searchTimedOut(st1)
	if h.current != st2 {
		t.Fatal("superseded search timeout cleared the in-flight search")
	}
}

func TestPopCurrentSearchStopsWatchdog(t *testing.T) {
	h := newTestHub(t)
	sess := newSession("test")

	st, _ := h.prepareSearch(sess, "q")
	clients, popped := h.popCurrentSearch()
	if popped != st || clients != nil {
		t.Fatalf("unexpected pop result: st=%v clients=%v", popped, clients)
	}
	select {
	case <-st.done:
	default:
		t.Fatal("expected done channel closed on pop")
	}
	if h.peekCurrentSearch() != nil {
		t.Fatal("expected no in-flight search after pop")
	}
}
