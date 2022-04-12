package main

import (
	"fmt"
	"github.com/anaskhan96/soup"
	"io/ioutil"
	"net/http"
	"os"
)

func main() {
	res, err := http.Get("https://www.nppa.gov.cn/nppa/contents/320/103799.shtml")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	defer res.Body.Close()

	html, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	dom := soup.HTMLParse(string(html))
	if dom.Error != nil {
		fmt.Println(dom.Error)
		os.Exit(1)
	}

	table := dom.FindStrict("table", "class", "trStyle tableNormal")
	if table.Error != nil {
		fmt.Println(table.Error)
		os.Exit(1)
	}

	items := table.FindAll("tr", "class", "item")
	for _, item := range items {
		itemFields := item.FindAll("td")
		for _, itemField := range itemFields {
			fmt.Print(itemField.Text(), " ")
		}
		fmt.Println("")
	}
}
