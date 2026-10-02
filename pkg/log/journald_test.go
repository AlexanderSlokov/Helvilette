package log

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/rs/zerolog"
)

// journalEntry is one thing the sink tried to send.
type journalEntry struct {
	message  string
	priority journal.Priority
	fields   map[string]string
}

// recordingJournal stands in for journald. A real send needs a running journald,
// which a unit test must not require, and the whole point of this package is the
// shape of what gets sent — so the shape is what the fake records.
type recordingJournal struct {
	entries []journalEntry
	err     error // when set, every send fails with it
}

func (r *recordingJournal) send(message string, priority journal.Priority, fields map[string]string) error {
	r.entries = append(r.entries, journalEntry{message: message, priority: priority, fields: fields})
	return r.err
}

func (r *recordingJournal) only(t *testing.T) journalEntry {
	t.Helper()

	if len(r.entries) != 1 {
		t.Fatalf("sent %d entries, want exactly 1: %+v", len(r.entries), r.entries)
	}
	return r.entries[0]
}

// newTestSink wires a sink to a fake journald and a capturable fallback.
func newTestSink() (*journalSink, *recordingJournal, *bytes.Buffer) {
	recorder := &recordingJournal{}
	fallback := &bytes.Buffer{}

	return &journalSink{
		identifier: "helvilette-test",
		send:       recorder.send,
		fallback:   fallback,
	}, recorder, fallback
}

// writeEvent pushes one zerolog event through the sink under test.
func writeEvent(t *testing.T, sink *journalSink, emit func(zerolog.Logger)) {
	t.Helper()

	emit(zerolog.New(sink).With().Timestamp().Logger())
}

// TestJournalSink_SplitsMessageFromFields is the point of ADR-0007 D2: before
// this, the whole JSON line landed in MESSAGE and nothing was addressable, so
// `journalctl NODE_ID=node-1` could not work.
func TestJournalSink_SplitsMessageFromFields(t *testing.T) {
	sink, recorder, _ := newTestSink()

	writeEvent(t, sink, func(l zerolog.Logger) {
		l.Info().Str("node_id", "node-1").Str("component", "agent").Msg("registered with Othela")
	})

	entry := recorder.only(t)
	if entry.message != "registered with Othela" {
		t.Errorf("MESSAGE = %q, want the message alone", entry.message)
	}
	for name, want := range map[string]string{
		"NODE_ID":           "node-1",
		"COMPONENT":         "agent",
		"SYSLOG_IDENTIFIER": "helvilette-test",
	} {
		if got := entry.fields[name]; got != want {
			t.Errorf("field %s = %q, want %q", name, got, want)
		}
	}
}

// Both are journald's own: PRIORITY carries the level and journald timestamps the
// entry on receipt. Sending ours too would let `journalctl -o json` contradict
// itself.
func TestJournalSink_DropsLevelAndTime(t *testing.T) {
	sink, recorder, _ := newTestSink()

	writeEvent(t, sink, func(l zerolog.Logger) { l.Warn().Msg("careful") })

	for _, name := range []string{"LEVEL", "TIME"} {
		if value, present := recorder.only(t).fields[name]; present {
			t.Errorf("field %s was sent with value %q; journald supplies it", name, value)
		}
	}
}

func TestJournalSink_MapsEveryLevelToAPriority(t *testing.T) {
	cases := map[string]struct {
		emit func(zerolog.Logger)
		want journal.Priority
	}{
		"debug": {func(l zerolog.Logger) { l.Debug().Msg("x") }, journal.PriDebug},
		"info":  {func(l zerolog.Logger) { l.Info().Msg("x") }, journal.PriInfo},
		"warn":  {func(l zerolog.Logger) { l.Warn().Msg("x") }, journal.PriWarning},
		"error": {func(l zerolog.Logger) { l.Error().Msg("x") }, journal.PriErr},
		"trace": {func(l zerolog.Logger) { l.Trace().Msg("x") }, journal.PriDebug},
	}

	previous := zerolog.GlobalLevel()
	zerolog.SetGlobalLevel(zerolog.TraceLevel)
	defer zerolog.SetGlobalLevel(previous)

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			sink, recorder, _ := newTestSink()
			writeEvent(t, sink, tc.emit)

			if got := recorder.only(t).priority; got != tc.want {
				t.Errorf("priority = %d, want %d", got, tc.want)
			}
		})
	}
}

