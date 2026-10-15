package main

import (
	"flag"
	"fmt"
	"github.com/oneworkspacestudio/calendar/internal/app"
	"log"
	"os"
	"path/filepath"
)

func main() {
	var o app.Options
	o.Kind = "calendar"
	flag.StringVar(&o.Root, "shared-data", "", "shared Stalwart installation folder")
	flag.StringVar(&o.Data, "data", "", "app preferences folder")
	flag.StringVar(&o.Listen, "listen", "127.0.0.1:8998", "loopback app address")
	flag.StringVar(&o.Origin, "origin", "", "public app origin")
	flag.StringVar(&o.Base, "base", os.Getenv("WORKSPACE_BASE_PATH"), "proxy base path")
	flag.BoolVar(&o.Demo, "demo", false, "isolated synthetic UI demonstration (no MyAccounts/native backend)")
	flag.BoolVar(&o.NoBrowser, "no-browser", false, "keep browser closed")
	exe, _ := os.Executable()
	flag.StringVar(&o.Manager, "services", filepath.Join(filepath.Dir(exe), "WorkspaceServices.exe"), "shared manager executable")
	showVersion := flag.Bool("version", false, "show application version")
	flag.Parse()
	if *showVersion {
		fmt.Println(app.Version)
		return
	}
	if e := app.Run(o); e != nil {
		log.Fatal(e)
	}
}
