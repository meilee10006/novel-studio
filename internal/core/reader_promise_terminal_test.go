package core

import "testing"

func TestReaderPromiseTerminalStatesCannotReopen(t *testing.T) {
	for _, tc := range []struct {
		name          string
		terminalState string
		reopen        map[string]any
	}{
		{"fulfilled to deferred", "fulfilled", map[string]any{"state": "deferred", "deadline_chapter": 10}},
		{"retired to advanced", "retired", map[string]any{"state": "advanced"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project, _, workspace, ready := acceptedFoundationProject(t)
			first := longformChapterArtifacts(1, []map[string]any{{
				"kind": "reader_promise", "local_id": "promise", "statement": "读者期待钥匙真相", "state": "advanced",
			}})
			settled := submitAndSettleChapter(t, project, workspace, ready, first)
			if settled.Result != "ACCEPTED" {
				t.Fatalf("create=%+v", settled)
			}
			promiseID := mappingID(settled.IDMappings, "reader_promise", "promise")
			if promiseID == "" {
				t.Fatalf("mapping=%+v", settled.IDMappings)
			}

			secondReady := readReady(t, workspace)
			terminal := map[string]any{"kind": "reader_promise", "promise_id": promiseID, "state": tc.terminalState}
			if tc.terminalState == "fulfilled" {
				terminal["event_ref"] = "e1"
			}
			if got := submitAndSettleChapter(t, project, workspace, secondReady, longformChapterArtifacts(2, []map[string]any{terminal})); got.Result != "ACCEPTED" {
				t.Fatalf("terminal=%+v", got)
			}

			thirdReady := readReady(t, workspace)
			reopen := map[string]any{"kind": "reader_promise", "promise_id": promiseID}
			for key, value := range tc.reopen {
				reopen[key] = value
			}
			got := submitAndSettleChapter(t, project, workspace, thirdReady, longformChapterArtifacts(3, []map[string]any{reopen}))
			if got.Result != "REWRITE" || !containsViolation(got.Violations, "terminal") {
				t.Fatalf("reopen=%+v", got)
			}
		})
	}
}
