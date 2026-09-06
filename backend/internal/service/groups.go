// JNexus 运维平台 — By JJ Zhang, Version 1.0
package service

import (
	"autoops/internal/model"
)

// GroupAndDescendants 返回指定分组及其全部后代分组的 ID（含自身）
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

// GroupAncestors 返回分组及其全部祖先 ID（含自身），用于规则匹配主机所在分组
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
