package core

import (
	"encoding/json"
	"testing"
)

func TestReaderPromiseStatementIsStableAcrossTransitions(t *testing.T) {
	project, _, workspace, ready := acceptedFoundationProject(t)
	first := longformChapterArtifacts(1, []map[string]any{{
		"kind": "reader_promise", "local_id": "umbrella-promise",
		"statement": "读者期待知道红色纸伞真正主人是谁", "state": "advanced",
	}})
	settled := submitAndSettleChapter(t, project, workspace, ready, first)
	if settled.Result != "ACCEPTED" {
		t.Fatalf("create=%+v", settled)
	}
	promiseID := mappingID(settled.IDMappings, "reader_promise", "umbrella-promise")
	if promiseID == "" {
		t.Fatalf("mapping missing: %+v", settled.IDMappings)
	}

	badReady := readReady(t, workspace)
	bad := longformChapterArtifacts(2, []map[string]any{{
		"kind": "reader_promise", "promise_id": promiseID,
		"statement": "读者改为期待知道凶手是谁", "state": "deferred", "deadline_chapter": 5,
	}})
	got := submitAndSettleChapter(t, project, workspace, badReady, bad)
	if got.Result != "REWRITE" || !containsViolation(got.Violations, "statement") {
		t.Fatalf("changed statement=%+v", got)
	}

	retry := readReady(t, workspace)
	good := longformChapterArtifacts(2, []map[string]any{{
		"kind": "reader_promise", "promise_id": promiseID,
		"state": "deferred", "deadline_chapter": 5,
	}})
	got = submitAndSettleChapter(t, project, workspace, retry, good)
	if got.Result != "ACCEPTED" {
		t.Fatalf("omitted statement=%+v", got)
	}
	raw, err := project.store.ReadCoreCanonStateBytes()
	if err != nil {
		t.Fatal(err)
	}
	var canon struct {
		Longform struct {
			ReaderPromises map[string]struct {
				Statement string `json:"statement"`
			} `json:"reader_promises"`
		} `json:"longform"`
	}
	if err := json.Unmarshal(raw, &canon); err != nil {
		t.Fatal(err)
	}
	if got := canon.Longform.ReaderPromises[promiseID].Statement; got != "读者期待知道红色纸伞真正主人是谁" {
		t.Fatalf("statement=%q", got)
	}
}
