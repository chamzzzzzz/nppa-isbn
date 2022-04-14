package main

import (
	"errors"
	"fmt"
	"github.com/anaskhan96/soup"
	"io/ioutil"
	"net/http"
	"os"
)

type ChannelType string

const (
	ChannelTypeImportOnlineGame      = "318"
	ChannelTypeImportElectronicGame  = "319"
	ChannelTypeMadeInChinaOnlineGame = "320"
	ChannelTypeChange                = "321"
	ChannelTypeRevoke                = "474"
)

var (
	ErrInvalidChannelType = errors.New("invalid channel type")
)

type Channel struct {
	Type     ChannelType
	Url      string
	Contents []*Content
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

	content := &Content{
		ChannelType: ChannelTypeMadeInChinaOnlineGame,
		Id:          "103799",
		Url:         "https://www.nppa.gov.cn/nppa/contents/320/103799.shtml",
	}

	err := isbn.GetChannelContent(content)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, item := range content.Items {
		fmt.Println(item)
	}

	content.Id = "76768"
	content.Url = "https://www.nppa.gov.cn/nppa/contents/320/76768.shtml"
	err = isbn.GetChannelContent(content)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	for _, item := range content.Items {
		fmt.Println(item)
	}
}
