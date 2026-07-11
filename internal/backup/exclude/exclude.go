package exclude

import (
	"path"
	"path/filepath"
	"strings"

	"pbs-win-backup/internal/config"
)

type Engine struct {
	root  string
	rules []parsedRule
}

type parsedRule struct {
	basenameOnly bool
	hasGlob      bool
	pattern      string
}

// Merge combines global and per-job exclusion rules without duplicates.
func Merge(global, job []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(global)+len(job))
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	for _, s := range global {
		add(s)
	}
	for _, s := range job {
		add(s)
	}
	return out
}

func New(rules []string) *Engine {
	return NewForRoot("", rules)
}

func NewForRoot(root string, rules []string) *Engine {
	e := &Engine{root: normRoot(root)}
	for _, r := range rules {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		e.rules = append(e.rules, parsedRule{
			basenameOnly: !strings.ContainsAny(r, `/\`),
			hasGlob:      strings.ContainsAny(r, "*?["),
			pattern:      normRulePattern(e.root, r),
		})
	}
	return e
}

func normRoot(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = filepath.Clean(s)
	if len(s) == 2 && s[1] == ':' {
		return strings.ToUpper(s[:1]) + `:\`
	}
	return strings.ToLower(s)
}

func normSlashes(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, `\`, `/`))
}

func normRulePattern(root, rule string) string {
	rule = strings.TrimSpace(rule)
	if strings.ContainsAny(rule, `/\`) || (len(rule) >= 2 && rule[1] == ':') {
		abs := normRoot(rule)
		if root != "" {
			if rel := relUnderRoot(root, abs); rel != "" || abs == root {
				if rel == "" {
					return ""
				}
				return normSlashes(rel)
			}
		}
		return normSlashes(abs)
	}
	return strings.ToLower(rule)
}

func relUnderRoot(root, full string) string {
	root = normRoot(root)
	full = normRoot(full)
	if root == "" {
		return normSlashes(strings.TrimPrefix(full, root))
	}
	if !strings.HasPrefix(full, root) {
		return ""
	}
	rel := strings.TrimPrefix(full, root)
	rel = strings.TrimPrefix(rel, `\`)
	return normSlashes(rel)
}

func relPath(root, fullPath string) string {
	full := normRoot(fullPath)
	if root == "" {
		return normSlashes(filepath.ToSlash(fullPath))
	}
	if rel := relUnderRoot(root, full); rel != "" {
		return rel
	}
	return normSlashes(filepath.ToSlash(fullPath))
}

// IsSystemName reports built-in Windows paths that should never be backed up.
func IsSystemName(name string, isDir bool) bool {
	_ = isDir
	if name == "" {
		return false
	}
	for _, s := range config.DefaultExclusions() {
		if strings.ContainsAny(s, `/\`) {
			continue
		}
		if strings.EqualFold(name, s) {
			return true
		}
	}
	return false
}

func (e *Engine) MatchPath(fullPath, name string, isDir bool) bool {
	if IsSystemName(name, isDir) {
		return true
	}
	lowerName := strings.ToLower(name)
	rel := relPath(e.root, fullPath)

	for _, r := range e.rules {
		if r.basenameOnly {
			if r.hasGlob {
				if ok, _ := filepath.Match(r.pattern, lowerName); ok {
					return true
				}
			} else if lowerName == r.pattern {
				return true
			}
			continue
		}
		if r.pattern == "" {
			continue
		}
		if r.hasGlob {
			if ok, _ := path.Match(r.pattern, rel); ok {
				return true
			}
			continue
		}
		if rel == r.pattern || strings.HasPrefix(rel, r.pattern+"/") {
			return true
		}
	}
	return false
}
