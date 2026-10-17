// admin bootstraps console accounts: go run ./cmd/admin -email you@x.in -role admin
// It prints a one-time password and an otpauth:// URL to scan into an authenticator app.
package main

import (
	"flag"
	"fmt"
	"os"

	"metro-buddy/server/internal/admin"
	"metro-buddy/server/internal/config"
	"metro-buddy/server/internal/db"
	"metro-buddy/server/internal/model"
	"metro-buddy/server/internal/secure"
)

func main() {
	email := flag.String("email", "", "admin email")
	role := flag.String("role", "moderator", "moderator | trust_safety_lead | admin")
	flag.Parse()
	if _, ok := model.AdminRoleRank[model.AdminRole(*role)]; *email == "" || !ok {
		flag.Usage()
		os.Exit(2)
	}
	cfg, err := config.Load()
	if err != nil {
		fail(err)
	}
	gdb, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		fail(err)
	}
	if err := db.Migrate(gdb); err != nil {
		fail(err)
	}
	box, err := secure.NewBox(cfg.DataKey)
	if err != nil {
		fail(err)
	}
	a, pw, url, err := admin.NewAdmin(gdb, box, *email, model.AdminRole(*role))
	if err != nil {
		fail(err)
	}
	fmt.Printf("created %s (%s)\npassword: %s\ntotp: %s\n", a.Email, a.Role, pw, url)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
