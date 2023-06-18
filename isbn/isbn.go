package isbn

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/chamzzzzzz/supersimplesoup"
)

const (
	ChannelImportOnlineGameApprovaled      = "jkwlyxspxx"
	ChannelImportElectronicGameApprovaled  = "jkdzyxspxx"
	ChannelMadeInChinaOnlineGameApprovaled = "gcwlyxspxx"
	ChannelGameChanged                     = "yxspbgxx"
	ChannelGameRevoked                     = "yxspcxxx"
)

var (
	ChannelChineseNames = map[string]string{
		ChannelImportOnlineGameApprovaled:      "进口网络游戏审批信息",
		ChannelImportElectronicGameApprovaled:  "进口电子游戏审批信息",
		ChannelMadeInChinaOnlineGameApprovaled: "国产网络游戏审批信息",
		ChannelGameChanged:                     "游戏审批变更信息",
		ChannelGameRevoked:                     "游戏审批撤销信息",
	}
	re = regexp.MustCompile("var _sblb = '(.*)';")
)

type Content struct {
	Title string  `json:"title"`
	URL   string  `json:"url"`
	Date  string  `json:"date"`
	Items []*Item `json:"items"`
}

type Item struct {
	Seq            string `json:"seq"`
	Name           string `json:"name"`
	Catalog        string `json:"catalog,omitempty"`
	Publisher      string `json:"publisher,omitempty"`
	Operator       string `json:"operator,omitempty"`
	ApprovalNumber string `json:"approvalNumber,omitempty"`
	ISBN           string `json:"isbn,omitempty"`
	Date           string `json:"date,omitempty"`
	ChangeInfo     string `json:"changeInfo,omitempty"`
	RevokeInfo     string `json:"revokeInfo,omitempty"`
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

		html, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, err
		}
		return html, nil
	}
	return nil, fmt.Errorf("bad gateway")
}

func GetDOM(url string) (*supersimplesoup.Node, error) {
	html, err := GetHTML(url, 10)
	if err != nil {
		return nil, err
	}
	dom, err := supersimplesoup.Parse(bytes.NewReader(html))
	if err != nil {
		return nil, err
	}
	return dom, nil
}

func GetPageContents(page int, getItem bool) ([]*Content, error) {
	var contents []*Content
	suffix := ""
	if page > 0 {
		suffix = fmt.Sprintf("_%d", page)
	}

	url := fmt.Sprintf("https://www.nppa.gov.cn/bsfw/jggs/yxspjg/index%s.html", suffix)
	dom, err := GetDOM(url)
	if err != nil {
		return nil, err
	}

	div, err := dom.Find("div", "class", "g-font-size-140 g-font-size-100--2xs g-line-height-1 g-mb-10")
	if err == nil && strings.TrimSpace(div.Text()) == "404" {
		return nil, fmt.Errorf("404")
	}

	for _, div := range dom.QueryAll("div", "class", "ellipsis") {
		a, err := div.Find("a")
		if err != nil {
			return nil, err
		}
		span, err := div.ParentNode().Find("span")
		if err != nil {
			return nil, err
		}

		content := &Content{
			Title: strings.TrimSpace(a.Text()),
			URL:   strings.TrimPrefix(strings.TrimSpace(a.Href()), "./"),
			Date:  strings.Trim(strings.TrimSpace(span.Text()), "[]"),
		}
		if getItem {
			if err := content.GetItems(); err != nil {
				return nil, err
			}
		}
		contents = append(contents, content)
	}
	return contents, nil
}

func (c *Content) GetChannel() string {
	f := strings.Split(c.URL, "/")
	if len(f) != 3 {
		return ""
	}
	return f[0]
}

func (c *Content) GetChannelChineseName() string {
	return ChannelChineseNames[c.GetChannel()]
}

func (c *Content) GetItems() error {
	url := "https://www.nppa.gov.cn/bsfw/jggs/yxspjg/" + c.URL
	dom, err := GetDOM(url)
	if err != nil {
		return err
	}

	table, err := dom.Find("table", "class", "trStyle tableOrder")
	if err != nil {
		return err
	}

	channel := c.GetChannel()
	var items []*Item
	for _, tr := range table.QueryAll("tr")[1:] {
		td := tr.QueryAll("td")
		item := &Item{}
		item.Seq = strings.TrimSpace(td[0].Text())
		item.Name = strings.TrimSpace(td[1].Text())
		switch channel {
		case ChannelImportElectronicGameApprovaled:
			if len(td) != 5 {
				return fmt.Errorf("item field len error")
			}
			item.Publisher = td[2].Text()
			item.ApprovalNumber = td[3].Text()
			item.Date = td[4].Text()
		case ChannelImportOnlineGameApprovaled, ChannelMadeInChinaOnlineGameApprovaled:
			if len(td) != 7 {
				return fmt.Errorf("item field len error")
			}
			script, err := tr.Find("script")
			if err != nil {
				return err
			}
			item.Publisher = strings.TrimSpace(td[2].Text())
			item.Operator = strings.TrimSpace(td[3].Text())
			item.ApprovalNumber = strings.TrimSpace(td[4].Text())
			item.ISBN = strings.TrimSpace(td[5].Text())
			item.Date = strings.TrimSpace(td[6].Text())
			matches := re.FindStringSubmatch(strings.ReplaceAll(script.Text(), "\n", ""))
			if len(matches) != 2 {
				return fmt.Errorf("catalog not found")
			}
			item.Catalog = matches[1]
		case ChannelGameChanged:
			if len(td) != 8 {
				return fmt.Errorf("item field len error")
			}
			item.Catalog = strings.TrimSpace(td[2].Text())
			item.Publisher = strings.TrimSpace(td[3].Text())
			item.Operator = strings.TrimSpace(td[4].Text())
			item.ChangeInfo = strings.TrimSpace(td[5].Text())
			item.ApprovalNumber = strings.TrimSpace(td[6].Text())
			item.Date = strings.TrimSpace(td[7].Text())
		case ChannelGameRevoked:
			if len(td) != 9 {
				return fmt.Errorf("item field len error")
			}
			item.Catalog = strings.TrimSpace(td[2].Text())
			item.Publisher = strings.TrimSpace(td[3].Text())
			item.Operator = strings.TrimSpace(td[4].Text())
			item.RevokeInfo = strings.TrimSpace(td[5].Text())
			item.ApprovalNumber = strings.TrimSpace(td[6].Text())
			item.ISBN = strings.TrimSpace(td[7].Text())
			item.Date = strings.TrimSpace(td[8].Text())
		}
		items = append(items, item)
	}
	c.Items = items
	return nil
}
