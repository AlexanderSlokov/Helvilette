package e2e_test

import (
	"strings"
	"testing"
)

// One Ansible JSON callback document, trimmed to the fields the parser reads.
// Captured from a run where the baseline fixture named a package that does not
// exist. The agent saves this as runs/<playbook>.json in its workspace.
const failedRunOutput = `{"plays":[{"tasks":[
{"task":{"name":"baseline-node : 1. Install chrony"},"hosts":{"localhost":{"failed":true,"msg":"No package matching 'chrony-nope' is available"}}},
{"task":{"name":"baseline-node : 2. Template chrony.conf"},"hosts":{"localhost":{"changed":true}}}
]}]}`

const succeededRunOutput = `{"plays":[{"tasks":[{"task":{"name":"nginx : install"},"hosts":{"localhost":{"changed":true}}}]}]}`

func TestFailedAnsibleTasks_NamesNodeTaskAndMessage(t *testing.T) {
	got := strings.Join(failedAnsibleTasks("node-1", failedRunOutput), "\n")

	for _, want := range []string{"node-1/localhost", "Install chrony", "No package matching 'chrony-nope' is available"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not mention %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Template chrony.conf") {
		t.Errorf("a task that did not fail was reported:\n%s", got)
	}
}

// The runs directory holds one file per playbook; `cat runs/*.json` hands the
// parser several documents back to back.
func TestFailedAnsibleTasks_ReadsConcatenatedDocuments(t *testing.T) {
	got := failedAnsibleTasks("node-1", succeededRunOutput+"\n"+failedRunOutput)

	if len(got) != 1 || !strings.Contains(got[0], "Install chrony") {
		t.Errorf("expected exactly the one failure from the second document, got %v", got)
	}
}

// Output that is not Ansible JSON must not panic the diagnostic, which runs
// precisely when something already went wrong.
func TestFailedAnsibleTasks_SurvivesUnparseableOutput(t *testing.T) {
	if got := failedAnsibleTasks("node-1", "not json at all"); len(got) != 0 {
		t.Errorf("expected no failures from unparseable output, got %v", got)
	}
}
