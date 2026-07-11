package pbsbackup

import "strings"

func catalogPath(rel string) string {
	return strings.ReplaceAll(rel, "/", `\`)
}
