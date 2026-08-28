package manifest

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ParseFile reads, unmarshals and validates a helvilette.yml file.
//
// The os.ReadFile error is returned unwrapped so callers can keep using
// os.IsNotExist to tell "no manifest here" apart from "broken manifest".
//
//	m, err := manifest.ParseFile("/srv/playbooks/nginx/helvilette.yml")
func ParseFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if err := Validate(&m); err != nil {
		return nil, fmt.Errorf("invalid manifest %s: %w", path, err)
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
