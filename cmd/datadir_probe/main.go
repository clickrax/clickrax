package main
import (
  "fmt"
  "pbs-win-backup/internal/config"
)
func main() {
  cfg, err := config.Load()
  if err != nil { fmt.Println("ERR:", err); return }
  fmt.Println("destinations:", len(cfg.Destinations))
  fmt.Println("jobs:", len(cfg.Jobs))
  fmt.Println("servers:", len(cfg.Servers))
}
