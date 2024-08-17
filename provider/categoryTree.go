package provider

import (
	"kandaoni.com/anqicms/model"
)

type CategoryTree struct {
	categories []*model.Category
	tree       []*model.Category
	treeKey    map[uint]bool
	deep       int
	icons      []string
	tmp        map[uint][]*model.Category
}

func NewCategoryTree(categories []*model.Category) *CategoryTree {
	ct := &CategoryTree{
		categories: categories,
		tree:       []*model.Category{},
		treeKey:    map[uint]bool{},
		deep:       1,
		icons:      []string{"└&nbsp;&nbsp;", "", "", ""},
		tmp:        map[uint][]*model.Category{},
	}

	return ct
}

func (ct *CategoryTree) GetTree(rootId uint, add string) []*model.Category {
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

			isTop++
			ct.deep++
			if !ct.treeKey[child.Id] {
				ct.treeKey[child.Id] = true
				ct.tree = append(ct.tree, child)
			}
			if ct.getChildren(child.Id) != nil {
				child.HasChildren = true
				ct.GetTree(child.Id, add)
				ct.deep--
			}
		}
	}

	var categories []*model.Category
	for _, v := range ct.tree {
		categories = append(categories, v)
	}
	return categories
}

func (ct *CategoryTree) GetTreeNode(rootId uint, add string) []*model.Category {
	var tree []*model.Category

	// 遍历分类列表
	for _, category := range ct.categories {
		// 找到当前节点的子节点
		if category.ParentId == rootId {
			category.Spacer = add
			// 递归构建子节点的子树
			space := add + ct.icons[0]
			category.Children = ct.GetTreeNode(category.Id, space)
			// 将当前节点加入到父节点的Children中
			tree = append(tree, category)
		}
	}

	return tree
}

func (ct *CategoryTree) getChildren(rootId uint) []*model.Category {
	if len(ct.tmp) == 0 {
		for _, v := range ct.categories {
			ct.tmp[v.ParentId] = append(ct.tmp[v.ParentId], v)
		}
	}

	if ct.tmp[rootId] != nil {
		return ct.tmp[rootId]
	}

	return nil
}
