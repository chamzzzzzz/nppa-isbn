package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"mime"
	"net"
	"net/smtp"
	"os"
	"text/template"
	"time"

	"github.com/chamzzzzzz/nppa-isbn/isbn"
)

var (
	latest []*isbn.Content
	addr   = os.Getenv("ISBN_SMTP_ADDR")
	user   = os.Getenv("ISBN_SMTP_USER")
	pass   = os.Getenv("ISBN_SMTP_PASS")
	source = "From: {{.From}}\r\nTo: {{.To}}\r\nSubject: {{.Subject}}\r\n\r\n{{.Body}}"
	t      *template.Template
)

func main() {
	funcs := template.FuncMap{
		"bencoding": mime.BEncoding.Encode,
	}
	t = template.Must(template.New("mail").Funcs(funcs).Parse(source))

	contents, err := isbn.GetPageContents(0, false)
	if err != nil {
		log.Fatalf("get latest contents fail. err='%s'", err)
	}
	if len(contents) == 0 {
		log.Fatalf("get latest contents fail, contents is empty")
	}
	log.Printf("get latest contents success. count=%d", len(contents))
	latest = contents

	begin, end := 10, 20
	for {
		now := time.Now()
		next := calc(now, begin, end)
		log.Printf("next check at %s", next.Format("2006-01-02 15:04:05"))
		time.Sleep(next.Sub(now))
		check(begin, end, 10*time.Minute)
	}
}

func calc(now time.Time, begin, end int) time.Time {
	t := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	days := 1
	d := time.Duration(begin) * time.Hour
	w := now.Weekday()
	if w == time.Sunday {
		days = 1
	} else if w == time.Saturday {
		days = 2
	} else {
		if now.Hour() < begin {
			days = 0
		} else if now.Hour() >= begin && now.Hour() <= end {
			days = 0
			d = now.Add(1 * time.Second).Sub(t)
		} else {
			if w == time.Friday {
				days = 3
			}
		}
	}
	return t.AddDate(0, 0, days).Add(d)
}

func check(begin, end int, d time.Duration) {
	start := time.Now()
	b := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), begin, 0, 0, 0, time.Local)
	e := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), end, 0, 0, 0, time.Local)
	log.Printf("start check at %s", start.Format("2006-01-02 15:04:05"))
	log.Printf("checking from [%v] to [%v] every %v.", b.Format("15:04:05"), e.Format("15:04:05"), d)
	count := 0
	for {
		now := time.Now()
		if now.Hour() < begin || now.Hour() > end {
			break
		}
		count++
		contents, err := diff()
		if err != nil {
			log.Printf("check fail, diff error. err='%s'", err)
		}
		if len(contents) > 0 {
			notification(contents)
			snapshot()
		}
		time.Sleep(d)
	}
	log.Printf("finish check at %s. used=%v, count=%d", time.Now().Format("2006-01-02 15:04:05"), time.Since(start), count)
}

func diff() ([]*isbn.Content, error) {
	contents, err := isbn.GetPageContents(0, false)
	if err != nil {
		return nil, err
	}
	if len(contents) == 0 {
		return nil, fmt.Errorf("get contents is empty")
	}

	var newest []*isbn.Content
	for _, content := range contents {
		var exist bool
		for _, l := range latest {
			if content.URL == l.URL {
				exist = true
				break
			}
		}
		if !exist {
			content.GetItems()
			newest = append(newest, content)
		}
	}
	if len(newest) > 0 {
		latest = contents
	}
	return newest, nil
}

func notification(contents []*isbn.Content) {
	type Data struct {
		From     string
		To       string
		Subject  string
		Body     string
		Contents []*isbn.Content
	}

	log.Printf("sending notification...")
	if addr == "" {
		log.Printf("send notification skip. addr is empty")
		return
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		log.Printf("send notification fail. err='%s'\n", err)
		return
	}

	body := ""
	subject := "审批信息"
	for _, content := range contents {
		body += fmt.Sprintf("%s (%d)\r\n", content.Title, len(content.Items))
	}

	data := Data{
		From:     fmt.Sprintf("%s <%s>", mime.BEncoding.Encode("UTF-8", "Monitor"), user),
		To:       user,
		Subject:  mime.BEncoding.Encode("UTF-8", fmt.Sprintf("「ISBN」%s", subject)),
		Body:     body,
		Contents: contents,
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		log.Printf("send notification fail. err='%s'\n", err)
		return
	}

	auth := smtp.PlainAuth("", user, pass, host)
	if err := smtp.SendMail(addr, auth, user, []string{user}, buf.Bytes()); err != nil {
		log.Printf("send notification fail. err='%s'\n", err)
	}
	log.Printf("send notification success.\n")
}

func snapshot() error {
	log.Printf("snapshot...")
	var bn []*isbn.Content
	for i := 0; i < 30; i++ {
		contents, err := isbn.GetPageContents(i, true)
		if err != nil {
			if err.Error() == "404" {
				break
			}
			log.Printf("snapshot fail, get page contents error. page=%d, err='%s'", i, err)
			return err
		}
		bn = append(bn, contents...)
	}
	b, err := json.MarshalIndent(bn, "", "  ")
	if err != nil {
		log.Printf("snapshot fail, marshal error. err='%s'", err)
		return err
	}
	file := fmt.Sprintf("isbn-snapshot-%s.json", time.Now().Format("20060102150405"))
	err = os.WriteFile(file, b, 0644)
	if err != nil {
		log.Printf("snapshot fail, write file error. err='%s'", err)
		return err
	}
	log.Printf("snapshot success. file=%s", file)
	return nil
}
