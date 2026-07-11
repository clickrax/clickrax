import type { models } from '../../wailsjs/go/models'
import { i18n } from '../i18n'

export interface TreeNode {
  path: string
  name: string
  isDir: boolean
  size: number
  modified: string
  owner: string
  attributes: string
  children: TreeNode[]
}

export type SortKey = 'name' | 'size' | 'modified' | 'owner'
export type SortDir = 'asc' | 'desc'

export function buildTree(files: models.SnapshotFile[]): TreeNode[] {
  const root: TreeNode[] = []
  const map = new Map<string, TreeNode>()

  const sorted = [...files].sort((a, b) => a.path.localeCompare(b.path, 'ru'))
  for (const f of sorted) {
    const parts = f.path.split('\\').filter(Boolean)
    let parentPath = ''
    for (let i = 0; i < parts.length; i++) {
      const name = parts[i]
      const path = parentPath ? `${parentPath}\\${name}` : name
      const isLast = i === parts.length - 1

      let node = map.get(path)
      if (!node) {
        node = {
          path,
          name,
          isDir: !isLast || f.is_dir,
          size: 0,
          modified: '',
          owner: '',
          attributes: '',
          children: [],
        }
        map.set(path, node)
        if (parentPath) {
          map.get(parentPath)!.children.push(node)
        } else {
          root.push(node)
        }
      }

      if (isLast) {
        if (f.is_dir) {
          node.isDir = true
        } else {
          node.isDir = false
          node.size = f.size
          node.modified = f.modified || ''
          node.owner = f.owner || ''
          node.attributes = f.attributes || ''
        }
      }
      parentPath = path
    }
  }

  for (const n of root) enrichNode(n)
  return root
}

function enrichNode(n: TreeNode): { size: number; modified: string } {
  if (!n.isDir || n.children.length === 0) {
    return { size: n.size, modified: n.modified }
  }
  let size = 0
  let modified = ''
  for (const c of n.children) {
    const stats = enrichNode(c)
    size += stats.size
    if (stats.modified > modified) modified = stats.modified
  }
  n.size = size
  if (!n.modified) n.modified = modified
  return { size, modified }
}

export function findNode(nodes: TreeNode[], path: string): TreeNode | null {
  for (const n of nodes) {
    if (n.path === path) return n
    const found = findNode(n.children, path)
    if (found) return found
  }
  return null
}

export function breadcrumbs(path: string): { name: string; path: string }[] {
  const rootName = i18n.global.t('restore_ext.snapshot_root')
  if (!path) return [{ name: rootName, path: '' }]
  const parts = path.split('\\').filter(Boolean)
  const crumbs: { name: string; path: string }[] = [{ name: rootName, path: '' }]
  let acc = ''
  for (const p of parts) {
    acc = acc ? `${acc}\\${p}` : p
    crumbs.push({ name: p, path: acc })
  }
  return crumbs
}

export function sortItems(items: TreeNode[], key: SortKey, dir: SortDir): TreeNode[] {
  const mul = dir === 'asc' ? 1 : -1
  return [...items].sort((a, b) => {
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1
    switch (key) {
      case 'size':
        return (a.size - b.size) * mul
      case 'modified': {
        const am = a.modified || ''
        const bm = b.modified || ''
        if (am === bm) return a.name.localeCompare(b.name, 'ru', { sensitivity: 'base' }) * mul
        return am.localeCompare(bm) * mul
      }
      case 'owner': {
        const ao = a.owner || ''
        const bo = b.owner || ''
        if (ao === bo) return a.name.localeCompare(b.name, 'ru', { sensitivity: 'base' }) * mul
        return ao.localeCompare(bo, 'ru', { sensitivity: 'base' }) * mul
      }
      default:
        return a.name.localeCompare(b.name, 'ru', { sensitivity: 'base' }) * mul
    }
  })
}

export function buildSearchItems(files: models.SnapshotFile[], query: string): TreeNode[] {
  const q = query.trim().toLowerCase()
  if (!q) return []
  return files
    .filter((f) => f.path.toLowerCase().includes(q))
    .map((f) => {
      const parts = f.path.split('\\').filter(Boolean)
      return {
        path: f.path,
        name: parts[parts.length - 1] || f.path,
        isDir: f.is_dir,
        size: f.size,
        modified: f.modified || '',
        owner: f.owner || '',
        attributes: f.attributes || '',
        children: [],
      }
    })
}

export function filterTree(nodes: TreeNode[], query: string): TreeNode[] {
  const q = query.trim().toLowerCase()
  if (!q) return nodes

  const out: TreeNode[] = []
  for (const n of nodes) {
    const kids = filterTree(n.children, q)
    const selfMatch = n.name.toLowerCase().includes(q) || n.path.toLowerCase().includes(q)
    if (selfMatch || kids.length) {
      out.push({ ...n, children: kids.length ? kids : n.children })
    }
  }
  return out
}

export function collectPaths(nodes: TreeNode[]): string[] {
  const paths: string[] = []
  const walk = (list: TreeNode[]) => {
    for (const n of list) {
      paths.push(n.path)
      if (n.children.length) walk(n.children)
    }
  }
  walk(nodes)
  return paths
}

export function countFiles(node: TreeNode): number {
  if (!node.isDir) return 1
  return node.children.reduce((sum, c) => sum + countFiles(c), 0)
}

export function checkState(node: TreeNode, checked: Set<string>): 'checked' | 'unchecked' | 'indeterminate' {
  if (!node.isDir) {
    return checked.has(node.path) ? 'checked' : 'unchecked'
  }
  if (node.children.length === 0) {
    return checked.has(node.path) ? 'checked' : 'unchecked'
  }
  let checkedCount = 0
  let indeterminate = false
  for (const c of node.children) {
    const st = checkState(c, checked)
    if (st === 'indeterminate') indeterminate = true
    else if (st === 'checked') checkedCount++
  }
  if (indeterminate || (checkedCount > 0 && checkedCount < node.children.length)) {
    return 'indeterminate'
  }
  if (checkedCount === node.children.length) return 'checked'
  return checked.has(node.path) ? 'checked' : 'unchecked'
}

export function setChecked(node: TreeNode, checked: Set<string>, value: boolean) {
  if (value) checked.add(node.path)
  else checked.delete(node.path)
  for (const c of node.children) {
    setChecked(c, checked, value)
  }
}

export function compactSelection(tree: TreeNode[], checked: Set<string>): string[] {
  const out: string[] = []
  const walk = (nodes: TreeNode[]) => {
    for (const n of nodes) {
      const st = checkState(n, checked)
      if (st === 'checked') {
        out.push(n.path)
      } else if (st === 'indeterminate') {
        walk(n.children)
      }
    }
  }
  walk(tree)
  return out
}

export function countCheckedFiles(tree: TreeNode[], checked: Set<string>): number {
  let total = 0
  const walk = (nodes: TreeNode[]) => {
    for (const n of nodes) {
      if (!n.isDir) {
        if (checked.has(n.path)) total++
      } else {
        walk(n.children)
      }
    }
  }
  walk(tree)
  return total
}

export function parentPath(path: string): string {
  const idx = path.lastIndexOf('\\')
  if (idx <= 0) return ''
  return path.slice(0, idx)
}
