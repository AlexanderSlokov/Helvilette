package manifest

import "fmt"

// Schema identifiers this parser accepts.
//
// The group is Helvilette's own DNS subdomain rather than a Kubernetes in-tree
// group such as apps/v1, following the same convention k3s uses for its own
// kinds (k3s.cattle.io/v1, helm.cattle.io/v1). The v1alpha1 suffix records that
// spec.vault and nodeGroups[].probes are still declared-but-unenforced, so the
// schema may still change. See ADR-0002 and issue #1.
const (
	SupportedAPIVersion = "helvilette.io/v1alpha1"
	SupportedKind       = "PlaybookDeployment"
)

// Validate reports the first structural problem found in m, or nil if the
// manifest can be dispatched. Every field it requires is one that Othela reads
// when building a Job; a manifest that passes Validate cannot silently produce
// zero matches for reasons the operator cannot see in the file.
//
//	if err := manifest.Validate(m); err != nil {
//	    return fmt.Errorf("helvilette.yml rejected: %w", err)
//	}
func Validate(m *Manifest) error {
	if err := validateTypeMeta(m); err != nil {
		return err
	}
	if m.Metadata.Name == "" {
		return fmt.Errorf("metadata.name is empty, expected a non-empty identifier such as %q", "my-company-edge-proxy-fleet")
	}
	if err := validateSpec(&m.Spec); err != nil {
		return err
	}
	return validateNodeGroups(m.Spec.NodeGroups)
}

// validateTypeMeta rejects manifests written against a different schema. Before
// this check, a stale apiVersion/kind unmarshalled cleanly into a zero-value
// Manifest, so the playbook was dispatched to nobody without any diagnostic.
func validateTypeMeta(m *Manifest) error {
	if m.APIVersion != SupportedAPIVersion {
		return fmt.Errorf("unsupported apiVersion %q, expected %q", m.APIVersion, SupportedAPIVersion)
	}
	if m.Kind != SupportedKind {
		return fmt.Errorf("unsupported kind %q, expected %q", m.Kind, SupportedKind)
	}
	return nil
}

func validateSpec(spec *Spec) error {
	if spec.Repo == "" {
		return fmt.Errorf("spec.repo is empty, expected a Git URL such as %q", "https://git.example.com/org/playbooks.git")
	}
	if spec.Playbook == "" {
		return fmt.Errorf("spec.playbook is empty, expected a repo-relative path such as %q", "playbook.yml")
	}
	return nil
}

func validateNodeGroups(groups []NodeGroup) error {
	if len(groups) == 0 {
		return fmt.Errorf("spec.nodeGroups is empty, expected at least one group carrying a name and a nodeSelector")
	}
	for i, group := range groups {
		if err := validateNodeGroup(i, group); err != nil {
			return err
		}
	}
	return nil
}

// validateNodeGroup surfaces the empty-nodeSelector trap: MatchNodeGroups treats
// an empty selector as matching nothing, which reads as "no agent is eligible"
// rather than the "matches everything" an operator tends to assume.
func validateNodeGroup(index int, group NodeGroup) error {
	if group.Name == "" {
		return fmt.Errorf("spec.nodeGroups[%d].name is empty, expected a non-empty identifier such as %q", index, "standard-proxies")
	}
	if len(group.NodeSelector) == 0 {
		return fmt.Errorf("spec.nodeGroups[%d] (%q) has an empty nodeSelector and would match no node, expected at least one label such as %q", index, group.Name, "role: edge-proxy")
	}
	return nil
}
