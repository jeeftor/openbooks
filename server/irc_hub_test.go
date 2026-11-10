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

// rescuedSearch simulates one full search cycle in which the primary bot
// times out and the query is rescued by the fallback: dispatch to primary,
// watchdog fires (auto-retries to fallback), fallback resolves the search.
// The primary failure streak is preserved — fallback resolution does not
// reset it.
func rescuedSearch(t *testing.T, h *ircHub, sess *session, query string) {
	t.Helper()
	st, _ := h.prepareSearch(sess, query)
	if st.bot != "search" {
		t.Fatalf("expected primary bot 'search', got %q", st.bot)
	}
	if !h.searchTimedOut(st) {
		t.Fatal("expected primary timeout to re-arm onto fallback")
	}
	if st.bot != "searchook" || !st.retried {
		t.Fatalf("expected retried fallback search, got bot=%q retried=%v", st.bot, st.retried)
	}
	resolveCurrent(h)
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

func TestPrimaryTimeoutRetriesFallback(t *testing.T) {
	h := newTestHub(t)
	sess := newSession("test")

	st, _ := h.prepareSearch(sess, "dune")
	if !h.searchTimedOut(st) {
		t.Fatal("expected retry onto fallback, got final failure")
	}
	if st.bot != "searchook" || !st.retried || st.phase != searchAwaitingResponse {
		t.Fatalf("unexpected retry state: bot=%q retried=%v phase=%v", st.bot, st.retried, st.phase)
	}
	if h.current != st {
		t.Fatal("in-flight search should survive the retry")
	}
	if h.searchDegraded {
		t.Fatal("single timeout must not degrade")
	}
}

func TestRetriedSearchFailsAfterFallbackTimeout(t *testing.T) {
	h := newTestHub(t)
	sess := newSession("test")

	st, _ := h.prepareSearch(sess, "q")
	h.searchTimedOut(st) // → retry onto fallback
	if st.bot != "searchook" {
		t.Fatalf("expected fallback after retry, got %q", st.bot)
	}
	// Fallback attempt times out → final failure, no further retry.
	if h.searchTimedOut(st) {
		t.Fatal("fallback timeout must be final, not another retry")
	}
	if h.peekCurrentSearch() != nil {
		t.Fatal("expected in-flight search cleared after final timeout")
	}
}

func TestRescuedSearchesDegradeToFallback(t *testing.T) {
	h := newTestHub(t)
	sess := newSession("test")

	// First rescued search must not degrade.
	rescuedSearch(t, h, sess, "q")
	if h.searchDegraded {
		t.Fatal("single rescued timeout must not degrade")
	}

	// Remaining strikes → degraded.
	for i := 1; i < searchDegradedThreshold; i++ {
		rescuedSearch(t, h, sess, "q")
	}
	if !h.searchDegraded {
		t.Fatal("expected degraded after consecutive primary timeouts")
	}

	// Degraded → straight to fallback, no wasted primary attempt.
	st, _ := h.prepareSearch(sess, "q")
	if st.bot != "searchook" || st.probe {
		t.Fatalf("expected fallback while degraded, got bot=%q probe=%v", st.bot, st.probe)
	}
}

func TestDegradedCooldownProbesPrimaryAndRecovers(t *testing.T) {
	h := newTestHub(t)
	sess := newSession("test")

	// Drive into degraded state (each search rescued by fallback).
	for i := 0; i < searchDegradedThreshold; i++ {
		rescuedSearch(t, h, sess, "q")
	}
	if !h.searchDegraded {
		t.Fatal("expected degraded")
	}

	// During cooldown → fallback directly; fallback success keeps degraded.
	st, _ := h.prepareSearch(sess, "q")
	if st.bot != "searchook" {
		t.Fatalf("expected fallback during cooldown, got %q", st.bot)
	}
	resolveCurrent(h)
	if !h.searchDegraded {
		t.Fatal("fallback resolution must not clear degraded state")
	}

	// Cooldown expires → probe primary.
	expireCooldown(h)
	probe, _ := h.prepareSearch(sess, "q")
	if probe.bot != "search" || !probe.probe {
		t.Fatalf("expected primary probe after cooldown, got bot=%q probe=%v", probe.bot, probe.probe)
	}

	// Probe times out → immediate re-degrade, and this search retries fallback.
	if !h.searchTimedOut(probe) {
		t.Fatal("expected probe timeout to retry fallback")
	}
	if !h.searchDegraded {
		t.Fatal("expected re-degrade after probe timeout")
	}
	if probe.bot != "searchook" {
		t.Fatalf("expected probe retry onto fallback, got %q", probe.bot)
	}
	resolveCurrent(h)
	if !h.searchDegraded {
		t.Fatal("fallback resolution after probe must not clear degraded")
	}

	// Next probe resolves on the primary → recovery.
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
	// A late timeout for the superseded search must not touch the new one.
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
