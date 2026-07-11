package main

import (
	"fmt"
	"os"

	"pbs-win-backup/internal/credential"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: checkcred <destination-id>")
		os.Exit(2)
	}
	id := os.Args[1]
	s, err := credential.GetSecret(id)
	if err != nil {
		fmt.Println("ERR:", err)
		os.Exit(1)
	}
	fmt.Printf("OK secret length=%d\n", len(s))
}
