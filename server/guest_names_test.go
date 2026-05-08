package server

import (
	"fmt"
	"testing"

	"github.com/evan-buss/openbooks/irc"
	"github.com/google/uuid"
)

func TestGuestNameWordListsIncludeRequestedAdjectives(t *testing.T) {
	t.Parallel()

	for _, adjective := range []string{"hairy", "eager", "moist"} {
		if !containsGuestWord(guestAdjectives, adjective) {
			t.Fatalf("guest adjectives missing %q", adjective)
		}
	}
}

func TestGuestNameWordListsHaveEnoughVariety(t *testing.T) {
	t.Parallel()

	if len(guestAdjectives) < 100 {
		t.Fatalf("guestAdjectives has %d words, want at least 100", len(guestAdjectives))
	}
	if len(guestAnimals) < 100 {
		t.Fatalf("guestAnimals has %d words, want at least 100", len(guestAnimals))
	}
}

func TestGuestNameWordListsAreIRCSafe(t *testing.T) {
	t.Parallel()

	assertValidGuestWords(t, "adjective", guestAdjectives)
	assertValidGuestWords(t, "animal", guestAnimals)
}

func TestGuestNameFromUUIDIsDeterministicAndValid(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("1f7f30f0-6076-439e-8d4d-439744894caf")
	name := guestNameFromUUID(userID)

	if name != guestNameFromUUID(userID) {
		t.Fatal("guest name should be deterministic for the same UUID")
	}
	if !validGeneratedUsername(name) {
		t.Fatalf("guest name %q is not IRC safe", name)
	}
}

func TestGuestNameFromUUIDVariesAcrossUUIDs(t *testing.T) {
	t.Parallel()

	names := make(map[string]struct{})
	for i := range 20 {
		userID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("guest-%d", i)))
		names[guestNameFromUUID(userID)] = struct{}{}
	}

	if len(names) < 2 {
		t.Fatal("guest names should vary across UUIDs")
	}
}

func TestGenerateUniqueUsernameUsesGuestNameWhenNameMissing(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("1f7f30f0-6076-439e-8d4d-439744894caf")
	server := New(Config{})

	got := server.generateUniqueUsername(userID)
	want := guestNameFromUUID(userID)
	if got != want {
		t.Fatalf("generateUniqueUsername() = %q, want %q", got, want)
	}
}

func TestGenerateUniqueUsernameTreatsWhitespaceNameAsMissing(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("1f7f30f0-6076-439e-8d4d-439744894caf")
	server := New(Config{UserName: "   "})

	got := server.generateUniqueUsername(userID)
	want := guestNameFromUUID(userID)
	if got != want {
		t.Fatalf("generateUniqueUsername() = %q, want %q", got, want)
	}
}

func TestGenerateUniqueUsernameAddsSuffixForActiveCollision(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("1f7f30f0-6076-439e-8d4d-439744894caf")
	otherID := uuid.MustParse("ecaaf931-90a9-4c16-8311-1337ccca25b7")
	baseName := guestNameFromUUID(userID)
	server := New(Config{})
	server.clients[otherID] = &Client{
		uuid: otherID,
		irc:  irc.New(baseName, "test"),
	}

	got := server.generateUniqueUsername(userID)
	if got == baseName {
		t.Fatalf("generateUniqueUsername() reused active name %q", got)
	}
	if !validGeneratedUsername(got) {
		t.Fatalf("generateUniqueUsername() returned unsafe username %q", got)
	}
	if len(got) > guestNameMaxLen {
		t.Fatalf("generateUniqueUsername() returned username length %d, want <= %d", len(got), guestNameMaxLen)
	}
}

func TestGenerateUniqueUsernameAllowsSameUUIDReconnectToReuseBaseName(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("1f7f30f0-6076-439e-8d4d-439744894caf")
	baseName := guestNameFromUUID(userID)
	server := New(Config{})
	server.clients[userID] = &Client{
		uuid: userID,
		irc:  irc.New(baseName, "test"),
	}

	got := server.generateUniqueUsername(userID)
	if got != baseName {
		t.Fatalf("generateUniqueUsername() = %q, want reconnect to keep %q", got, baseName)
	}
}

func TestGenerateUniqueUsernameKeepsManyActiveClientsUnique(t *testing.T) {
	t.Parallel()

	server := New(Config{})
	seen := make(map[string]struct{})

	for i := range 300 {
		userID := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("active-client-%d", i)))
		username := server.generateUniqueUsername(userID)
		if _, exists := seen[username]; exists {
			t.Fatalf("generated duplicate active username %q at client %d", username, i)
		}
		if !validGeneratedUsername(username) {
			t.Fatalf("generated unsafe username %q at client %d", username, i)
		}

		seen[username] = struct{}{}
		server.clients[userID] = &Client{
			uuid: userID,
			irc:  irc.New(username, "test"),
		}
	}
}

func TestGenerateUniqueUsernameSuffixesExplicitName(t *testing.T) {
	t.Parallel()

	userID := uuid.MustParse("1f7f30f0-6076-439e-8d4d-439744894caf")
	server := New(Config{UserName: "openbooks_abs_dev"})

	got := server.generateUniqueUsername(userID)
	if got == "openbooks_abs_dev" {
		t.Fatal("generateUniqueUsername() should suffix explicit server names for per-client IRC connections")
	}
	if len(got) > guestNameMaxLen {
		t.Fatalf("generateUniqueUsername() returned username length %d, want <= %d", len(got), guestNameMaxLen)
	}
}

func TestGeneratedUsernameValidationRejectsUnsafeNames(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"",
		"_slug",
		"Hairy_slug",
		"hairy-slug",
		"hairy slug",
		"hairy_slug!",
		"hairy_slug_extra_long",
	} {
		if validGeneratedUsername(name) {
			t.Fatalf("validGeneratedUsername(%q) = true, want false", name)
		}
	}
}

func containsGuestWord(words []string, want string) bool {
	for _, word := range words {
		if word == want {
			return true
		}
	}
	return false
}

func assertValidGuestWords(t *testing.T, label string, words []string) {
	t.Helper()

	seen := make(map[string]struct{}, len(words))
	for _, word := range words {
		if !validGuestNamePart(word) {
			t.Fatalf("%s word %q is not IRC safe", label, word)
		}
		if _, exists := seen[word]; exists {
			t.Fatalf("%s word %q is duplicated", label, word)
		}
		seen[word] = struct{}{}
	}
}
