package provider

import (
	"kandaoni.com/anqicms/model"
)

type PlaceTree struct {
	places  []*model.Place
	tree    []*model.Place
	treeKey map[uint]bool
	deep    int
	icons   []string
	tmp     map[uint][]*model.Place
}

func NewPlaceTree(places []*model.Place) *PlaceTree {
	ct := &PlaceTree{
		places:  places,
		tree:    []*model.Place{},
		treeKey: map[uint]bool{},
		deep:    1,
		icons:   []string{"└－", "", "", ""},
		tmp:     map[uint][]*model.Place{},
	}

	return ct
}

func (ct *PlaceTree) GetTree(rootId uint, add string) []*model.Place {
	return ct.getTreeRecursive(rootId, add, []model.ParentCategory{})
}

func (ct *PlaceTree) getTreeRecursive(rootId uint, add string, fullParents []model.ParentCategory) []*model.Place {
	isTop := 1
	children := ct.getChildren(rootId)
	space := ct.icons[3]
	if children != nil {
		cnt := len(children)
		for _, child := range children {
			if ct.deep > 1 {
				if isTop == 1 {
					space = ct.icons[1]
					add += ct.icons[0]
				}

				if isTop == cnt {
					space = ct.icons[2]
				} else {
					space = ct.icons[1]
				}
			}

			child.Spacer = add + space

			childFullParents := make([]model.ParentCategory, len(fullParents))
			copy(childFullParents, fullParents)
			childFullParents = append(childFullParents, model.ParentCategory{
				Id:    child.Id,
				Title: child.Title,
			})
			child.Parents = fullParents

			isTop++
			ct.deep++
			if !ct.treeKey[child.Id] {
				ct.treeKey[child.Id] = true
				ct.tree = append(ct.tree, child)
			}
			if ct.getChildren(child.Id) != nil {
				child.HasChildren = true
				ct.getTreeRecursive(child.Id, add, childFullParents)
				ct.deep--
			}
		}
	}

	var places []*model.Place
	for _, v := range ct.tree {
		places = append(places, v)
	}
	return places
}

func (ct *PlaceTree) GetTreeNode(rootId uint, add string) []*model.Place {
	return ct.getTreeNodeRecursive(rootId, add, []model.ParentCategory{})
}

func (ct *PlaceTree) getTreeNodeRecursive(rootId uint, add string, fullParents []model.ParentCategory) []*model.Place {
	var tree []*model.Place

	for _, place := range ct.places {
		if place.ParentId == rootId {
			place.Spacer = add

			childFullParents := make([]model.ParentCategory, len(fullParents))
			copy(childFullParents, fullParents)
			childFullParents = append(childFullParents, model.ParentCategory{
				Id:    place.Id,
				Title: place.Title,
			})
			place.Parents = fullParents

			space := add + ct.icons[0]
			place.Children = ct.getTreeNodeRecursive(place.Id, space, childFullParents)
			tree = append(tree, place)
		}
	}

	return tree
}

func (ct *PlaceTree) getChildren(rootId uint) []*model.Place {
	if len(ct.tmp) == 0 {
		for _, v := range ct.places {
			ct.tmp[v.ParentId] = append(ct.tmp[v.ParentId], v)
		}
	}

	if ct.tmp[rootId] != nil {
		return ct.tmp[rootId]
	}

	return nil
}
