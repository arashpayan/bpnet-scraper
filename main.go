package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// Language ids
const (
	English    int = 1
	Icelandic      = 2
	German         = 3
	Spanish        = 4
	Persian        = 5
	Arabic         = 6
	French         = 7
	Portuguese     = 8
	Chinese        = 9
	Italian        = 10
	Dutch          = 11
	Romanian       = 12
	Latvian        = 13
	Belarusian     = 14
	Russian        = 15
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
	case English:
		return "Obligatory"
	case German:
		return "Pflichtgebet"
	case Russian:
		return "Oбязательная" // TODO
	default:
		log.Fatalf("No translation for 'Obligatory' found for %s", l.EnglishName)
	}
	return ""
}

func (l Language) tablets() string {
	switch l.ID {
	case English:
		return "Tablets"
	case German:
		return "Tableten"
	case Russian:
		return "" // TODO
	default:
		log.Fatalf("No translation for 'Tablets' found for %s", l.EnglishName)
	}
	return ""
}

func (l Language) occassional() string {
	switch l.ID {
	case English:
		return "Occassional"
	case German:
		return "Besondere Gelegenheiten" // TODO
	case Russian:
		return "случайный" // TODO
	default:
		log.Fatalf("No translation for 'Occassional' found for %s", l.EnglishName)
	}
	return ""
}

