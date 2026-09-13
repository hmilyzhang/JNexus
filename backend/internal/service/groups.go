// JNexus Ops Platform — By JJ Zhang, Version 1.0
package service

import (
	"jnexus/internal/model"
)

// GroupAndDescendants returns the IDs of the given group and all its descendant groups (including itself)
func GroupAndDescendants(rootID uint) []uint {
	var groups []model.HostGroup
	model.DB.Find(&groups)
	children := map[uint][]uint{}
	for _, g := range groups {
		if g.ParentID != nil {
			children[*g.ParentID] = append(children[*g.ParentID], g.ID)
		}
	}
	out := []uint{rootID}
	queue := []uint{rootID}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, c := range children[cur] {
			out = append(out, c)
			queue = append(queue, c)
		}
	}
	return out
}

// GroupAncestors returns the IDs of the group and all its ancestors (including itself), used to match rules against the host's group
func GroupAncestors(groupID uint) []uint {
	var groups []model.HostGroup
	model.DB.Find(&groups)
	parent := map[uint]*uint{}
	for _, g := range groups {
		parent[g.ID] = g.ParentID
	}
	out := []uint{groupID}
	cur := groupID
	for {
		p := parent[cur]
		if p == nil {
			break
		}
		out = append(out, *p)
		cur = *p
	}
	return out
}
