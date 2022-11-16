package isbn

import (
	"database/sql"
)

type Database struct {
	DN  string
	DSN string
}

func (database *Database) Migrate() error {
	db, err := sql.Open(database.DN, database.DSN)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err = db.Exec("CREATE TABLE IF NOT EXISTS content (ChannelID CHAR(8) NOT NULL, ID CHAR(8) NOT NULL, Title VARCHAR(64) NOT NULL, URL VARCHAR(256) NOT NULL, Date CHAR(16) NOT NULL, PRIMARY KEY (ChannelID,ID))"); err != nil {
		return err
	}
	if _, err = db.Exec("CREATE TABLE IF NOT EXISTS item (ChannelID CHAR(8) NOT NULL, ContentID CHAR(8) NOT NULL, Seq CHAR(8) NOT NULL, Name VARCHAR(256) NOT NULL, Catalog VARCHAR(256), Publisher VARCHAR(256), Operator VARCHAR(256), ApprovalNumber VARCHAR(256) NOT NULL, ISBN VARCHAR(256), ChangeInfo VARCHAR(256), RevokeInfo VARCHAR(256), Date CHAR(16) NOT NULL, PRIMARY KEY (ChannelID,ContentID,Seq))"); err != nil {
		return err
	}
	return nil
}

func (database *Database) HasContent(content *Content) (bool, error) {
	db, err := sql.Open(database.DN, database.DSN)
	if err != nil {
		return false, err
	}
	defer db.Close()

	rows, err := db.Query("SELECT ChannelID, ID FROM content WHERE ChannelID = ? AND ID = ?", content.ChannelID, content.ID)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		return true, nil
	}
	return false, nil
}

func (database *Database) AddContent(content *Content) error {
	db, err := sql.Open(database.DN, database.DSN)
	if err != nil {
		return err
	}
	defer db.Close()

	for _, item := range content.Items {
		_, err := db.Exec("INSERT INTO item(ChannelID, ContentID, Seq, Name, Catalog, Publisher, Operator, ApprovalNumber, ISBN, ChangeInfo, RevokeInfo, Date) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)",
			item.ChannelID, item.ContentID, item.Seq, item.Name, item.Catalog, item.Publisher, item.Operator, item.ApprovalNumber, item.ISBN, item.ChangeInfo, item.RevokeInfo, item.Date)
		if err != nil {
			return err
		}
	}

	if _, err := db.Exec("INSERt INTO content(ChannelID, ID, Title, URL, Date) VALUES(?,?,?,?,?)", content.ChannelID, content.ID, content.Title, content.URL, content.Date); err != nil {
		return err
	}
	return nil
}
