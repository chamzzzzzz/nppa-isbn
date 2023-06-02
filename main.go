package main

import (
	"bytes"
	"fmt"
	"log"
	"mime"
	"net"
	"net/smtp"
	"os"
	"text/template"
	"time"

	"github.com/chamzzzzzz/nppa-isbn/isbn"
	_ "github.com/go-sql-driver/mysql"
	"github.com/robfig/cron/v3"
	"github.com/urfave/cli/v3"
)

var (
	logger = log.New(os.Stdout, "nppa-isbn: ", log.Ldate|log.Lmicroseconds)
)

var (
	templateSource = "From: {{.From}}\r\nTo: {{.To}}\r\nSubject: {{.Subject}}\r\n\r\n{{.Body}}"
)

type App struct {
	cli                *cli.App
	database           *isbn.Database
	full               bool
	tz                 string
	spec               string
	enableNotification bool
	smtpAddr           string
	smtpUser           string
	smtpPass           string
	mailTemplateFile   string
	template           *template.Template
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
				Category:    "database",
			},
			&cli.StringFlag{
				Name:        "dsn",
				Value:       "root:root@/nppa-isbn",
				Usage:       "database source name",
				Destination: &app.database.DSN,
				Category:    "database",
			},
			&cli.BoolFlag{
				Name:        "full",
				Value:       false,
				Usage:       "full collect",
				Destination: &app.full,
				Category:    "mode",
			},
			&cli.StringFlag{
				Name:        "spec",
				Value:       "30 * * * *",
				Usage:       "cron spec",
				Destination: &app.spec,
				Category:    "cron",
			},
			&cli.StringFlag{
				Name:        "tz",
				Value:       "Local",
				Usage:       "time zone",
				Destination: &app.tz,
				Category:    "cron",
			},
			&cli.BoolFlag{Name: "notification", Value: false, EnvVars: []string{"NPPA_ISBN_NOTIFICATION"}, Destination: &app.enableNotification, Category: "notification"},
			&cli.StringFlag{Name: "smtp-addr", Value: "smtp.mail.me.com:587", EnvVars: []string{"NPPA_ISBN_SMTP_ADDR"}, Destination: &app.smtpAddr, Category: "notification"},
			&cli.StringFlag{Name: "smtp-user", EnvVars: []string{"NPPA_ISBN_SMTP_USER"}, Destination: &app.smtpUser, Category: "notification"},
			&cli.StringFlag{Name: "smtp-password", EnvVars: []string{"NPPA_ISBN_SMTP_PASSWORD"}, Destination: &app.smtpPass, Category: "notification"},
			&cli.StringFlag{Name: "mail-template-file", EnvVars: []string{"NPPA_ISBN_MAIL_TEMPLATE_FILE"}, Destination: &app.mailTemplateFile, Category: "notification"},
		},
		Action: app.run,
	}
	return app.cli.Run(os.Args)
}

func (app *App) run(c *cli.Context) error {
	source := templateSource
	if app.mailTemplateFile != "" {
		b, err := os.ReadFile(app.mailTemplateFile)
		if err != nil {
			return err
		}
		source = string(b)
	}

	funcs := template.FuncMap{
		"bencoding": mime.BEncoding.Encode,
	}
	if t, err := template.New("mail").Funcs(funcs).Parse(source); err != nil {
		return err
	} else {
		app.template = t
	}

	if err := app.database.Migrate(); err != nil {
		return err
	}
	if app.full {
		logger.Printf("full collecting.")
		if _, err := app.collect(true); err != nil {
			return err
		}
		logger.Printf("full collecting finished.")
	}
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
	type Data struct {
		From    string
		To      string
		Subject string
		Body    string
		Content *isbn.Content
	}

	if !app.enableNotification {
		logger.Printf("notification disabled.")
		return
	}
	logger.Printf("sending notification...")

	addr := app.smtpAddr
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		logger.Printf("send notification fail. err='%s'\n", err)
		return
	}
	user := app.smtpUser
	password := app.smtpPass
	to := []string{user}

	for _, content := range contents {
		data := Data{
			From:    fmt.Sprintf("%s <%s>", mime.BEncoding.Encode("UTF-8", "ISBN Monitor"), user),
			To:      to[0],
			Subject: mime.BEncoding.Encode("UTF-8", fmt.Sprintf("ISBN-%s", content.Title)),
			Body:    content.URL,
			Content: content,
		}

		var buf bytes.Buffer
		if err := app.template.Execute(&buf, data); err != nil {
			logger.Printf("send notification fail. title='%s', err='%s'\n", content.Title, err)
			continue
		}

		auth := smtp.PlainAuth("", user, password, host)
		if err := smtp.SendMail(addr, auth, user, to, buf.Bytes()); err != nil {
			logger.Printf("send notification fail. title='%s', err='%s'\n", content.Title, err)
		}
		logger.Printf("send notification success. title='%s'\n", content.Title)
	}
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
