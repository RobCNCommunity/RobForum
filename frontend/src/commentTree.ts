import type { Comment } from '@/api'

export interface CommentNode {
  item: Comment
  children: CommentNode[]
}

export function buildCommentTree(comments: Comment[]): CommentNode[] {
  const nodes = new Map<number, CommentNode>()
  for (const item of comments) nodes.set(item.id, { item, children: [] })

  const roots: CommentNode[] = []
  for (const item of comments) {
    const node = nodes.get(item.id)
    if (!node) continue
    const parent = item.parent_id ? nodes.get(item.parent_id) : undefined
    if (parent && parent.item.id !== item.id) parent.children.push(node)
    else roots.push(node)
  }
  return roots
}
