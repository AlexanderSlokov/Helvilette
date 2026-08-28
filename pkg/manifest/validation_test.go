package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validManifest() *Manifest {
	return &Manifest{
		APIVersion: SupportedAPIVersion,
		Kind:       SupportedKind,
		Metadata:   Metadata{Name: "edge-proxy-fleet"},
		Spec: Spec{
			Repo:     "https://git.example.com/org/playbooks.git",
			Branch:   "main",
			Playbook: "playbook.yml",
			NodeGroups: []NodeGroup{
				{
					Name:         "standard-proxies",
					NodeSelector: map[string]string{"role": "edge-proxy"},
				},
			},
		},
	}
}

func TestValidateAcceptsCompleteManifest(t *testing.T) {
	assert.NoError(t, Validate(validManifest()))
}

func TestValidateRejectsMissingFields(t *testing.T) {
	tests := []struct {
		name        string
		mutate      func(*Manifest)
		wantMessage string
	}{
		{
			name:        "empty apiVersion",
			mutate:      func(m *Manifest) { m.APIVersion = "" },
			wantMessage: `unsupported apiVersion "", expected "helvilette.io/v1alpha1"`,
		},
		{
			name:        "empty kind",
			mutate:      func(m *Manifest) { m.Kind = "" },
			wantMessage: `unsupported kind "", expected "PlaybookDeployment"`,
		},
		{
			name:        "missing metadata.name",
			mutate:      func(m *Manifest) { m.Metadata.Name = "" },
			wantMessage: "metadata.name is empty",
		},
		{
			name:        "missing spec.repo",
			mutate:      func(m *Manifest) { m.Spec.Repo = "" },
			wantMessage: "spec.repo is empty",
		},
		{
			name:        "missing spec.playbook",
			mutate:      func(m *Manifest) { m.Spec.Playbook = "" },
			wantMessage: "spec.playbook is empty",
		},
		{
			name:        "no node groups",
			mutate:      func(m *Manifest) { m.Spec.NodeGroups = nil },
			wantMessage: "spec.nodeGroups is empty",
		},
		{
			name:        "node group without a name",
			mutate:      func(m *Manifest) { m.Spec.NodeGroups[0].Name = "" },
			wantMessage: "spec.nodeGroups[0].name is empty",
		},
		{
			name:        "node group without a selector",
			mutate:      func(m *Manifest) { m.Spec.NodeGroups[0].NodeSelector = nil },
			wantMessage: `spec.nodeGroups[0] ("standard-proxies") has an empty nodeSelector`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validManifest()
			tt.mutate(m)

			err := Validate(m)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantMessage)
		})
	}
}

// spec.branch stays optional: Othela passes it through as the Job version and an
// empty value means "whatever the clone's default branch is".
func TestValidateAllowsEmptyBranch(t *testing.T) {
	m := validManifest()
	m.Spec.Branch = ""

	assert.NoError(t, Validate(m))
}

// Every rejection has to name the field, so an operator can fix the file without
// reading the parser. Guards the CLAUDE.md rule on exception messages.
func TestValidateErrorsNameTheExpectedShape(t *testing.T) {
	m := validManifest()
	m.Spec.NodeGroups[0].NodeSelector = map[string]string{}

	err := Validate(m)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "role: edge-proxy", "error should show the expected shape")
}
