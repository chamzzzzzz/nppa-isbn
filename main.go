package main

import (
	"github.com/chamzzzzzz/nppa-isbn/isbn"
	_ "github.com/go-sql-driver/mysql"
	"github.com/robfig/cron/v3"
	"github.com/urfave/cli/v2"
	"log"
	"os"
	"time"
)

var (
	logger = log.New(os.Stdout, "nppa-isbn: ", log.Ldate|log.Lmicroseconds)
)

type App struct {
	cli      *cli.App
	database *isbn.Database
	tz       string
	spec     string
}

func (app *App) Run() error {
	app.database = &isbn.Database{}
	app.cli = &cli.App{
		Usage: "nppa isbn collector and monitoring",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "dn",
				Value:       "mysql",
				Usage:       "database driver name",
				Destination: &app.database.DN,
			},
			&cli.StringFlag{
				Name:        "dsn",
				Value:       "root:root@/nppa-isbn",
				Usage:       "database source name",
				Destination: &app.database.DSN,
			},
			&cli.StringFlag{
				Name:        "spec",
				Value:       "30 * * * *",
				Usage:       "cron spec",
				Destination: &app.spec,
			},
			&cli.StringFlag{
				Name:        "tz",
				Value:       "Local",
				Usage:       "time zone",
				Destination: &app.tz,
			},
		},
		Action: app.run,
	}
	return app.cli.Run(os.Args)
}

func (app *App) run(c *cli.Context) error {
	if err := app.database.Migrate(); err != nil {
		return err
	}
	logger.Printf("full collecting.")
	if _, err := app.collect(true); err != nil {
		return err
	}
	logger.Printf("full collecting finished.")
	return app.cron()
}

func (app *App) cron() error {
	logger.Printf("monitoring.")
	c := cron.New(
		cron.WithLocation(location(app.tz)),
		cron.WithLogger(cron.VerbosePrintfLogger(logger)),
		cron.WithChain(cron.SkipIfStillRunning(cron.VerbosePrintfLogger(logger))),
	)
	c.AddFunc(app.spec, app.monitoring)
	c.Run()
	return nil
}

func (app *App) collect(full bool) ([]*isbn.Content, error) {
	page := 1
	if full {
		page = 10
	}

	var contents []*isbn.Content
	for _, channelID := range []string{isbn.ChannelImportOnlineGameApprovaled, isbn.ChannelImportElectronicGameApprovaled, isbn.ChannelMadeInChinaOnlineGameApprovaled, isbn.ChannelGameChanged, isbn.ChannelGameRevoked} {
		channel, err := isbn.GetChannel(channelID, page)
		if err != nil {
			return contents, err
		}

		for _, content := range channel.Contents {
			if has, err := app.database.HasContent(content); err != nil {
				return contents, err
			} else if has {
				continue
			}

			if content, err := isbn.GetContent(content.ChannelID, content.ID); err != nil {
				return contents, err
			} else {
				if err := app.database.AddContent(content); err != nil {
					return contents, err
				}
				contents = append(contents, content)
			}
		}
	}
	return contents, nil
}

func (app *App) monitoring() {
	if contents, err := app.collect(false); err != nil {
		logger.Printf("monitoring, err='%s'\n", err)
	} else {
		if len(contents) > 0 {
			logger.Printf("monitoring found new isbn content.")
			app.notification(contents)
		}
	}
}

func (app *App) notification(contents []*isbn.Content) {
	logger.Printf("send notification.")
}

func location(tz string) *time.Location {
	if loc, err := time.LoadLocation(tz); err != nil {
		return time.Local
	} else {
		return loc
	}
}

func main() {
	if err := (&App{}).Run(); err != nil {
		logger.Printf("run, err='%s'\n", err)
	}
}
