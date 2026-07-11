package main

import (
	"fmt"
	"os"

	"pbs-win-backup/internal/config"
	"pbs-win-backup/internal/credential"
	"pbs-win-backup/internal/pbs"
	"pbs-win-backup/internal/pbsbackup"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("config:", err)
		os.Exit(1)
	}
	for _, s := range cfg.Servers {
		fmt.Println("=== server", s.Name, s.ID, "===")
		secret, err := credential.GetSecret(s.ID)
		if err != nil {
			fmt.Println("secret:", err)
			continue
		}
		fmt.Printf("secret len=%d token_id=%q namespace=%q datastore=%q\n",
			len(secret), s.TokenID, s.Namespace, s.Datastore)

		c := pbs.NewClient(s, secret)
		r := c.TestConnection()
		fmt.Printf("REST TestConnection: ok=%v\n  %s\n", r.OK, r.Message)

		if err := pbsbackup.ProbeBackupAccess(s, secret, "Dan"); err != nil {
			fmt.Println("Backup protocol:", err)
		} else {
			fmt.Println("Backup protocol: OK")
		}
	}
}
