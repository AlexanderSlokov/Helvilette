package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFile(t *testing.T) {
	content := `
apiVersion: v1
kind: Helvilette
metadata:
  name: test-app
spec:
  repo: https://github.com/example/repo
  branch: main
  playbook: site.yml
  nodeGroups:
    - name: web
      nodeSelector:
        role: webserver
      ansible:
        extra_vars:
          env: prod
`
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "helvilette.yml")
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	manifest, err := ParseFile(filePath)
	require.NoError(t, err)

	assert.Equal(t, "v1", manifest.APIVersion)
	assert.Equal(t, "test-app", manifest.Metadata.Name)
	assert.Equal(t, "https://github.com/example/repo", manifest.Spec.Repo)
	assert.Equal(t, "site.yml", manifest.Spec.Playbook)
	assert.Len(t, manifest.Spec.NodeGroups, 1)
	assert.Equal(t, "webserver", manifest.Spec.NodeGroups[0].NodeSelector["role"])
	assert.Equal(t, "prod", manifest.Spec.NodeGroups[0].Ansible.ExtraVars["env"])
}

func TestMatchNodeGroups(t *testing.T) {
	manifest := &Manifest{
		Spec: Spec{
			NodeGroups: []NodeGroup{
				{
					Name: "exact-match",
					NodeSelector: map[string]string{
						"role": "web",
						"env":  "prod",
					},
				},
				{
					Name: "partial-match", // Should match if agent has more labels
					NodeSelector: map[string]string{
						"role": "web",
					},
				},
				{
					Name: "no-match",
					NodeSelector: map[string]string{
						"role": "db",
					},
				},
				{
					Name: "empty-selector",
					NodeSelector: map[string]string{},
				},
			},
		},
	}

	tests := []struct {
		name           string
		agentLabels    map[string]string
		expectedGroups []string
	}{
		{
			name: "exact match",
			agentLabels: map[string]string{
				"role": "web",
				"env":  "prod",
			},
			expectedGroups: []string{"exact-match", "partial-match"},
		},
		{
			name: "superset labels",
			agentLabels: map[string]string{
				"role":   "web",
				"env":    "prod",
				"region": "us-east-1",
			},
			expectedGroups: []string{"exact-match", "partial-match"},
		},
		{
			name: "no match",
			agentLabels: map[string]string{
				"role": "cache",
			},
			expectedGroups: []string{},
		},
		{
			name: "partial labels (doesn't meet exact match)",
			agentLabels: map[string]string{
				"role": "web",
				"env":  "dev", // different env
			},
			expectedGroups: []string{"partial-match"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := MatchNodeGroups(manifest, tt.agentLabels)
			
			var matchedNames []string
			for _, m := range matches {
				matchedNames = append(matchedNames, m.Name)
			}
			
			assert.ElementsMatch(t, tt.expectedGroups, matchedNames)
		})
	}
}
