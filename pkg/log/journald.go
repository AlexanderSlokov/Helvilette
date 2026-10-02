// Sending logs to journald as native fields rather than as a JSON blob inside
// MESSAGE, so `journalctl NODE_ID=node-1` works and a collector needs neither a
// path nor a parse stage. See ADR-0007 R1 and D2.
package log

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/coreos/go-systemd/v22/journal"
	"github.com/rs/zerolog"
)

// identifierPrefix makes every component selectable in one go:
// `journalctl -t 'helvilette*'` finds the control plane and every agent without
// knowing either unit name.
const identifierPrefix = "helvilette-"

// sendToJournal is the one call this package makes into journald. Named so a
// test can substitute a recorder; journal.Send needs a running journald, which a
// unit test must not.
type sendToJournal func(message string, priority journal.Priority, fields map[string]string) error

// journalSink turns the JSON zerolog hands it into one journald entry.
//
// It is an io.Writer so it can sit where os.Stdout otherwise would, which keeps
// the rest of this package unaware of which destination is in use.
type journalSink struct {
	identifier string
	send       sendToJournal
	// fallback receives the original line when a send fails. journald can be
	// restarted underneath a long-running agent, and an entry that cannot be
	// delivered is worth more on stderr than dropped.
	fallback io.Writer
}

// newJournalSink builds the sink used when the process runs as a systemd unit.
func newJournalSink(identifier string, fallback io.Writer) *journalSink {
	return &journalSink{
		identifier: identifier,
		send:       journal.Send,
		fallback:   fallback,
	}
}

// Write converts one zerolog event into a journald entry.
//
// The returned count is always len(p) on success: zerolog treats a short write as
// an error, and the number of bytes journald accepted bears no relation to the
// number zerolog produced.
func (s *journalSink) Write(p []byte) (int, error) {
	event, err := decodeEvent(p)
	if err != nil {
		// Not a zerolog event. Pass it through rather than swallow it; something
		// is writing to this logger that this package did not produce.
		return s.fallback.Write(p)
	}

	message, fields := s.splitEvent(event)

	if err := s.send(message, priorityOf(event), fields); err != nil {
		fmt.Fprintf(s.fallback, "journald send failed (%v), original entry: %s", err, p)
		return len(p), nil
	}

	return len(p), nil
}

func decodeEvent(p []byte) (map[string]any, error) {
	decoder := json.NewDecoder(strings.NewReader(string(p)))
	// Without this, every number arrives as a float64 and an integer field is
	// rendered as "2e+00" in the journal.
	decoder.UseNumber()

	var event map[string]any
	if err := decoder.Decode(&event); err != nil {
		return nil, err
	}
	return event, nil
}

// splitEvent separates the human-readable message from the fields that make the
// entry queryable.
//
// level and time are dropped: journald records PRIORITY and its own timestamp,
// and duplicating them would make `journalctl -o json` contradict itself.
func (s *journalSink) splitEvent(event map[string]any) (string, map[string]string) {
	fields := map[string]string{"SYSLOG_IDENTIFIER": s.identifier}
	message := ""

	for key, value := range event {
		switch key {
		case zerolog.LevelFieldName, zerolog.TimestampFieldName:
			continue
		case zerolog.MessageFieldName:
			message, _ = value.(string)
			continue
		}

		name, ok := fieldName(key)
		if !ok {
			continue
		}
		fields[name] = fieldValue(value)
	}

	return message, fields
}

// fieldName converts a zerolog key into a journald field name, reporting whether
// the result is usable.
//
// An unusable name is skipped rather than sent. go-systemd's appendVariable
// prints "contains invalid character, ignoring" to stderr and then writes the
// name anyway, which both corrupts the entry and floods stderr once per log line.
func fieldName(key string) (string, bool) {
	name := strings.ToUpper(key)

	if name == "" || name[0] == '_' {
		return "", false
	}
	for _, c := range name {
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
			return "", false
		}
	}

	return name, true
}

// fieldValue renders a value as the byte string journald stores. Everything that
// is not already text is marshalled back to JSON, so a nested object such as the
// RawJSON task_logs on a report survives intact as one field.
func fieldValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case nil:
		return ""
	}

	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprintf("[unencodable: %v]", err)
	}
	return string(encoded)
}

// priorityOf maps a zerolog level onto a syslog priority. journald has more
// levels than zerolog, so several map onto one.
func priorityOf(event map[string]any) journal.Priority {
	name, _ := event[zerolog.LevelFieldName].(string)

	switch level, _ := zerolog.ParseLevel(name); level {
	case zerolog.TraceLevel, zerolog.DebugLevel:
		return journal.PriDebug
	case zerolog.InfoLevel:
		return journal.PriInfo
	case zerolog.WarnLevel:
		return journal.PriWarning
	case zerolog.ErrorLevel:
		return journal.PriErr
	case zerolog.FatalLevel:
		return journal.PriCrit
	case zerolog.PanicLevel:
		return journal.PriEmerg
	}

	return journal.PriInfo
}

// IdentifierFor returns the SYSLOG_IDENTIFIER for a binary at the given path.
//
// Native journal sends do not inherit the unit's SyslogIdentifier= — that setting
// applies to the stream transport only — so the binary has to supply its own, and
// the two must agree or `journalctl -t` stops finding anything. Deriving it from
// the binary name means neither has to be configured.
//
// Exported and pure so the drift guard beside each unit file can call it without
// depending on the name of the test binary it happens to run inside.
//
//	IdentifierFor("/usr/local/bin/othela") == "helvilette-othela"
func IdentifierFor(path string) string {
	return identifierPrefix + filepath.Base(path)
}

// underJournal reports whether systemd has connected this process's stderr to the
// journal, which it does only for its own units.
//
// This is the signal systemd documents for choosing the native protocol
// ("automatic protocol upgrading", systemd.io/JOURNAL_NATIVE_PROTOCOL). The
// presence of /run/systemd/journal/socket is not: that is also true when an
// operator runs the binary by hand on a systemd machine, where upgrading would
// swallow their terminal output. See ADR-0007 D2.
func underJournal() bool {
	// The error cases are a missing or malformed JOURNAL_STREAM and a failed
	// fstat, none of which mean "running as a unit".
	isStream, err := journal.StderrIsJournalStream()
	return err == nil && isStream
}
