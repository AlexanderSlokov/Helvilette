// Manifest v1alpha2: the file the standalone agent reads from the root of its
// own checkout. One file lists every playbook in the repository, in order, and
// says which of them may change machines on their own.
//
// v1alpha1 (types.go) stays for Othela, which is frozen and still reads it.
package manifest

// APIVersionV1Alpha2 is the schema identity the agent accepts. The file keeps
// the Kubernetes shape (apiVersion, kind, metadata, spec) on purpose: anyone
// who has read a Kubernetes manifest can read this one.
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

// RepoSpec lists playbooks in the order the agent runs them.
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
	// ApplyAuto lets the agent change the machine.
	ApplyAuto ApplyMode = "auto"
	// ApplyPlan only checks and records; nothing on the machine changes.
	ApplyPlan ApplyMode = "plan"
)

// EffectiveApply returns the playbook's mode. No apply key means plan: a
// playbook changes machines only when the file says so explicitly.
//
//	if entry.EffectiveApply() == manifest.ApplyAuto { ... }
func (p PlaybookEntry) EffectiveApply() ApplyMode {
	if p.Apply == "" {
		return ApplyPlan
	}
	return p.Apply
}
