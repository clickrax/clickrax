package main

import (
	"fmt"
	"os"

	"pbs-win-backup/internal/config"
	"pbs-win-backup/internal/credential"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("config:", err)
		os.Exit(1)
	}
	ids := make([]string, 0, len(cfg.Servers))
	for _, s := range cfg.Servers {
		ids = append(ids, s.ID)
	}
	credential.MigrateSecrets(ids)
	for _, id := range ids {
		ok := credential.HasSecret(id)
		fmt.Printf("server %s service-ready=%v\n", id, ok)
	}
}
