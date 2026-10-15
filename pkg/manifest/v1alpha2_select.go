package manifest

// PlaybookSelection is a playbook chosen for one node, with the nodeGroup whose
// selector matched and whose variables apply.
type PlaybookSelection struct {
	Entry PlaybookEntry
	Group NodeGroup
}

// SelectPlaybooks returns, in file order, every playbook that targets a node
// with these labels, each with its first matching nodeGroup.
// An empty result means the node runs nothing, which is a normal state, not an
// error.
//
//	for _, s := range manifest.SelectPlaybooks(m, map[string]string{"role": "edge-proxy"}) { ... }
func SelectPlaybooks(m *RepoManifest, labels map[string]string) []PlaybookSelection {
	var selected []PlaybookSelection
	for _, entry := range m.Spec.Playbooks {
		if group, ok := firstMatchingGroup(entry.NodeGroups, labels); ok {
			selected = append(selected, PlaybookSelection{Entry: entry, Group: group})
		}
	}
	return selected
}

// firstMatchingGroup keeps v1alpha1's rule: file order wins when several
// groups match. Overlapping selectors such as {role: proxy} and
// {role: proxy, tier: hot} are allowed; the one written first wins.
func firstMatchingGroup(groups []NodeGroup, labels map[string]string) (NodeGroup, bool) {
	for _, group := range groups {
		if isSubset(group.NodeSelector, labels) {
			return group, true
		}
	}
	return NodeGroup{}, false
}
