package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The example from ADR-0009 D3, so the record and the parser cannot disagree.
const adr0009Example = `
apiVersion: helvilette.naughtian.org/v1alpha2
kind: PlaybookDeployment
metadata:
  name: edge-fleet
  description: Baseline on every node, nginx on edge proxies
spec:
  playbooks:
    - name: baseline
      playbook: baseline.yml
      apply: auto
      nodeGroups:
        - name: all-nodes
          nodeSelector:
            managed: "true"
          ansible:
            extra_vars:
              chrony_pool: ntp.ubuntu.com

    - name: web
      playbook: web.yml
      nodeGroups:
        - name: standard-proxies
          nodeSelector:
            role: edge-proxy
          ansible:
            extra_vars:
              nginx_http_port: "80"
            vault-password-file: /etc/helvilette/vault-pass
`

func TestDecodeRepoManifest_AcceptsADRExample(t *testing.T) {
	m, err := DecodeRepoManifest([]byte(adr0009Example))
	require.NoError(t, err)

	require.Len(t, m.Spec.Playbooks, 2)
	assert.Equal(t, "edge-fleet", m.Metadata.Name)
	assert.Equal(t, ApplyAuto, m.Spec.Playbooks[0].EffectiveApply())
	assert.Equal(t, ApplyPlan, m.Spec.Playbooks[1].EffectiveApply(), "no apply key must mean plan")
	assert.Equal(t, "/etc/helvilette/vault-pass", m.Spec.Playbooks[1].NodeGroups[0].Ansible.VaultPasswordFile)
}

func TestParseRepoManifest_ReturnsNotExistUnwrapped(t *testing.T) {
	_, err := ParseRepoManifest(filepath.Join(t.TempDir(), "helvilette.yml"))

	assert.True(t, os.IsNotExist(err), "the agent falls back to <hostname>.yml on os.IsNotExist; got %v", err)
}

func TestParseRepoManifest_NamesTheFileOnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "helvilette.yml")
	require.NoError(t, os.WriteFile(path, []byte("apiVersion: nope\n"), 0644))

	_, err := ParseRepoManifest(path)

	require.Error(t, err)
	assert.Contains(t, err.Error(), path)
}

func TestDecodeRepoManifest_PointsV1Alpha1AtTheMigration(t *testing.T) {
	_, err := DecodeRepoManifest([]byte("apiVersion: helvilette.naughtian.org/v1alpha1\nkind: PlaybookDeployment\n"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Othela fleet schema")
	assert.Contains(t, err.Error(), "ADR-0009")
}

func TestDecodeRepoManifest_NamesReplacementForRemovedFields(t *testing.T) {
	tests := map[string]struct{ snippet, want string }{
		"spec.repo":          {"spec:\n  repo: git://x/y\n", "agent.yaml"},
		"spec.branch":        {"spec:\n  branch: main\n", "agent.yaml"},
		"spec.playbook":      {"spec:\n  playbook: site.yml\n", "spec.playbooks[].playbook"},
		"spec.vault":         {"spec:\n  vault: {}\n", "Vault lookup"},
		"spec.nodeGroups":    {"spec:\n  nodeGroups: []\n", "under the spec.playbooks[] entry"},
		"metadata.namespace": {"metadata:\n  namespace: default\n", "never had a function"},
	}
	for field, tt := range tests {
		t.Run(field, func(t *testing.T) {
			_, err := DecodeRepoManifest([]byte("apiVersion: helvilette.naughtian.org/v1alpha2\n" + tt.snippet))

			require.Error(t, err)
			assert.Contains(t, err.Error(), field+" was removed")
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

// A misspelt key must fail loudly: `aply: auto` silently read as plan would
// look like a Helvilette bug to the operator.
func TestDecodeRepoManifest_RejectsUnknownKeys(t *testing.T) {
	_, err := DecodeRepoManifest([]byte(adr0009Example + "      aply: auto\n"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "aply")
}

func TestDecodeRepoManifest_RejectsEmptyFile(t *testing.T) {
	_, err := DecodeRepoManifest(nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.playbooks")
}

func TestDecodeRepoManifest_ReportsBrokenYAML(t *testing.T) {
	_, err := DecodeRepoManifest([]byte("apiVersion: [unclosed\n"))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing YAML")
}

func TestDecodeRepoManifest_RejectsProbesByName(t *testing.T) {
	withProbes := adr0009Example + "          probes:\n            nginx.service: {}\n"

	_, err := DecodeRepoManifest([]byte(withProbes))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.playbooks[1].nodeGroups[0]")
	assert.Contains(t, err.Error(), "probes, which are not supported yet")
}
