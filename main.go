package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/anaskhan96/soup"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
)

type ChannelType string

const (
	ChannelTypeImportOnlineGame      = "进口网络游戏审批信息"
	ChannelTypeImportElectronicGame  = "进口电子游戏审批信息"
	ChannelTypeMadeInChinaOnlineGame = "国产网络游戏审批信息"
	ChannelTypeChange                = "游戏审批变更信息"
	ChannelTypeRevoke                = "游戏审批撤销信息"
)

var ChannelTypeUrlNumber = map[ChannelType]string{
	ChannelTypeImportOnlineGame:      "318",
	ChannelTypeImportElectronicGame:  "319",
	ChannelTypeMadeInChinaOnlineGame: "320",
	ChannelTypeChange:                "321",
	ChannelTypeRevoke:                "747",
}

var (
	ErrInvalidChannelPageUrl = errors.New("invalid channel page url")
	ErrInvalidChannelType    = errors.New("invalid channel type")
)

type Channel struct {
	Type     ChannelType
	Contents map[string]*Content
}

type Content struct {
	ChannelType ChannelType
	Url         string
	Id          string
	Title       string
	Items       []*Item
}

type Item struct {
	ChannelType    ChannelType
	ContentId      string
	Seq            string
	Name           string
	Type           string
	Publisher      string
	Operator       string
	ApprovalNumber string
	ISBN           string
	Time           string
	ChangeInfo     string
	RevokeInfo     string
}

type ISBN struct {
	Channels map[ChannelType]*Channel
}

func (content *Content) NewItem() *Item {
	return &Item{
		ChannelType: content.ChannelType,
		ContentId:   content.Id,
	}
}

func (content *Content) NewAndAppendItem() *Item {
	return content.AppendItem(content.NewItem())
}

func (content *Content) AppendItem(item *Item) *Item {
	content.Items = append(content.Items, item)
	return item
}

func NewISBN() *ISBN {
	isbn := &ISBN{
		Channels: make(map[ChannelType]*Channel),
	}

	isbn.NewChannel(ChannelTypeImportOnlineGame)
	isbn.NewChannel(ChannelTypeImportElectronicGame)
	isbn.NewChannel(ChannelTypeMadeInChinaOnlineGame)
	isbn.NewChannel(ChannelTypeChange)
	isbn.NewChannel(ChannelTypeRevoke)
	return isbn
}

func (isbn *ISBN) NewChannel(channelType ChannelType) *Channel {
	channel := &Channel{
		Type:     channelType,
		Contents: make(map[string]*Content),
	}
	isbn.Channels[channel.Type] = channel
	return channel
}

func (isbn *ISBN) Get() error {
	for _, channel := range isbn.Channels {
		if err := isbn.GetChannel(channel); err != nil {
			return err
		}
	}
	return nil
}

func (isbn *ISBN) SaveToJsonFile(filePath string) error {
	jsonBytes, err := json.MarshalIndent(isbn, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, jsonBytes, 0666)
}

func (isbn *ISBN) getChannelPageUrl(channel *Channel, page int) string {
	pageSuffix := ""
	if page > 1 {
		pageSuffix = fmt.Sprintf("_%d", page)
	}
	return fmt.Sprintf("https://www.nppa.gov.cn/nppa/channels/%s%s.shtml", ChannelTypeUrlNumber[channel.Type], pageSuffix)
}

func (isbn *ISBN) GetChannelPage(channel *Channel, page int) error {
	url := isbn.getChannelPageUrl(channel, page)
	res, err := http.Get(url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return ErrInvalidChannelPageUrl
	}

	html, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	dom := soup.HTMLParse(string(html))
	if dom.Error != nil {
		return dom.Error
	}

	divs := dom.FindAllStrict("div", "class", "ellipsis")
	for _, div := range divs {
		a := div.Find("a")
		href := strings.Trim(a.Attrs()["href"], " ")
		contentUrl := fmt.Sprintf("https://www.nppa.gov.cn%s", href)
		contentTitle := strings.Trim(a.Text(), " ")
		contentId := path.Base(strings.Trim(href, ".shtml"))
		content := &Content{
			ChannelType: channel.Type,
			Url:         contentUrl,
			Id:          contentId,
			Title:       contentTitle,
		}
		channel.Contents[content.Id] = content
	}
	return nil
}

func (isbn *ISBN) GetChannel(channel *Channel) error {
	fmt.Println(channel.Type, "获取中...")
	for page := 1; page < 20; page++ {
		err := isbn.GetChannelPage(channel, page)
		if err != nil {
			if page > 1 && err == ErrInvalidChannelPageUrl {
				break
			}
			return err
		}
	}

	for _, content := range channel.Contents {
		fmt.Println(content.Title, "获取中...")
		if err := isbn.GetChannelContent(content); err != nil {
			return err
		}
	}
	return nil
}

func (isbn *ISBN) GetChannelContent(content *Content) error {
	switch content.ChannelType {
	case ChannelTypeImportOnlineGame:
		return isbn.GetImportOnlineGameChannelContent(content)
	case ChannelTypeImportElectronicGame:
		return isbn.GetImportElectronicGameChannelContent(content)
	case ChannelTypeMadeInChinaOnlineGame:
		return isbn.GetMadeInChinaOnlineGameChannelContent(content)
	case ChannelTypeChange:
		return isbn.GetChangeChannelContent(content)
	case ChannelTypeRevoke:
		return isbn.GetRevokeChannelContent(content)
	default:
		return ErrInvalidChannelType
	}
}

