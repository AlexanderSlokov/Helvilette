package manifest

import "gopkg.in/yaml.v3"

type Manifest struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       Spec     `yaml:"spec"`
}

type Metadata struct {
	Name        string            `yaml:"name"`
	Namespace   string            `yaml:"namespace"`
	Labels      map[string]string `yaml:"labels"`
	Description string            `yaml:"description"`
}

type Spec struct {
	Repo       string      `yaml:"repo"`
	Branch     string      `yaml:"branch"`
	Playbook   string      `yaml:"playbook"`
	NodeGroups []NodeGroup `yaml:"nodeGroups"`
}

type NodeGroup struct {
	Name         string            `yaml:"name"`
	NodeSelector map[string]string `yaml:"nodeSelector"`
	Ansible      AnsibleConfig     `yaml:"ansible"`

	// Probes is captured only so v1alpha2 can reject it by name: the key is
	// reserved until probes have a schema (ADR-0009 D10). v1alpha1 ignores it,
	// as it always has.
	Probes yaml.Node `yaml:"probes,omitempty" json:"-"`
}

type AnsibleConfig struct {
	VaultPasswordFile string            `yaml:"vault-password-file"`
	ExtraVars         map[string]string `yaml:"extra_vars"`
}