func (l Language) theFast() string {
	switch l.ID {
	case English:
		return "The Fast"
	default:
		log.Fatalf("No translation for 'The Fast' found for %s", l.EnglishName)
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
	citation     string
	htmlPrayer   string
	openingWords string
}

// PBPrayer is the format of prayers in the app database
type PBPrayer struct {
	ID           int    `db:"id"`
	Category     string `db:"category"`
	PrayerText   string `db:"prayerText"`
	OpeningWords string `db:"openingWords"`
	Citation     string `db:"citation"`
	Author       string `db:"author"`
	Language     string `db:"language"`
	WordCount    int    `db:"wordCount"`
	SearchText   string `db:"searchText"`
}

type authorIDMap map[int]string

// var languageAuthorMap = make(map[string]authorIDMap)
var languageAuthorMap = map[string]authorIDMap{
	"en": map[int]string{ // English
		1: "The Báb",
		2: "Bahá'u'lláh",
		3: "`Abdu'l-Bahá",
	},
	"es": map[int]string{ // Spanish
		1: "El Báb",
		2: "Bahá'u'lláh",
		3: "`Abdu'l-Bahá",
	},
	"fr": map[int]string{ // French
		1: "Le Bab",
		2: "Bahá'u'lláh",
		3: "`Abdu'l-Bahá",
	},
	"nl": map[int]string{ // Dutch
		1: "de Báb",
		2: "Bahá'u'lláh",
		3: "`Abdu'l-Bahá",
	},
	"is": map[int]string{ // Icelandic
		1: "Bábinn",
		2: "Bahá’u’lláh",
		3: "`Abdu'l-Bahá",
	},
	"fj": map[int]string{ // Fijian
		1: "Na Báb",
		2: "Bahá’u’lláh",
		3: "`Abdu'l-Bahá",
	},
	"cs": map[int]string{ // Czech
		1: "Báb",
		2: "Bahá’u’lláh",
		3: "`Abdu'l-Bahá",
	},
	"sk": map[int]string{ // Slovak
		1: "Báb",
		2: "Bahá’u’lláh",
		3: "`Abdu'l-Bahá",
	},
	"de": map[int]string{ // German
		1: "Báb",
		2: "Bahá’u’lláh",
		3: "`Abdu'l-Bahá",
	},
	"ru": map[int]string{ // Russian
		1: "Баб",
		2: "Бахаулла",
		3: "Абдул-Баха",
	},
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	langIDToScrape := flag.Int("language", 0, "Language to scrape")
	mergeDBsList := flag.String("merge", "", "Comma separated list of db files")
	flag.Parse()

	if *langIDToScrape >= 1 {
		scrapeLanguage(*langIDToScrape)
	} else if *mergeDBsList != "" {
		mergeDBs(*mergeDBsList)
	} else {
		log.Fatal("You need to specify a command")
	}
}

func mergeDBs(dbsCommaSeparated string) {
	dbs := strings.Split(dbsCommaSeparated, ",")

	// delete any old mergings
	os.Remove("merged.db")

	db, err := sql.Open("sqlite3", "merged.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	const createTableSQL = `
	CREATE TABLE prayers (	id INTEGER PRIMARY KEY,
							category TEXT NOT NULL,
							prayerText TEXT NOT NULL,
							openingWords TEXT NOT NULL,
							citation TEXT NOT NULL,
							author TEXT NOT NULL,
							language TEXT NOT NULL,
							wordCount INTEGER NOT NULL,
							searchText TEXT NOT NULL)`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Print("Merging")
	for _, dbPath := range dbs {
		fmt.Print(".")
		mergeDB(dbPath, db)
	}
	fmt.Print(" DONE!\n")

	fmt.Print("Creating indices... ")
	_, err = db.Exec(`CREATE INDEX search_text_index ON prayers (searchText)`)
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(`CREATE INDEX language_index ON prayers (language)`)
	if err != nil {
		log.Fatal(err)
	}
	_, err = db.Exec(`CREATE INDEX category_index on prayers (category)`)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print("DONE!\n")
}

func mergeDB(langDBPath string, mergedDB *sql.DB) {
	langDB, err := sqlx.Open("sqlite3", langDBPath)
	if err != nil {
		log.Fatal(err)
	}
	defer langDB.Close()

	rows, err := langDB.Queryx("SELECT * FROM prayers")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	tx, err := mergedDB.Begin()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	const insertSQL = `INSERT INTO prayers (id, category, prayerText, openingWords, citation, author, language, wordCount, searchText) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	for rows.Next() {
		prayer := PBPrayer{}
		err = rows.StructScan(&prayer)
		if err != nil {
			log.Fatal(err)
		}
		searchText := strings.Replace(prayer.PrayerText, `<p>`, "", -1)
		searchText = strings.Replace(searchText, `</p>`, "", -1)
		searchText = strings.Replace(searchText, `<p class="opening">`, "", -1)
		searchText = strings.Replace(searchText, `<span class="versal">`, "", -1)
		searchText = strings.Replace(searchText, `</span>`, "", -1)
		searchText = strings.Replace(searchText, `<p class="noindent">`, "", -1)
		searchText = strings.Replace(searchText, `<br/>`, "", -1)
		searchText = strings.Replace(searchText, `<i>`, "", -1)
		searchText = strings.Replace(searchText, `</i>`, "", -1)
		searchText = strings.Replace(searchText, `<p class="comment">`, "", -1)
		searchText = strings.Replace(searchText, `<p class="commentcaps">`, "", -1)
		searchText = strings.Replace(searchText, `<em>`, "", -1)
		searchText = strings.Replace(searchText, `</em>`, "", -1)
		prayer.WordCount = len(strings.Fields(searchText))

		prayer.SearchText = searchText

		_, err := tx.Exec(insertSQL, prayer.ID, prayer.Category, prayer.PrayerText, prayer.OpeningWords, prayer.Citation, prayer.Author, prayer.Language, prayer.WordCount, prayer.SearchText)
		if err != nil {
			log.Fatal(err)
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}

func scrapeLanguage(langIDToScrape int) {
	fmt.Printf("Looking up language…")
	lang, err := lookUpLanguage(langIDToScrape)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf(" DONE!\n")

	fmt.Printf("Retrieving prayers…")
	pr, err := prayersForLanguage(langIDToScrape)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf(" DONE!\n")

	categorize(pr, *lang)

	markup(pr)

	// categories := make(map[string]int)
	// for _, p := range pr.Prayers {
	// 	count := categories[p.category]
	// 	count++
	// 	categories[p.category] = count
	// }
	//
	// for category, count := range categories {
	// 	fmt.Printf("%s: %d\n", category, count)
	// }

	fmt.Printf("Populating database…")
	err = populateDatabase(*pr, *lang)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf(" DONE!\n")
}

func populateDatabase(pr PrayersResponse, lang Language) error {
	// delete any old database files that may be around
	os.Remove(lang.ISOName + ".db")

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

	for _, prayer := range pr.Prayers {
		const insertSQL = `INSERT INTO prayers (id, category, prayerText, openingWords, citation, author, language) VALUES (?, ?, ?, ?, ?, ?, ?)`
		openingWords := ""
		if prayer.Title != "" {
			openingWords = prayer.Title
		} else {
			openingWords = prayer.openingWords
		}
		_, err = tx.Exec(insertSQL, prayer.ID, prayer.category, prayer.htmlPrayer, openingWords, prayer.citation, languageAuthorMap[lang.ISOName][prayer.AuthorID], lang.ISOName)
		if err != nil {
			log.Fatal(err)
		}
	}

	return tx.Commit()
}

func markup(pr *PrayersResponse) {
	for i := range pr.Prayers {
		prayer := &pr.Prayers[i]
		// if prayer.ID != 6664 {
		// 	continue
		// }

		parts := strings.FieldsFunc(prayer.Text, func(r rune) bool {
			return r == '\n'
		})
		var cleanedParts []string
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				cleanedParts = append(cleanedParts, trimmed)
				// log.Print(trimmed)
			}
		}

		var markedParts []string
		markedOpening := false
		for i, p := range cleanedParts {
			if strings.HasPrefix(p, "##") {
				markedParts = append(markedParts, `<p class="commentcaps">`+p[2:]+"</p>")
			} else if strings.HasPrefix(p, "#") {
				// log.Printf("Single hash")
				// log.Printf("%d %s", prayer.ID, p)
				prayer.openingWords = p[1:]
			} else if strings.HasPrefix(p, "*") {
				// if this is the last asterisk'ed paragraph, it's a citation
				if i == len(cleanedParts)-1 {
					prayer.citation = p[1:]
					continue
				}
				markedParts = append(markedParts, `<p class="comment">`+p[1:]+"</p>")
			} else {
				if markedOpening {
					markedParts = append(markedParts, "<p>"+p+"</p>")
				} else {
					min := 35
					if len(p) < 35 {
						min = len(p)
					}
					prayer.openingWords = p[:min] + "…"
					marked := `<p class="opening"><span class="versal">` + p[0:1] + `</span>` + p[1:] + "</p>"
					markedParts = append(markedParts, marked)
					markedOpening = true
				}
			}
		}

		htmlPrayer := bytes.Buffer{}
		for i, p := range markedParts {
			htmlPrayer.WriteString(p)
			if i != len(markedParts)-1 {
				htmlPrayer.WriteString("\n\n")
			}
		}
		prayer.htmlPrayer = htmlPrayer.String()
	}
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
			prayer.Title = tag.Name
		case tagKindOccassional:
			prayer.category = lang.occassional()
			prayer.Title = tag.Name
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
