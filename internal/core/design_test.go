package core

import (
	"strings"
	"testing"

	"github.com/chenhongyang/novel-studio/internal/domain"
	"github.com/chenhongyang/novel-studio/internal/store"
)

func TestDesignArtifactRefBindsPayloadAndHardInputs(t *testing.T) {
	a := []byte(`{"schema_version":1,"artifact_type":"story_concept","inputs":["creative_brief@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"],"sources":[],"payload":{"story":"同一个故事"}}`)
	b := []byte(`{
      "payload":{"story":"同一个故事"},
      "sources":[],
      "inputs":["creative_brief@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"],
      "artifact_type":"story_concept",
      "schema_version":1
    }`)

	_, _, refA, err := canonicalDesignArtifact(a)
	if err != nil {
		t.Fatal(err)
	}
	_, _, refB, err := canonicalDesignArtifact(b)
	if err != nil {
		t.Fatal(err)
	}
	if refA == refB {
		t.Fatalf("hard input change did not change artifact ref: %s", refA)
	}
}

func TestDesignBundleRefIsStableAcrossJSONFormatting(t *testing.T) {
	a := []byte(`{"schema_version":1,"selections":{"story_concept":"story_concept@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`)
	b := []byte("{\n  \"selections\": {\"story_concept\": \"story_concept@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"},\n  \"schema_version\": 1\n}\n")

	_, _, refA, err := canonicalDesignBundle(a)
	if err != nil {
		t.Fatal(err)
	}
	_, _, refB, err := canonicalDesignBundle(b)
	if err != nil {
		t.Fatal(err)
	}
	if refA != refB {
		t.Fatalf("same bundle got refs %q and %q", refA, refB)
	}
}

func TestDesignStoreImmutableAndHeadRoundTrip(t *testing.T) {
	coreStore := store.NewCoreStore(t.TempDir())
	digest := strings.Repeat("a", 64)
	original := []byte(`{"schema_version":1,"artifact_type":"story_concept"}`)

	if err := coreStore.SaveCoreDesignArtifact(digest, original); err != nil {
		t.Fatalf("save original artifact: %v", err)
	}
	if err := coreStore.SaveCoreDesignArtifact(digest, original); err != nil {
		t.Fatalf("idempotent artifact save: %v", err)
	}
	if err := coreStore.SaveCoreDesignArtifact(digest, []byte(`{"schema_version":1,"artifact_type":"different"}`)); err == nil {
		t.Fatal("conflicting artifact bytes unexpectedly overwrote immutable object")
	}

	head := &domain.CoreDesignHead{
		SchemaVersion: 1,
		DesignRoot:    strings.Repeat("b", 64),
		Checkpoint:    domain.DesignCheckpointStoryLocked,
	}
	if err := coreStore.SaveCoreDesignHead(head); err != nil {
		t.Fatalf("save design head: %v", err)
	}
	got, err := coreStore.LoadCoreDesignHead()
	if err != nil {
		t.Fatalf("load design head: %v", err)
	}
	if got == nil {
		t.Fatal("design head missing after save")
	}
	if *got != *head {
		t.Fatalf("design head round-trip mismatch: got %+v want %+v", *got, *head)
	}
}
