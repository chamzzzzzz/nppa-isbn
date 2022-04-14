package main

import (
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
	ChannelTypeImportOnlineGame      = "318"
	ChannelTypeImportElectronicGame  = "319"
	ChannelTypeMadeInChinaOnlineGame = "320"
	ChannelTypeChange                = "321"
	ChannelTypeRevoke                = "747"
)

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
}

type ISBN struct {
	Channels map[string]*Channel
}

func NewISBN() *ISBN {
	return &ISBN{
		Channels: make(map[string]*Channel),
	}
}

func NewChannel(channelType ChannelType) *Channel {
	return &Channel{
		Type:     channelType,
		Contents: make(map[string]*Content),
	}
}

func (isbn *ISBN) getChannelPageUrl(channel *Channel, page int) string {
	pageSuffix := ""
	if page > 1 {
		pageSuffix = fmt.Sprintf("_%d", page)
	}
	return fmt.Sprintf("https://www.nppa.gov.cn/nppa/channels/%s%s.shtml", channel.Type, pageSuffix)
}

func (isbn *ISBN) GetChannelPage(channel *Channel, page int) error {
	url := isbn.getChannelPageUrl(channel, page)
	fmt.Println("getting", url)
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
		href := a.Attrs()["href"]
		contentUrl := fmt.Sprintf("https://www.nppa.gov.cn%s", href)
		contentTitle := a.Text()
		contentId := path.Base(strings.Trim(href, ".shtml"))
		content := &Content{
			ChannelType: channel.Type,
			Url:         contentUrl,
			Id:          contentId,
			Title:       contentTitle,
		}
		channel.Contents[content.Id] = content
		fmt.Println(content)
	}
	return nil
}

func (isbn *ISBN) GetChannel(channel *Channel) error {
	for page := 1; page < 100; page++ {
		err := isbn.GetChannelPage(channel, page)
		if err != nil {
			if page > 1 && err == ErrInvalidChannelPageUrl {
				return nil
			}
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
	return nil
}

func (isbn *ISBN) GetImportElectronicGameChannelContent(content *Content) error {
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

		seq := fields[0].Text()
		name := fields[1].Text()
		_type := fields[2].Text()
		publisher := fields[3].Text()
		operator := fields[4].Text()
		approvalNumber := fields[5].Text()
		isbn := ""
		_time := ""

		if len(fields) > 7 {
			isbn = fields[6].Text()
			_time = fields[7].Text()
		} else {
			_time = fields[6].Text()
		}

		item := &Item{content.ChannelType, content.Id, seq, name, _type, publisher, operator, approvalNumber, isbn, _time}
		content.Items = append(content.Items, item)
	}
	return nil
}

func (isbn *ISBN) GetChangeChannelContent(content *Content) error {
	return nil
}

func (isbn *ISBN) GetRevokeChannelContent(content *Content) error {
	return nil
}

func main() {
	isbn := NewISBN()

	channel := NewChannel(ChannelTypeMadeInChinaOnlineGame)
	err := isbn.GetChannel(channel)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, content := range channel.Contents {
		fmt.Println(content)
		if err := isbn.GetChannelContent(content); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		for _, item := range content.Items {
			fmt.Println(item)
		}
	}
}
