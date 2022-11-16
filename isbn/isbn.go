package isbn

import (
	"fmt"
	"github.com/anaskhan96/soup"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"time"
)

const (
	ChannelImportOnlineGameApprovaled      = "318"
	ChannelImportElectronicGameApprovaled  = "319"
	ChannelMadeInChinaOnlineGameApprovaled = "320"
	ChannelGameChanged                     = "321"
	ChannelGameRevoked                     = "747"
)

type Channel struct {
	ID       string
	Contents []*Content
}

type Content struct {
	ChannelID string
	ID        string
	Title     string
	URL       string
	Date      string
	Items     []*Item
}

type Item struct {
	ChannelID      string
	ContentID      string
	Seq            string
	Name           string
	Catalog        string
	Publisher      string
	Operator       string
	ApprovalNumber string
	ISBN           string
	Date           string
	ChangeInfo     string
	RevokeInfo     string
}

func GetHTML(url string, retries int) ([]byte, error) {
	for i := 0; i < retries; i++ {
		res, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		defer res.Body.Close()

		if res.StatusCode == http.StatusBadGateway {
			time.Sleep(500 * time.Millisecond)
			continue
		}

		html, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		return html, nil
	}
	return nil, fmt.Errorf("bad gateway")
}

func GetDOM(url string) (soup.Root, error) {
	html, err := GetHTML(url, 10)
	if err != nil {
		return soup.Root{}, err
	}
	dom := soup.HTMLParse(string(html))
	if dom.Error != nil {
		return soup.Root{}, dom.Error
	}
	return dom, nil
}

func GetChannel(channelID string, page int) (*Channel, error) {
	channel := &Channel{ID: channelID}
	for i := 1; i <= page; i++ {
		suffix := ""
		if i > 1 {
			suffix = fmt.Sprintf("_%d", i)
		}
		url := fmt.Sprintf("https://www.nppa.gov.cn/nppa/channels/%s%s.shtml", channelID, suffix)

		dom, err := GetDOM(url)
		if err != nil {
			return nil, err
		}

		div := dom.FindStrict("div", "class", "g-font-size-140 g-font-size-100--2xs g-line-height-1 g-mb-10")
		if div.Error == nil && strings.TrimSpace(div.Text()) == "404" {
			break
		}

		for _, div := range dom.FindAllStrict("div", "class", "ellipsis") {
			a := div.Find("a")
			if a.Error != nil {
				return nil, a.Error
			}
			span := div.FindNextSibling()
			if span.Error != nil {
				return nil, span.Error
			}

			href := strings.TrimSpace(a.Attrs()["href"])
			content := &Content{
				ChannelID: channelID,
				ID:        path.Base(strings.Trim(href, ".shtml")),
				Title:     strings.TrimSpace(a.Text()),
				URL:       fmt.Sprintf("https://www.nppa.gov.cn%s", href),
			}
			channel.Contents = append(channel.Contents, content)
		}
	}
	return channel, nil
}

func GetContent(channelID, contentID string) (*Content, error) {
	content := &Content{
		ChannelID: channelID,
		ID:        contentID,
		URL:       fmt.Sprintf("https://www.nppa.gov.cn/nppa/contents/%s/%s.shtml", channelID, contentID),
	}
	dom, err := GetDOM(content.URL)
	if err != nil {
		return nil, err
	}

	h2 := dom.FindStrict("h2", "class", "m3page_t")
	if h2.Error != nil {
		return nil, h2.Error
	}
	content.Title = strings.TrimSpace(h2.Text())

	span := dom.FindStrict("span", "class", "m3pageFun_s1")
	if span.Error != nil {
		return nil, span.Error
	}
	content.Date = strings.TrimSpace(span.Text())

	style := "trStyle"
	if channelID == ChannelMadeInChinaOnlineGameApprovaled {
		style = "trStyle tableNormal"
	}
	table := dom.FindStrict("table", "class", style)
	if table.Error != nil {
		return nil, table.Error
	}
	for _, tr := range table.FindAll("tr")[1:] {
		td := tr.FindAll("td")
		item := &Item{ChannelID: channelID, ContentID: contentID}
		item.Seq = strings.TrimSpace(td[0].Text())
		item.Name = strings.TrimSpace(td[1].Text())
		switch channelID {
		case ChannelImportElectronicGameApprovaled:
			if len(td) != 5 {
				return nil, fmt.Errorf("item field len error")
			}
			item.Publisher = td[2].Text()
			item.ApprovalNumber = td[3].Text()
			item.Date = td[4].Text()
		case ChannelImportOnlineGameApprovaled, ChannelMadeInChinaOnlineGameApprovaled:
			if len(td) < 7 {
				return nil, fmt.Errorf("item field len error")
			}
			item.Catalog = strings.TrimSpace(td[2].Text())
			item.Publisher = strings.TrimSpace(td[3].Text())
			item.Operator = strings.TrimSpace(td[4].Text())
			item.ApprovalNumber = strings.TrimSpace(td[5].Text())
			if len(td) > 7 {
				item.ISBN = strings.TrimSpace(td[6].Text())
				item.Date = strings.TrimSpace(td[7].Text())
			} else {
				item.Date = strings.TrimSpace(td[6].Text())
			}
		case ChannelGameChanged:
			if len(td) < 8 {
				return nil, fmt.Errorf("item field len error")
			}
			item.Catalog = strings.TrimSpace(td[2].Text())
			item.Publisher = strings.TrimSpace(td[3].Text())
			item.Operator = strings.TrimSpace(td[4].Text())
			item.ChangeInfo = strings.TrimSpace(td[5].Text())
			item.ApprovalNumber = strings.TrimSpace(td[6].Text())
			if len(td) > 8 {
				item.ISBN = strings.TrimSpace(td[7].Text())
				item.Date = strings.TrimSpace(td[8].Text())
			} else {
				item.Date = strings.TrimSpace(td[7].Text())
			}
		case ChannelGameRevoked:
			if len(td) != 9 {
				return nil, fmt.Errorf("item field len error")
			}
			item.Catalog = strings.TrimSpace(td[2].Text())
			item.Publisher = strings.TrimSpace(td[3].Text())
			item.Operator = strings.TrimSpace(td[4].Text())
			item.RevokeInfo = strings.TrimSpace(td[5].Text())
			item.ApprovalNumber = strings.TrimSpace(td[6].Text())
			item.ISBN = strings.TrimSpace(td[7].Text())
			item.Date = strings.TrimSpace(td[8].Text())
		}
		content.Items = append(content.Items, item)
	}
	return content, nil
}
