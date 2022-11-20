package isbn

import (
	"database/sql"
)

type Database struct {
	DN  string
	DSN string
	db  *sql.DB
}

func (database *Database) getdb() (*sql.DB, error) {
	if database.db == nil {
		db, err := sql.Open(database.DN, database.DSN)
		if err != nil {
			return nil, err
		}
		database.db = db
	}
	return database.db, nil
}

func (database *Database) Close() {
	if database.db != nil {
		database.db.Close()
		database.db = nil
	}
}

func (database *Database) Migrate() error {
	db, err := database.getdb()
	if err != nil {
		return err
	}

	if _, err = db.Exec("CREATE TABLE IF NOT EXISTS content (ChannelID CHAR(8) NOT NULL, ID CHAR(8) NOT NULL, Title VARCHAR(64) NOT NULL, URL VARCHAR(256) NOT NULL, Date CHAR(16) NOT NULL, PRIMARY KEY (ChannelID,ID))"); err != nil {
		return err
	}
	if _, err = db.Exec("CREATE TABLE IF NOT EXISTS item (ChannelID CHAR(8) NOT NULL, ContentID CHAR(8) NOT NULL, Seq CHAR(8) NOT NULL, Name VARCHAR(256) NOT NULL, Catalog VARCHAR(256), Publisher VARCHAR(256), Operator VARCHAR(256), ApprovalNumber VARCHAR(256) NOT NULL, ISBN VARCHAR(256), ChangeInfo VARCHAR(256), RevokeInfo VARCHAR(256), Date CHAR(16) NOT NULL, PRIMARY KEY (ChannelID,ContentID,Seq))"); err != nil {
		return err
	}
	return nil
}

func (database *Database) HasContent(content *Content) (bool, error) {
	db, err := database.getdb()
	if err != nil {
		return false, err
	}

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
	db, err := database.getdb()
	if err != nil {
		return err
	}

	for _, item := range content.Items {
		if has, err := database.HasItem(item); err != nil {
			return err
		} else if has {
			continue
		} else {
			err = database.AddItem(item)
			if err != nil {
				return err
			}
		}
	}

	if _, err := db.Exec("INSERT INTO content(ChannelID, ID, Title, URL, Date) VALUES(?,?,?,?,?)", content.ChannelID, content.ID, content.Title, content.URL, content.Date); err != nil {
		return err
	}
	return nil
}

func (database *Database) HasItem(item *Item) (bool, error) {
	db, err := database.getdb()
	if err != nil {
		return false, err
	}

	rows, err := db.Query("SELECT ChannelID, ContentID, Seq FROM item WHERE ChannelID = ? AND ContentID = ? AND Seq = ?", item.ChannelID, item.ContentID, item.Seq)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	for rows.Next() {
		return true, nil
	}
	return false, nil
}

func (database *Database) AddItem(item *Item) error {
	db, err := database.getdb()
	if err != nil {
		return err
	}

	if _, err := db.Exec("INSERT INTO item(ChannelID, ContentID, Seq, Name, Catalog, Publisher, Operator, ApprovalNumber, ISBN, ChangeInfo, RevokeInfo, Date) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)",
		item.ChannelID, item.ContentID, item.Seq, item.Name, item.Catalog, item.Publisher, item.Operator, item.ApprovalNumber, item.ISBN, item.ChangeInfo, item.RevokeInfo, item.Date); err != nil {
		return err
	}
	return nil
}
