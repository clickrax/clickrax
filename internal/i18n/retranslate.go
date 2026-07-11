package i18n

import (
	"regexp"
	"sort"
	"strings"
)

// Keys handled by dedicated helpers must not participate in generic template matching.
var retranslateSkipKeys = map[string]bool{
	"quick.name_prefix":  true,
	"quick.default_name": true,
}

// statusKeys are translated by exact match when re-reading stored records.
var statusKeys = []string{
	"backup.interrupted",
	"backup.interrupted_incomplete",
	"backup.cancelled",
	"backup.stopping",
	"backup.last_cancelled",
	"backup.last_error",
	"backup.no_successful",
	"quick.default_name",
	"dest.smb_ok",
	"dest.ftp_ok",
}

var (
	onlinePBSDatastoresRE = regexp.MustCompile(`^(?:Онлайн|Online)\. PBS (.+), datastores?: (.+)$`)
	onlinePBSConfirmedRE  = regexp.MustCompile(`^(?:Онлайн|Online)\. PBS (.+) — (?:доступ к datastore |access to datastore )(.+?)(?: подтверждён| confirmed)$`)
	onlinePBSNsRE         = regexp.MustCompile(`^, namespace (.+)$`)
	templateVarRE         = regexp.MustCompile(`\{\{(\w+)\}\}`)
)

// RetranslateStored maps a message saved in any supported language to the active bundle.
func (b *Bundle) RetranslateStored(msg string) string {
	if msg == "" || b == nil {
		return msg
	}
	msg = strings.TrimSpace(msg)
	suffixKey := ""
	if base, key, ok := stripConnectionTestSuffix(msg); ok {
		msg = base
		suffixKey = key
	}
	for _, key := range statusKeys {
		if msg == ru[key] || msg == en[key] {
			out := b.T(key)
			if suffixKey != "" {
				out += b.T(suffixKey)
			}
			return out
		}
	}
	// Fixed-prefix {{err}} templates (e.g. pbs.index_load_prev_err) before generic
	// patterns like pbs.didx.load ("загрузка {{name}}: {{err}}").
	for key, ruTpl := range ru {
		if !strings.Contains(ruTpl, "{{") {
			continue
		}
		enTpl := en[key]
		for _, tpl := range []string{ruTpl, enTpl} {
			if tpl == "" || !strings.Contains(tpl, "{{err}}") {
				continue
			}
			beforeErr := strings.Split(tpl, "{{err}}")[0]
			if strings.Contains(beforeErr, "{{") {
				continue
			}
			prefix := strings.TrimSuffix(beforeErr, ": ")
			if prefix == "" {
				continue
			}
			sep := prefix + ": "
			if rest, ok := strings.CutPrefix(msg, sep); ok && rest != "" {
				out := b.Tf(key, map[string]string{"err": rest})
				if suffixKey != "" {
					out += b.T(suffixKey)
				}
				return out
			}
		}
	}
	if out, ok := b.retranslateByTemplate(msg); ok {
		if suffixKey != "" {
			out += b.T(suffixKey)
		}
		return out
	}
	if out, ok := b.retranslateConnectionTemplate(msg); ok {
		if suffixKey != "" {
			return out + b.T(suffixKey)
		}
		return out
	}
	if suffixKey != "" {
		return msg + b.T(suffixKey)
	}
	return msg
}

func stripConnectionTestSuffix(msg string) (base, key string, ok bool) {
	for _, suffixKey := range []string{"test.protocol_not_checked"} {
		for _, suffix := range []string{ru[suffixKey], en[suffixKey]} {
			if suffix != "" && strings.HasSuffix(msg, suffix) {
				return strings.TrimSuffix(msg, suffix), suffixKey, true
			}
		}
	}
	return msg, "", false
}

