package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func selectionNames(selected []PlaybookSelection) []string {
	names := make([]string, 0, len(selected))
	for _, s := range selected {
		names = append(names, s.Entry.Name+"/"+s.Group.Name)
	}
	return names
}

// Every matching playbook, in file order.
func TestSelectPlaybooks_KeepsFileOrder(t *testing.T) {
	m, err := DecodeRepoManifest([]byte(exampleManifest))
	require.NoError(t, err)

	got := SelectPlaybooks(m, map[string]string{"managed": "true", "role": "edge-proxy"})

	assert.Equal(t, []string{"baseline/all-nodes", "web/standard-proxies"}, selectionNames(got))
}

func TestSelectPlaybooks_SkipsPlaybooksThatDoNotMatch(t *testing.T) {
	m, err := DecodeRepoManifest([]byte(exampleManifest))
	require.NoError(t, err)

	got := SelectPlaybooks(m, map[string]string{"managed": "true"})

	assert.Equal(t, []string{"baseline/all-nodes"}, selectionNames(got))
}

// No match is an empty result, not an error.
func TestSelectPlaybooks_ReturnsNothingForUnmatchedNode(t *testing.T) {
	m, err := DecodeRepoManifest([]byte(exampleManifest))
	require.NoError(t, err)

	assert.Empty(t, SelectPlaybooks(m, map[string]string{"role": "db"}))
}

// Within one playbook, the first matching group wins.
func TestSelectPlaybooks_FirstMatchingGroupWins(t *testing.T) {
	m := validRepoManifest()
	m.Spec.Playbooks[0].NodeGroups = []NodeGroup{
		{Name: "proxies", NodeSelector: map[string]string{"role": "edge-proxy"}},
		{Name: "hot-proxies", NodeSelector: map[string]string{"role": "edge-proxy", "tier": "hot"}},
	}

	got := SelectPlaybooks(m, map[string]string{"role": "edge-proxy", "tier": "hot"})

	assert.Equal(t, []string{"baseline/proxies"}, selectionNames(got))
}
