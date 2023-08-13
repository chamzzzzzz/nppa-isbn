package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"mime"
	"net"
	"net/smtp"
	"os"
	"text/template"

	"github.com/chamzzzzzz/nppa-isbn/isbn"
)

var (
	addr = os.Getenv("ISBN_SMTP_ADDR")
	user = os.Getenv("ISBN_SMTP_USER")
	pass = os.Getenv("ISBN_SMTP_PASS")
	t    = template.Must(template.New("isbn").Parse("From: {{.From}}\r\nTo: {{.To}}\r\nSubject: {{.Subject}}\r\n\r\n{{.Body}}"))
)

func main() {
	var full bool
	flag.BoolVar(&full, "full", false, "full archive")
	flag.StringVar(&addr, "addr", addr, "notification smtp addr")
	flag.StringVar(&user, "user", user, "notification smtp user")
	flag.StringVar(&pass, "pass", pass, "notification smtp pass")
	flag.Parse()

	page := 1
	if full {
		page = 30
		log.Printf("full archive. page=%d", page)
	} else {
		log.Printf("increment archive. page=%d", page)
	}

	var bn []*isbn.Content
	for i := 0; i < page; i++ {
		contents, err := isbn.GetPageContents(i, false)
		if err != nil {
			if err.Error() == "404" {
				break
			}
			log.Printf("get page contents fail. page=%d, err='%s'", i, err)
			return
		}
		log.Printf("get page contents success. page=%d, contents=%d", i, len(contents))
		bn = append(bn, contents...)
	}
	log.Printf("get contents success. contents=%d", len(bn))

	var newbn []*isbn.Content
	for _, content := range bn {
		p := fmt.Sprintf("data/%s.json", content.Title)
		_, err := os.Stat(p)
		if err == nil {
			log.Printf("skip archived content. title=%s, path=%s", content.Title, p)
			continue
		}
		if !os.IsNotExist(err) {
			log.Printf("stat content fail. title=%s, path=%s, err='%s'", content.Title, p, err)
			return
		}
		if err = content.GetItems(); err != nil {
			log.Printf("get content items fail. title=%s, err='%s'", content.Title, err)
			return
		}
		if len(content.Items) == 0 {
			log.Printf("skip empty content. title=%s", content.Title)
			continue
		}
		log.Printf("get content items success. title=%s, items=%d", content.Title, len(content.Items))
		newbn = append(newbn, content)
	}

	err := os.MkdirAll("data", 0755)
	if err != nil {
		if !os.IsExist(err) {
			log.Printf("mkdir data fail. err='%s'", err)
			return
		}
	}

	for _, content := range newbn {
		p := fmt.Sprintf("data/%s.json", content.Title)
		b, err := json.MarshalIndent(content, "", "  ")
		if err != nil {
			log.Printf("marshal content fail. title=%s, err='%s'", content.Title, err)
			return
		}
		err = os.WriteFile(p, b, 0644)
		if err != nil {
			log.Printf("write content fail. title=%s, err='%s'", content.Title, err)
			return
		}
		log.Printf("write content success. title=%s, path=%s", content.Title, p)
	}

	if !full && len(newbn) > 0 {
		notification(newbn)
	}
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
