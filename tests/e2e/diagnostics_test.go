package e2e_test

import (
	"strings"
	"testing"
)

// A report line as Othela actually logs it, trimmed to the fields the parser
// reads. Captured from a run where the baseline fixture named a package that
// does not exist.
const failedReportLine = `{"level":"info","component":"othela","node_id":"node-2","job_id":"job-x","status":"Failed","task_logs":{"plays":[{"tasks":[` +
	`{"task":{"name":"baseline-node : 1. Install chrony"},"hosts":{"localhost":{"failed":true,"msg":"No package matching 'chrony-nope' is available"}}},` +
	`{"task":{"name":"baseline-node : 2. Template chrony.conf"},"hosts":{"localhost":{"changed":true}}}` +
	`]}]},"message":"report received"}`

const succeededReportLine = `{"level":"info","component":"othela","node_id":"node-1","status":"Success","task_logs":{"plays":[]},"message":"report received"}`

// TestFailedAnsibleTasks_CollapsesRepeats is the reason this parser exists: an
// agent retries every poll interval, so one broken task produced dozens of
// identical entries and buried anything else. A plain CI log said only
// "exit status 2".
func TestFailedAnsibleTasks_CollapsesRepeats(t *testing.T) {
	journal := strings.Join([]string{failedReportLine, failedReportLine, failedReportLine}, "\n")

	got := failedAnsibleTasks(journal)

	if strings.Count(got, "Install chrony") != 1 {
		t.Errorf("the same failure was reported more than once:\n%s", got)
	}
	if !strings.Contains(got, "(seen 3 times)") {
		t.Errorf("repeat count missing:\n%s", got)
	}
	for _, want := range []string{"node-2/localhost", "No package matching 'chrony-nope' is available"} {
		if !strings.Contains(got, want) {
			t.Errorf("output does not mention %q:\n%s", want, got)
		}
	}
}

func TestFailedAnsibleTasks_IgnoresPassingTasksAndReports(t *testing.T) {
	journal := strings.Join([]string{succeededReportLine, failedReportLine}, "\n")

	got := failedAnsibleTasks(journal)

	if strings.Contains(got, "Template chrony.conf") {
		t.Errorf("a task that did not fail was reported:\n%s", got)
	}
	if strings.Contains(got, "node-1") {
		t.Errorf("a successful report was reported:\n%s", got)
	}
}

// Journals carry lines that are not reports at all, and systemd prefixes its own.
// A diagnostic that panics on them is worse than none.
func TestFailedAnsibleTasks_SurvivesNonReportLines(t *testing.T) {
	journal := strings.Join([]string{
		"",
		"not json at all",
		`{"level":"debug","component":"othela","message":"node polling for work"}`,
		failedReportLine,
	}, "\n")

	got := failedAnsibleTasks(journal)

	if !strings.Contains(got, "Install chrony") {
		t.Errorf("the real failure was lost among unparseable lines:\n%s", got)
	}
}
