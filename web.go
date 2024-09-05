package main

import (
	"database/sql"
	"embed"
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"net/http"
	"time"
)

//go:embed listing.html
var listingHtml string

//go:embed static/*
var staticAssets embed.FS

type RawListingQuery struct {
	Start string
	End   string
}

type ValidatedListingQuery struct {
	Start time.Time
	End   time.Time
}

func (r RawListingQuery) isEmpty() bool {
	return r.Start == "" && r.End == ""
}

func (r RawListingQuery) Validate() (ValidatedListingQuery, error) {
	ISO8601 := "2006-01-02T15:04"
	start, err := time.Parse(ISO8601, r.Start)
	start = ForceLocalTime(start)
	if err != nil {
		return ValidatedListingQuery{}, err
	}
	end, err := time.Parse(ISO8601, r.End)
	end = ForceLocalTime(end)
	if err != nil {
		return ValidatedListingQuery{}, err
	}
	return ValidatedListingQuery{start, end}, nil
}

func parseRawGetQuery(r *http.Request) (RawListingQuery, error) {
	ret := RawListingQuery{}
	query := r.URL.Query()
	ret.Start = query.Get("start")
	ret.End = query.Get("end")
	log.Info(ret)
	return ret, nil
}

func listLatest(db *sql.DB) (*sql.Rows, error) {
	return db.Query("SELECT id, timestamp, unixepoch(timestamp, 'subsec') as epoch, msg FROM datalog order by timestamp desc LIMIT 10000")

}

func ForceLocalTime(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.Local)
}

func listSpecificInterval(db *sql.DB, startTime time.Time, endTime time.Time) (*sql.Rows, error) {
	start := startTime.Format("2006-01-02 15:04:05")
	end := endTime.Format("2006-01-02 15:04:05")
	log.Info(start)
	log.Info(end)
	return db.Query("SELECT id, timestamp, unixepoch(timestamp, 'subsec') as epoch, msg  FROM datalog WHERE timestamp between datetime(?) and datetime(?) order by timestamp desc LIMIT 1000000", start, end)
}

type Data struct {
	ID        int     `json:"id"`
	Timestamp string  `json:"timestamp"`
	Epoch     float64 `json:"epoch"`
	Msg       string  `json:"msg"`
}

func rowsToData(rows *sql.Rows) ([]Data, error) {
	var allData = []Data{}
	for rows.Next() {
		d := Data{}
		err := rows.Scan(&d.ID, &d.Timestamp, &d.Epoch, &d.Msg)
		if err != nil {
			return nil, err
		}
		allData = append(allData, d)
	}

	return allData, nil
}

func listingHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rawQuery, err := parseRawGetQuery(r)
		if err != nil {
			http.Error(w, "Failed to parse the query", http.StatusBadRequest)
			return
		}
		var rows *sql.Rows
		if rawQuery.isEmpty() {
			rows, err = listLatest(db)
		} else {
			validatedQuery, err := rawQuery.Validate()
			if err != nil {
				http.Error(w, "Failed to validate the query", http.StatusBadRequest)
				return
			}
			rows, err = listSpecificInterval(db, validatedQuery.Start, validatedQuery.End)
		}
		if err != nil {
			log.Error(err)
			return
		}
		allData, err := rowsToData(rows)
		if err != nil {
			log.Error("Failed to convert the result to Array")
			http.Error(w, "Failed to convert the result to Array", http.StatusInternalServerError)
			return
		}
		jsonBytes, err := json.Marshal(allData)
		if err != nil {
			http.Error(w, "Failed to convert the result to JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write(jsonBytes)
		if err != nil {
			log.Errorf("Failed to write the response: %v\n", err)
		}
	}
}
func ServeStaticString(s string, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, err := w.Write([]byte(s))
		if err != nil {
			log.Error(err)
		}
	}
}

func assetFileServer() (*http.Handler, error) {
	sub, err := fs.Sub(staticAssets, "static")
	if err != nil {
		log.Errorf("Failed to get the sub directory: %v\n", err)
		return nil, err
	}
	server := http.FileServer(http.FS(sub))
	return &server, nil
}

func RunWebServer(db *sql.DB) {
	fileServer, err := assetFileServer()
	if err != nil {
		log.Errorf("Failed to create the file server: %v\n", err)
		return
	}
	http.Handle("/static/", http.StripPrefix("/static", *fileServer))
	http.Handle("/", ServeStaticString(listingHtml, "text/html"))
	http.HandleFunc("/list", listingHandler(db))
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Errorf("Failed to start the server: %v\n", err)
		return
	}
}
