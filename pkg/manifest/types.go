package manifest

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
}

type AnsibleConfig struct {
	VaultPasswordFile string            `yaml:"vault-password-file"`
	ExtraVars         map[string]string `yaml:"extra_vars"`
}
