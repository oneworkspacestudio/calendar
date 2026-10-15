package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/oneworkspacestudio/calendar/shared"
	"log"
	"os"
	"os/signal"
)

var version = "0.1.0-preview.1"

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Use install, run, status, uninstall-app, upgrade, external or autostart")
	}
	command := os.Args[1]
	flags := flag.NewFlagSet(command, flag.ExitOnError)
	root := flags.String("root", shared.DefaultRoot(), "shared private NTFS data folder")
	app := flags.String("app", "calendar", "calendar, mail or contacts")
	appVersion := flags.String("app-version", version, "dependent app version")
	settings := flags.String("settings", "", "initial/upgrade JSON settings; imported into protected storage")
	output := flags.String("output", "", "new private NTFS backup folder")
	enabled := flags.Bool("enabled", false, "explicitly enable login startup")
	smtp := flags.Int("smtp-port", 0, "explicit private SMTP port (zero disables)")
	imap := flags.Int("imap-port", 0, "explicit private IMAP port (zero disables)")
	publicSMTP := flags.Int("public-smtp-port", 0, "explicit public SMTP port (zero disables)")
	allowPublic := flags.Bool("allow-public-smtp", false, "explicitly authorize the public SMTP listener")
	flags.Parse(os.Args[2:])
	var cfg *shared.Config
	if *settings != "" {
		b, e := os.ReadFile(*settings)
		if e != nil {
			log.Fatal(e)
		}
		cfg = &shared.Config{}
		if e = json.Unmarshal(b, cfg); e != nil {
			log.Fatal(e)
		}
	}
	var e error
	switch command {
	case "mail-listeners":
		e = shared.ConfigureMailListeners(context.Background(), *root, shared.MailListeners{SMTP: *smtp, IMAP: *imap, PublicSMTP: *publicSMTP, AllowPublic: *allowPublic})
	case "install":
		_, e = shared.Install(*root, *app, *appVersion, cfg)
	case "uninstall-app":
		e = shared.Uninstall(*root, *app)
	case "run":
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		e = shared.Run(ctx, *root)
	case "upgrade":
		if cfg == nil {
			log.Fatal("upgrade requires --settings")
		}
		e = shared.Upgrade(*root, *cfg)
	case "stop":
		e = shared.Stop(context.Background(), *root)
	case "backup":
		e = shared.Backup(*root, *output)
	case "status":
		c, s, err := shared.Load(*root)
		if err != nil {
			e = err
			break
		}
		run, _ := shared.ReadRunning(*root)
		json.NewEncoder(os.Stdout).Encode(map[string]any{"version": version, "service": s, "running": run, "engineURL": shared.EngineURL(c)})
	case "discover":
		json.NewEncoder(os.Stdout).Encode(shared.DiscoverManaged())
	case "external":
		json.NewEncoder(os.Stdout).Encode(shared.DiscoverExternal(""))
	case "autostart":
		exe, _ := os.Executable()
		e = shared.Autostart(*root, exe, *enabled)
	case "version":
		fmt.Println(version)
	default:
		e = fmt.Errorf("unknown command %s", command)
	}
	if e != nil {
		log.Fatal(e)
	}
}
