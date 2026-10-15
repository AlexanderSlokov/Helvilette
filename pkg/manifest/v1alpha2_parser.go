package manifest

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// ParseRepoManifest reads, decodes and validates the v1alpha2 helvilette.yml at
// path. The os.ReadFile error is returned unwrapped so the agent can use
// os.IsNotExist to fall back to <fqdn>.yml, <hostname>.yml, local.yml
// (ADR-0009 D7).
//
//	m, err := manifest.ParseRepoManifest(filepath.Join(checkout, "helvilette.yml"))
func ParseRepoManifest(path string) (*RepoManifest, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m, err := DecodeRepoManifest(content)
	if err != nil {
		return nil, fmt.Errorf("invalid manifest %s: %w", path, err)
	}
	return m, nil
}

// DecodeRepoManifest turns helvilette.yml content into a validated
// RepoManifest. Unknown keys are errors: a misspelt `aply: auto` must not
// silently leave a playbook in plan mode.
//
//	m, err := manifest.DecodeRepoManifest([]byte(yamlText))
func DecodeRepoManifest(content []byte) (*RepoManifest, error) {
	if err := checkSchemaIdentity(content); err != nil {
		return nil, err
	}
	if err := checkRemovedFields(content); err != nil {
		return nil, err
	}
	m, err := decodeStrict(content)
	if err != nil {
		return nil, err
	}
	if err := ValidateRepoManifest(m); err != nil {
		return nil, err
	}
	return m, nil
}

// schemaIdentity is read leniently first, so a v1alpha1 file gets a migration
// hint rather than a list of unknown-field errors.
type schemaIdentity struct {
	APIVersion string `yaml:"apiVersion"`
}

func checkSchemaIdentity(content []byte) error {
	var id schemaIdentity
	if err := yaml.Unmarshal(content, &id); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}
	if id.APIVersion == SupportedAPIVersion {
		return fmt.Errorf("apiVersion %q is the Othela fleet schema; the agent reads %q. "+
			"Move spec.repo and spec.branch to agent.yaml and list playbooks under spec.playbooks. See ADR-0009",
			id.APIVersion, APIVersionV1Alpha2)
	}
	return nil
}

func decodeStrict(content []byte) (*RepoManifest, error) {
	dec := yaml.NewDecoder(bytes.NewReader(content))
	dec.KnownFields(true)
	var m RepoManifest
	if err := dec.Decode(&m); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("helvilette.yml is empty, expected apiVersion %q and a spec.playbooks list", APIVersionV1Alpha2)
		}
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}
	return &m, nil
}
