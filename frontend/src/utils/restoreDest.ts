import type { TreeNode } from './restoreTree'

/** Resolve catalog path to likely on-disk destination for preview. */
export function resolveOriginalDest(catalogPath: string, sources: string[]): string {
  const norm = catalogPath.replace(/\//g, '\\')
  if (/^[a-zA-Z]:[\\/]/.test(norm)) return norm
  const root = (sources[0] || '').replace(/[\\/]+$/, '')
  if (!root) return norm
  return `${root}\\${norm.replace(/^\\+/, '')}`
}

/** All checked leaf files in the tree. */
export function collectCheckedFilePaths(tree: TreeNode[], checked: Set<string>): string[] {
  const files: string[] = []
  const walk = (nodes: TreeNode[]) => {
    for (const n of nodes) {
      if (!n.isDir && checked.has(n.path)) files.push(n.path)
      if (n.children.length) walk(n.children)
    }
  }
  walk(tree)
  return files.sort()
}