// Numbers must not arrive as floats. Without json.Number a count of 2 renders as
// "2e+00" in the journal, which is both wrong and unqueryable.
func TestJournalSink_RendersNumbersAsWritten(t *testing.T) {
	sink, recorder, _ := newTestSink()

	writeEvent(t, sink, func(l zerolog.Logger) {
		l.Info().Int("count", 2).Int("poll_interval", 5000).Msg("scan complete")
	})

	entry := recorder.only(t)
	for name, want := range map[string]string{"COUNT": "2", "POLL_INTERVAL": "5000"} {
		if got := entry.fields[name]; got != want {
			t.Errorf("field %s = %q, want %q", name, got, want)
		}
	}
}

// A report's task_logs is RawJSON. It has to survive as one field, because that
// payload is what explains a failed playbook.
func TestJournalSink_KeepsNestedJSONIntact(t *testing.T) {
	sink, recorder, _ := newTestSink()

	writeEvent(t, sink, func(l zerolog.Logger) {
		l.Info().RawJSON("task_logs", []byte(`{"plays":[{"name":"baseline"}]}`)).Msg("report received")
	})

	got := recorder.only(t).fields["TASK_LOGS"]
	for _, want := range []string{`"plays"`, `"baseline"`} {
		if !strings.Contains(got, want) {
			t.Errorf("TASK_LOGS = %q, does not contain %s", got, want)
		}
	}
}

// go-systemd prints "contains invalid character, ignoring" to stderr and then
// writes the name anyway, which corrupts the entry and floods stderr once per log
// line. So an unusable name has to be dropped here instead.
func TestFieldName(t *testing.T) {
	valid := map[string]string{
		"node_id":       "NODE_ID",
		"component":     "COMPONENT",
		"configSources": "CONFIGSOURCES",
		"count":         "COUNT",
	}
	for key, want := range valid {
		if got, ok := fieldName(key); !ok || got != want {
			t.Errorf("fieldName(%q) = %q, %v; want %q, true", key, got, ok, want)
		}
	}

	for _, key := range []string{"", "_leading", "with-dash", "with.dot", "with space", "unicodé"} {
		if got, ok := fieldName(key); ok {
			t.Errorf("fieldName(%q) = %q, true; journald would reject that name", key, got)
		}
	}
}

func TestJournalSink_SkipsUnusableFieldNamesButKeepsTheRest(t *testing.T) {
	sink, recorder, _ := newTestSink()

	writeEvent(t, sink, func(l zerolog.Logger) {
		l.Info().Str("with-dash", "dropped").Str("node_id", "kept").Msg("mixed")
	})

	entry := recorder.only(t)
	if _, present := entry.fields["WITH-DASH"]; present {
		t.Error("an unusable field name reached journald")
	}
	if entry.fields["NODE_ID"] != "kept" {
		t.Errorf("NODE_ID = %q, want kept: one bad name must not lose the entry", entry.fields["NODE_ID"])
	}
}

// journald can be restarted underneath a long-running agent. An entry that cannot
// be delivered is worth more on stderr than dropped.
func TestJournalSink_FallsBackWhenTheSendFails(t *testing.T) {
	sink, recorder, fallback := newTestSink()
	recorder.err = errors.New("socket gone")

	writeEvent(t, sink, func(l zerolog.Logger) { l.Error().Str("node_id", "node-9").Msg("undeliverable") })

	for _, want := range []string{"socket gone", "undeliverable", "node-9"} {
		if !strings.Contains(fallback.String(), want) {
			t.Errorf("fallback does not mention %q:\n%s", want, fallback.String())
		}
	}
}

// Something other than zerolog writing here must not vanish silently.
func TestJournalSink_PassesNonJSONToTheFallback(t *testing.T) {
	sink, recorder, fallback := newTestSink()

	if _, err := sink.Write([]byte("plain text from somewhere else\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if len(recorder.entries) != 0 {
		t.Errorf("sent %d entries for unparseable input, want 0", len(recorder.entries))
	}
	if !strings.Contains(fallback.String(), "plain text from somewhere else") {
		t.Errorf("fallback did not receive the line:\n%s", fallback.String())
	}
}

// Native sends do not inherit the unit's SyslogIdentifier=, so the binary derives
// the same value the unit declares. The guards beside each unit file assert the
// two agree; this asserts the derivation itself.
func TestIdentifierFor(t *testing.T) {
	cases := map[string]string{
		"/usr/local/bin/othela":           "helvilette-othela",
		"/usr/local/bin/agent":            "helvilette-agent",
		"othela":                          "helvilette-othela",
		"/tmp/go-build123/b001/exe/agent": "helvilette-agent",
	}

	for path, want := range cases {
		if got := IdentifierFor(path); got != want {
			t.Errorf("IdentifierFor(%q) = %q, want %q", path, got, want)
		}
	}
}
