package isbn

import (
	"testing"
)

func TestGetPageContents(t *testing.T) {
	contents, err := GetPageContents(0, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) == 0 {
		t.Fatal("contents is empty")
	}
	for _, content := range contents {
		t.Logf("【%s】 %s %s", content.GetChannelChineseName(), content.Title, content.Date)
	}
}

func TestGetPageContentsWithItems(t *testing.T) {
	contents, err := GetPageContents(0, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) == 0 {
		t.Fatal("contents is empty")
	}
	for _, content := range contents {
		t.Logf("【%s】 %s %s (%d)", content.GetChannelChineseName(), content.Title, content.Date, len(content.Items))
		for _, item := range content.Items {
			t.Logf("  %s %s", item.Seq, item.Name)
		}
	}
}
