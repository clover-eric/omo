package main

import (
	"context"
	"fmt"
	"os"

	"omo/internal/store"
	"omo/internal/version"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Printf("omoctl %s\n", version.Info())
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "dev-seed-ready" {
		if len(os.Args) != 4 {
			fmt.Println("usage: omoctl dev-seed-ready DB_PATH DOMAIN")
			os.Exit(2)
		}
		if err := seedReady(os.Args[2], os.Args[3]); err != nil {
			fmt.Fprintf(os.Stderr, "seed ready: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Println("omoctl: administrative commands will be added in later phases")
}

func seedReady(dbPath string, domain string) error {
	appStore, err := store.Open(context.Background(), dbPath)
	if err != nil {
		return err
	}
	defer appStore.Close()
	if err := appStore.SetSetting(context.Background(), "bootstrap.phase1_complete", "true"); err != nil {
		return err
	}
	return appStore.SetSetting(context.Background(), "bootstrap.domain", domain)
}
