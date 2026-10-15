package manifest

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// v1alpha1Leftovers captures the fields v1alpha2 removed (ADR-0009 D8). Strict
// decoding would reject them anyway, but only as "field repo not found"; this
// says what replaced each one. A yaml.Node field captures the value whatever
// its type, and a non-zero Kind means the key was present. (yaml.v3 only
// captures into yaml.Node by value; a *yaml.Node field fails on scalars.)
type v1alpha1Leftovers struct {
	Metadata struct {
		Namespace yaml.Node `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		Repo       yaml.Node `yaml:"repo"`
		Branch     yaml.Node `yaml:"branch"`
		Playbook   yaml.Node `yaml:"playbook"`
		Vault      yaml.Node `yaml:"vault"`
		NodeGroups yaml.Node `yaml:"nodeGroups"`
	} `yaml:"spec"`
}

// removedField pairs a v1alpha1 field with what an operator should do instead.
type removedField struct {
	node        yaml.Node
	field       string
	replacement string
}

func (l v1alpha1Leftovers) removedFields() []removedField {
	return []removedField{
		{l.Metadata.Namespace, "metadata.namespace", "delete it; it never had a function"},
		{l.Spec.Repo, "spec.repo", "set repoURL in agent.yaml on each node"},
		{l.Spec.Branch, "spec.branch", "set branch in agent.yaml on each node"},
		{l.Spec.Playbook, "spec.playbook", "use spec.playbooks[].playbook"},
		{l.Spec.Vault, "spec.vault", "use Ansible's Vault lookup, or nodeGroups[].ansible.vault-password-file as a path on the node"},
		{l.Spec.NodeGroups, "spec.nodeGroups", "move each group under the spec.playbooks[] entry it belongs to"},
	}
}

// checkRemovedFields rejects the first removed v1alpha1 field it finds, naming
// its replacement.
func checkRemovedFields(content []byte) error {
	var leftovers v1alpha1Leftovers
	if err := yaml.Unmarshal(content, &leftovers); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}
	for _, f := range leftovers.removedFields() {
		if f.node.Kind != 0 {
			return fmt.Errorf("%s was removed in %s: %s. See ADR-0009 D8", f.field, APIVersionV1Alpha2, f.replacement)
		}
	}
	return nil
}
