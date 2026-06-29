package manifest

import (
	"os"

	"gopkg.in/yaml.v3"
)

// ParseFile reads and unmarshals a helvilette.yml file
func ParseFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	return &m, nil
}

// MatchNodeGroups returns all nodeGroups whose nodeSelector is a subset of the given labels
func MatchNodeGroups(manifest *Manifest, labels map[string]string) []NodeGroup {
	var matches []NodeGroup

	for _, group := range manifest.Spec.NodeGroups {
		if isSubset(group.NodeSelector, labels) {
			matches = append(matches, group)
		}
	}

	return matches
}

func isSubset(selector, labels map[string]string) bool {
	if len(selector) == 0 {
		return false
	}
	
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}
