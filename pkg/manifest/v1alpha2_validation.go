package manifest

import (
	"fmt"
	"path/filepath"
)

// ValidateRepoManifest reports the first structural problem in m, naming the
// field and the expected shape, or nil if the agent can act on it.
//
//	if err := manifest.ValidateRepoManifest(m); err != nil { ... }
func ValidateRepoManifest(m *RepoManifest) error {
	if m.APIVersion != APIVersionV1Alpha2 {
		return fmt.Errorf("unsupported apiVersion %q, expected %q", m.APIVersion, APIVersionV1Alpha2)
	}
	if m.Kind != SupportedKind {
		return fmt.Errorf("unsupported kind %q, expected %q", m.Kind, SupportedKind)
	}
	if m.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is empty, expected a non-empty identifier such as %q", "edge-fleet")
	}
	return validatePlaybookEntries(m.Spec.Playbooks)
}

func validatePlaybookEntries(entries []PlaybookEntry) error {
	if len(entries) == 0 {
		return fmt.Errorf("spec.playbooks is empty, expected at least one entry with a name, a playbook and nodeGroups")
	}
	firstIndex := make(map[string]int, len(entries))
	for i, entry := range entries {
		field := fmt.Sprintf("spec.playbooks[%d]", i)
		if err := validatePlaybookEntry(field, entry); err != nil {
			return err
		}
		if j, taken := firstIndex[entry.Name]; taken {
			return fmt.Errorf("%s.name %q is already used by spec.playbooks[%d]; names must be unique because run records and explain refer to them", field, entry.Name, j)
		}
		firstIndex[entry.Name] = i
	}
	return nil
}

func validatePlaybookEntry(field string, entry PlaybookEntry) error {
	if entry.Name == "" {
		return fmt.Errorf("%s.name is empty, expected a non-empty identifier such as %q", field, "baseline")
	}
	if err := validatePlaybookPath(field, entry.Playbook); err != nil {
		return err
	}
	if err := validateApplyMode(field, entry.Apply); err != nil {
		return err
	}
	if err := validateNodeGroups(field+".nodeGroups", entry.NodeGroups); err != nil {
		return err
	}
	return rejectReservedProbes(field+".nodeGroups", entry.NodeGroups)
}

// validatePlaybookPath keeps the playbook inside the checkout. The repository
// is the only thing a signed commit vouches for; a path out of it would run a
// file nobody signed.
func validatePlaybookPath(field, path string) error {
	if path == "" {
		return fmt.Errorf("%s.playbook is empty, expected a repo-relative path such as %q", field, "baseline.yml")
	}
	if !filepath.IsLocal(path) {
		return fmt.Errorf("%s.playbook %q leaves the repository, expected a relative path without '..' such as %q", field, path, "playbooks/web.yml")
	}
	return nil
}

func validateApplyMode(field string, mode ApplyMode) error {
	switch mode {
	case "", ApplyAuto, ApplyPlan:
		return nil
	}
	return fmt.Errorf("%s.apply is %q, expected %q or %q; leave it out to plan only", field, mode, ApplyAuto, ApplyPlan)
}

// rejectReservedProbes refuses probes by name until they have a schema, so a
// probe the operator wrote is never silently ignored.
func rejectReservedProbes(field string, groups []NodeGroup) error {
	for i, group := range groups {
		if group.Probes.Kind != 0 {
			return fmt.Errorf("%s[%d] (%q) declares probes, which are not supported yet; remove the probes key for now", field, i, group.Name)
		}
	}
	return nil
}