func (isbn *ISBN) GetImportOnlineGameChannelContent(content *Content) error {
	res, err := http.Get(content.Url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	html, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	dom := soup.HTMLParse(string(html))
	if dom.Error != nil {
		return dom.Error
	}

	table := dom.FindStrict("table", "class", "trStyle")
	if table.Error != nil {
		return table.Error
	}

	items := table.FindAll("tr")[1:]
	for _, item := range items {
		fields := item.FindAll("td")

		if len(fields) < 7 {
			continue
		}

		item := content.NewAndAppendItem()
		item.Seq = fields[0].Text()
		item.Name = fields[1].Text()
		item.Type = fields[2].Text()
		item.Publisher = fields[3].Text()
		item.Operator = fields[4].Text()
		item.ApprovalNumber = fields[5].Text()
		if len(fields) > 7 {
			item.ISBN = fields[6].Text()
			item.Time = fields[7].Text()
		} else {
			item.Time = fields[6].Text()
		}
	}
	return nil
}

func (isbn *ISBN) GetImportElectronicGameChannelContent(content *Content) error {
	res, err := http.Get(content.Url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	html, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	dom := soup.HTMLParse(string(html))
	if dom.Error != nil {
		return dom.Error
	}

	table := dom.FindStrict("table", "class", "trStyle")
	if table.Error != nil {
		return table.Error
	}

	items := table.FindAll("tr")[1:]
	for _, item := range items {
		fields := item.FindAll("td")

		if len(fields) != 5 {
			continue
		}

		item := content.NewAndAppendItem()
		item.Seq = fields[0].Text()
		item.Name = fields[1].Text()
		item.Publisher = fields[2].Text()
		item.ApprovalNumber = fields[3].Text()
		item.Time = fields[4].Text()
	}
	return nil
}

func (isbn *ISBN) GetMadeInChinaOnlineGameChannelContent(content *Content) error {
	res, err := http.Get(content.Url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	html, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	dom := soup.HTMLParse(string(html))
	if dom.Error != nil {
		return dom.Error
	}

	table := dom.FindStrict("table", "class", "trStyle tableNormal")
	if table.Error != nil {
		return table.Error
	}

	items := table.FindAll("tr", "class", "item")
	for _, item := range items {
		fields := item.FindAll("td")

		if len(fields) < 7 {
			continue
		}

		item := content.NewAndAppendItem()
		item.Seq = fields[0].Text()
		item.Name = fields[1].Text()
		item.Type = fields[2].Text()
		item.Publisher = fields[3].Text()
		item.Operator = fields[4].Text()
		item.ApprovalNumber = fields[5].Text()
		if len(fields) > 7 {
			item.ISBN = fields[6].Text()
			item.Time = fields[7].Text()
		} else {
			item.Time = fields[6].Text()
		}
	}
	return nil
}

func (isbn *ISBN) GetChangeChannelContent(content *Content) error {
	res, err := http.Get(content.Url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	html, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	dom := soup.HTMLParse(string(html))
	if dom.Error != nil {
		return dom.Error
	}

	table := dom.FindStrict("table", "class", "trStyle")
	if table.Error != nil {
		return table.Error
	}

	items := table.FindAll("tr")[1:]
	for _, item := range items {
		fields := item.FindAll("td")

		if len(fields) < 8 {
			continue
		}

		item := content.NewAndAppendItem()
		item.Seq = fields[0].Text()
		item.Name = fields[1].Text()
		item.Type = fields[2].Text()
		item.Publisher = fields[3].Text()
		item.Operator = fields[4].Text()
		item.ChangeInfo = fields[5].Text()
		item.ApprovalNumber = fields[6].Text()
		if len(fields) > 8 {
			item.ISBN = fields[7].Text()
			item.Time = fields[8].Text()
		} else {
			item.Time = fields[7].Text()
		}
	}
	return nil
}

func (isbn *ISBN) GetRevokeChannelContent(content *Content) error {
	res, err := http.Get(content.Url)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	html, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	dom := soup.HTMLParse(string(html))
	if dom.Error != nil {
		return dom.Error
	}

	table := dom.FindStrict("table", "class", "trStyle")
	if table.Error != nil {
		return table.Error
	}

	items := table.FindAll("tr")[1:]
	for _, item := range items {
		fields := item.FindAll("td")

		if len(fields) != 9 {
			continue
		}

		item := content.NewAndAppendItem()
		item.Seq = fields[0].Text()
		item.Name = fields[1].Text()
		item.Type = fields[2].Text()
		item.Publisher = fields[3].Text()
		item.Operator = fields[4].Text()
		item.RevokeInfo = fields[5].Text()
		item.ApprovalNumber = fields[6].Text()
		item.ISBN = fields[7].Text()
		item.Time = fields[8].Text()
	}
	return nil
}

func main() {
	isbn := NewISBN()

	if err := isbn.Get(); err != nil {
		fmt.Println("get error:", err)
		os.Exit(1)
	}

	if err := isbn.SaveToJsonFile("isbn.json"); err != nil {
		fmt.Println("save to json file error:", err)
		os.Exit(1)
	}
}
