package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	_ "github.com/mattn/go-sqlite3"
)

// Language ...
type Language struct {
	ID          int `json:"id"`
	Name        string
	EnglishName string `json:"English"`
	ISOName     string `json:"Culture"`
	LeftToRight bool   `json:"IsLeftToRight"`
	PrayerCount int
}

func (l Language) obligatory() string {
	switch l.ID {
	case 1:
		return "Obligatory"
	case 3:
		return ""
	default:
		log.Fatalf("No translation for 'Obligatory' found for %s", l.EnglishName)
	}
	return ""
}

func (l Language) tablets() string {
	switch l.ID {
	case 1:
		return "Tablets"
	case 3:
		return ""
	default:
		log.Fatalf("No translation for 'Tablets' found for %s", l.EnglishName)
	}
	return ""
}

func (l Language) occassional() string {
	switch l.ID {
	case 1:
		return "Occassional"
	case 3:
		return ""
	default:
		log.Fatalf("No translation for 'Occassional' found for %s", l.EnglishName)
	}
	return ""
}

// PrayersResponse ...
type PrayersResponse struct {
	ErrorMessage string
	IsInError    bool
	Version      int
	Prayers      []Prayer
}

// Tag ...
type Tag struct {
	ID   int `json:"Id"`
	Name string
	Kind string
}

const (
	tagKindGeneral     string = "GENERAL"
	tagKindOccassional        = "OCCASSIONAL"
	tagKindTablets            = "TABLETS"
	tagKindObligatory         = "OBLIGATORY"
)

// Prayer ...
type Prayer struct {
	ID           int `json:"Id"`
	AuthorID     int `json:"AuthorId"`
	LanguageID   int `json:"LanguageId"`
	Text         string
	FirstTagName string `json:"FirstTagName"`
	Tags         []Tag
	Title        string
	category     string
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	langIDToScrape := flag.Int("language", 0, "Language to scrape")
	flag.Parse()

	if *langIDToScrape < 1 {
		log.Fatal("You need to specify a valid language id")
	}

	fmt.Printf("Looking up language...")
	lang, err := lookUpLanguage(*langIDToScrape)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf(" DONE!\n")

	fmt.Printf("Retrieving prayers...")
	pr, err := prayersForLanguage(*langIDToScrape)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf(" DONE!\n")

	categorize(pr, *lang)

	categories := make(map[string]int)
	for _, p := range pr.Prayers {
		count := categories[p.category]
		count++
		categories[p.category] = count
	}

	for category, count := range categories {
		fmt.Printf("%s: %d\n", category, count)
	}

	// err = populateDatabase(*pr, *lang)
	// if err != nil {
	// 	log.Fatal(err)
	// }
}

func populateDatabase(pr PrayersResponse, lang Language) error {
	db, err := sql.Open("sqlite3", lang.ISOName+".db")
	if err != nil {
		return err
	}
	defer db.Close()

	const createTableSQL = `CREATE TABLE prayers (id INTEGER PRIMARY KEY, category TEXT NOT NULL, prayerText TEXT NOT NULL, openingWords TEXT NOT NULL, citation TEXT NOT NULL, author TEXT NOT NULL, language TEXT NOT NULL)`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return err
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// for _, prayer := range pr.Prayers {
	//     const insertSQL = `INSERT INTO prayers (id, category, prayerText, openingWords, citation, author, language)`
	//     db.Exec(insertSQL, prayer.ID, prayer.Category, prayer.Text, )
	// }

	return tx.Commit()
}

func categorize(pr *PrayersResponse, lang Language) {
	// kinds := make(map[string]int)
	for i := range pr.Prayers {
		prayer := &pr.Prayers[i]
		tag := prayer.Tags[0]
		switch tag.Kind {
		case tagKindGeneral:
			prayer.category = tag.Name
		case tagKindObligatory:
			prayer.category = lang.obligatory()
		case tagKindOccassional:
			prayer.category = lang.occassional()
		case tagKindTablets:
			prayer.category = lang.tablets()
		default:
			log.Fatalf("Unknown tag kind - %v", tag.Kind)
		}
	}
}

func prayersForLanguage(id int) (*PrayersResponse, error) {
	urlStr := fmt.Sprintf("https://bahaiprayers.net/api/prayer/prayersystembylanguage?html=false&languageid=%d", id)
	resp, err := http.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Error retrieving prayers. HTTP code %d", resp.StatusCode)
		if buf, err := ioutil.ReadAll(resp.Body); err != nil {
			log.Fatal(err)
		} else {
			log.Fatal(string(buf))
		}
	}

	dec := json.NewDecoder(resp.Body)
	pr := PrayersResponse{}
	err = dec.Decode(&pr)
	if err != nil {
		log.Fatalf("Error parsing prayers response: %v", err)
	}

	return &pr, nil
}

func lookUpLanguage(id int) (*Language, error) {
	resp, err := http.Get("https://bahaiprayers.net/api/prayer/languages")
	if err != nil {
		log.Fatalf("Unable to look up language: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("http code %d - %v", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("http code %d - %s", resp.StatusCode, string(buf))
	}

	dec := json.NewDecoder(resp.Body)
	var langs []Language
	err = dec.Decode(&langs)
	if err != nil {
		log.Fatalf("Error parsing languages response: %v", err)
	}

	for _, l := range langs {
		if l.ID == id {
			return &l, nil
		}
	}

	return nil, fmt.Errorf("language %d not found", id)
}
