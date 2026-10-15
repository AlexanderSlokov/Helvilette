package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validRepoManifest() *RepoManifest {
	return &RepoManifest{
		APIVersion: APIVersionV1Alpha2,
		Kind:       SupportedKind,
		Metadata:   RepoMetadata{Name: "edge-fleet"},
		Spec: RepoSpec{Playbooks: []PlaybookEntry{
			{
				Name:       "baseline",
				Playbook:   "baseline.yml",
				NodeGroups: []NodeGroup{{Name: "all", NodeSelector: map[string]string{"managed": "true"}}},
			},
		}},
	}
}

func TestValidateRepoManifest_AcceptsMinimalManifest(t *testing.T) {
	assert.NoError(t, ValidateRepoManifest(validRepoManifest()))
}

func TestValidateRepoManifest_RejectsWithFieldAndShape(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RepoManifest)
		want   string
	}{
		{"wrong apiVersion", func(m *RepoManifest) { m.APIVersion = "v9" }, `unsupported apiVersion "v9"`},
		{"wrong kind", func(m *RepoManifest) { m.Kind = "Cluster" }, `unsupported kind "Cluster"`},
		{"no name", func(m *RepoManifest) { m.Metadata.Name = "" }, "metadata.name is empty"},
		{"no playbooks", func(m *RepoManifest) { m.Spec.Playbooks = nil }, "spec.playbooks is empty"},
		{"playbook without name", func(m *RepoManifest) { m.Spec.Playbooks[0].Name = "" }, "spec.playbooks[0].name is empty"},
		{"playbook without path", func(m *RepoManifest) { m.Spec.Playbooks[0].Playbook = "" }, "spec.playbooks[0].playbook is empty"},
		{"absolute path", func(m *RepoManifest) { m.Spec.Playbooks[0].Playbook = "/etc/site.yml" }, "leaves the repository"},
		{"escaping path", func(m *RepoManifest) { m.Spec.Playbooks[0].Playbook = "../other/site.yml" }, "leaves the repository"},
		{"unknown apply", func(m *RepoManifest) { m.Spec.Playbooks[0].Apply = "yes" }, `spec.playbooks[0].apply is "yes", expected "auto" or "plan"`},
		{"no nodeGroups", func(m *RepoManifest) { m.Spec.Playbooks[0].NodeGroups = nil }, "spec.playbooks[0].nodeGroups is empty"},
		{"empty selector", func(m *RepoManifest) { m.Spec.Playbooks[0].NodeGroups[0].NodeSelector = nil }, `spec.playbooks[0].nodeGroups[0] ("all") has an empty nodeSelector`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validRepoManifest()
			tt.mutate(m)

			err := ValidateRepoManifest(m)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.want)
		})
	}
}

func TestValidateRepoManifest_AcceptsNestedRelativePath(t *testing.T) {
	m := validRepoManifest()
	m.Spec.Playbooks[0].Playbook = "playbooks/web.yml"

	assert.NoError(t, ValidateRepoManifest(m))
}

func TestValidateRepoManifest_RejectsDuplicatePlaybookNames(t *testing.T) {
	m := validRepoManifest()
	m.Spec.Playbooks = append(m.Spec.Playbooks, m.Spec.Playbooks[0])

	err := ValidateRepoManifest(m)

	require.Error(t, err)
	assert.Contains(t, err.Error(), `spec.playbooks[1].name "baseline" is already used by spec.playbooks[0]`)
}

// ADR-0004 applies within each playbook, with the v1alpha2 field path.
func TestValidateRepoManifest_RejectsIdenticalSelectorsWithinPlaybook(t *testing.T) {
	m := validRepoManifest()
	group := m.Spec.Playbooks[0].NodeGroups[0]
	group.Name = "all-again"
	m.Spec.Playbooks[0].NodeGroups = append(m.Spec.Playbooks[0].NodeGroups, group)

	err := ValidateRepoManifest(m)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.playbooks[0].nodeGroups[0]")
	assert.Contains(t, err.Error(), "identical nodeSelector")
}

// The same selector in two different playbooks is the normal case: a node
// runs both, in order (ADR-0009 D6).
func TestValidateRepoManifest_AllowsSameSelectorAcrossPlaybooks(t *testing.T) {
	m := validRepoManifest()
	second := m.Spec.Playbooks[0]
	second.Name, second.Playbook = "web", "web.yml"
	m.Spec.Playbooks = append(m.Spec.Playbooks, second)

	assert.NoError(t, ValidateRepoManifest(m))
}
