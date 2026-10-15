// Manifest v1alpha2: the file the standalone agent reads from the root of its
// own checkout. One file lists every playbook in the repository, in order, and
// says which of them may change machines on their own. See ADR-0009.
//
// v1alpha1 (types.go) stays for Othela, which is frozen and still reads it.
package manifest

// APIVersionV1Alpha2 is the schema identity the agent accepts. The envelope
// keeps Kubernetes' shape on purpose: the doctrine rejects copying Kubernetes'
// topology, not its file format (ADR-0009 D2).
const APIVersionV1Alpha2 = "helvilette.naughtian.org/v1alpha2"

// RepoManifest is a parsed v1alpha2 helvilette.yml.
type RepoManifest struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   RepoMetadata `yaml:"metadata"`
	Spec       RepoSpec     `yaml:"spec"`
}

// RepoMetadata drops v1alpha1's namespace, which never had a function.
type RepoMetadata struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Labels      map[string]string `yaml:"labels"`
}

// RepoSpec lists playbooks in the order the agent runs them (ADR-0009 D6).
type RepoSpec struct {
	Playbooks []PlaybookEntry `yaml:"playbooks"`
}

// PlaybookEntry is one playbook in the repository and the nodes it targets.
type PlaybookEntry struct {
	Name       string      `yaml:"name"`
	Playbook   string      `yaml:"playbook"`
	Apply      ApplyMode   `yaml:"apply"`
	NodeGroups []NodeGroup `yaml:"nodeGroups"`
}

// ApplyMode says whether a playbook may change a machine on its own.
type ApplyMode string

const (
	// ApplyAuto applies after a passing check run.
	ApplyAuto ApplyMode = "auto"
	// ApplyPlan only checks and records; nothing on the machine changes.
	ApplyPlan ApplyMode = "plan"
)

// EffectiveApply returns the playbook's mode, reading an absent key as plan:
// changing machines must be asked for in a signed commit (ADR-0009 D4).
//
//	if entry.EffectiveApply() == manifest.ApplyAuto { ... }
func (p PlaybookEntry) EffectiveApply() ApplyMode {
	if p.Apply == "" {
		return ApplyPlan
	}
	return p.Apply
}