func (b *Bundle) retranslateConnectionTemplate(msg string) (string, bool) {
	if m := onlinePBSDatastoresRE.FindStringSubmatch(msg); len(m) == 3 {
		return b.Tf("dest.online_pbs_datastores", map[string]string{
			"version": strings.TrimSpace(m[1]),
			"n":       strings.TrimSpace(m[2]),
		}), true
	}
	if m := onlinePBSConfirmedRE.FindStringSubmatch(msg); len(m) == 3 {
		datastorePart := strings.TrimSpace(m[2])
		vars := map[string]string{
			"version":   strings.TrimSpace(m[1]),
			"datastore": datastorePart,
			"ns":        "",
		}
		if idx := strings.LastIndex(datastorePart, ", namespace "); idx >= 0 {
			vars["datastore"] = strings.TrimSpace(datastorePart[:idx])
			if ns := onlinePBSNsRE.FindStringSubmatch(strings.TrimSpace(datastorePart[idx:])); len(ns) == 2 {
				vars["ns"] = b.Tf("dest.online_pbs_ns", map[string]string{"ns": ns[1]})
			}
		}
		return b.Tf("dest.online_pbs", vars), true
	}
	for _, key := range []string{"dest.pbs_datastore_failed"} {
		for _, tpl := range []string{ru[key], en[key]} {
			if tpl == "" || !strings.Contains(tpl, "{{version}}") || !strings.Contains(tpl, "{{err}}") {
				continue
			}
			prefix := strings.Split(tpl, "{{version}}")[0]
			suffix := strings.Split(tpl, "{{err}}")[1]
			if !strings.HasPrefix(msg, prefix) || !strings.HasSuffix(msg, suffix) {
				continue
			}
			rest := strings.TrimSuffix(strings.TrimPrefix(msg, prefix), suffix)
			parts := strings.SplitN(rest, ": ", 2)
			if len(parts) != 2 {
				continue
			}
			return b.Tf(key, map[string]string{
				"version": strings.TrimSpace(parts[0]),
				"err":     strings.TrimSpace(parts[1]),
			}), true
		}
	}
	return "", false
}

func matchI18nTemplate(tpl, msg string) (map[string]string, bool) {
	tpl = strings.TrimSpace(tpl)
	msg = strings.TrimSpace(msg)
	if tpl == "" || msg == "" {
		return nil, false
	}
	if !strings.Contains(tpl, "{{") {
		if msg == tpl {
			return map[string]string{}, true
		}
		return nil, false
	}
	parts := templateVarRE.Split(tpl, -1)
	vars := templateVarRE.FindAllStringSubmatch(tpl, -1)
	if len(vars) == 0 {
		return nil, false
	}
	var b strings.Builder
	b.WriteString("^")
	names := make([]string, 0, len(vars))
	for i, part := range parts {
		b.WriteString(regexp.QuoteMeta(part))
		if i < len(vars) {
			names = append(names, vars[i][1])
			b.WriteString(`(.+?)`)
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, false
	}
	m := re.FindStringSubmatch(msg)
	if m == nil {
		return nil, false
	}
	out := make(map[string]string, len(names))
	for i, name := range names {
		out[name] = strings.TrimSpace(m[i+1])
	}
	return out, true
}

func (b *Bundle) retranslateByTemplate(msg string) (string, bool) {
	msg = strings.TrimSpace(msg)
	keys := make([]string, 0, len(ru))
	for key := range ru {
		if !retranslateSkipKeys[key] {
			keys = append(keys, key)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		ri, rj := ru[keys[i]], ru[keys[j]]
		if len(ri) != len(rj) {
			return len(ri) > len(rj)
		}
		return keys[i] < keys[j]
	})
	for _, key := range keys {
		ruTpl := ru[key]
		enTpl := en[key]
		for _, tpl := range []string{ruTpl, enTpl} {
			if tpl == "" {
				continue
			}
			if !strings.Contains(tpl, "{{") {
				if msg == tpl {
					return b.T(key), true
				}
				continue
			}
			if vars, ok := matchI18nTemplate(tpl, msg); ok {
				return b.Tf(key, vars), true
			}
		}
	}
	return "", false
}

// RetranslateJobName localizes auto-generated quick-backup job names.
func (b *Bundle) RetranslateJobName(name string) string {
	if name == "" || b == nil {
		return name
	}
	if name == ru["quick.default_name"] || name == en["quick.default_name"] {
		return b.T("quick.default_name")
	}
	for _, prefix := range []string{"Быстрый ", "Quick "} {
		if rest, ok := strings.CutPrefix(name, prefix); ok {
			return b.Tf("quick.name_prefix", map[string]string{"time": rest})
		}
	}
	return name
}

// NormalizeBackupType maps legacy localized type labels to canonical codes.
func NormalizeBackupType(tpe string) string {
	switch strings.ToLower(strings.TrimSpace(tpe)) {
	case "full", "полный":
		return "full"
	case "incremental", "инкрементальный", "инкр.", "incr.":
		return "incremental"
	case "restore", "восстановление":
		return "restore"
	default:
		return tpe
	}
}

// FormatBackupType returns a localized backup type label for UI strings.
func (b *Bundle) FormatBackupType(tpe string) string {
	switch NormalizeBackupType(tpe) {
	case "full":
		return b.T("backup_type.full")
	case "incremental":
		return b.T("backup_type.incremental")
	case "restore":
		return b.T("backup_type.restore")
	default:
		return tpe
	}
}
