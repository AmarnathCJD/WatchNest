package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
)

func createNewDB() string {
	req, _ := http.NewRequest("POST", "https://jsonblob.com/api/jsonBlob", bytes.NewBuffer([]byte(`{}`)))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	res, _ := http.DefaultClient.Do(req)

	return res.Header.Get("Location")
}

func getDB(dbURL string) (WatchList, error) {
	req, _ := http.NewRequest("GET", dbURL, nil)
	req.Header.Add("Accept", "application/json")

	res, _ := http.DefaultClient.Do(req)

	var db WatchList
	if err := json.NewDecoder(res.Body).Decode(&db); err != nil {
		return WatchList{}, err
	}

	return db, nil
}

func updateDB(baseURL string, keys WatchList) error {
	body, err := json.Marshal(keys)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PUT", baseURL, nil)
	if err != nil {
		log.Println(err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
	req.Body = io.NopCloser(bytes.NewReader(body))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		return err
	}
	defer resp.Body.Close()
	return nil
}

type Entry struct {
	Title string `json:"title"`
	ID    string `json:"id"`
	Done  bool   `json:"done"`
	Prio  int    `json:"prio"`
}

type WatchList struct {
	Entries []Entry `json:"entries"`
}

type Query struct {
	contents map[string]string
}

func (q *Query) get(key string) string {
	if value, ok := q.contents[key]; ok {
		return value
	}
	return ""
}

func (q *Query) getBool(key string) bool {
	return q.get(key) == "true"
}

func (q *Query) getInt(key string) int {
	value := q.get(key)
	if value == "" {
		return 0
	}

	var i int
	fmt.Sscanf(value, "%d", &i)
	return i
}

func extractQueryBody(r *http.Request) (*Query, error) {
	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			return nil, err
		}
	} else if r.Method == "GET" {
		var body = make(map[string]string)
		for key, value := range r.URL.Query() {
			body[key] = value[0]
		}
		return &Query{contents: body}, nil
	}

	var body = make(map[string]string)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, err
	}

	return &Query{contents: body}, nil
}

func WatchListModif(w http.ResponseWriter, r *http.Request) {
	query, err := extractQueryBody(r)
	if err != nil || query == nil {
		http.Error(w, `{"error": "could not parse request body"}`, http.StatusBadRequest)
		return
	}

	db := query.get("db")

	if query.get("action") == "add" {
		if db == "" {
			http.Error(w, `{"error": "no db specified, create a new one first"}`, http.StatusBadRequest)
			return
		}

		if !strings.Contains(db, "https://jsonblob.com/api/jsonBlob/") {
			db = "https://jsonblob.com/api/jsonBlob/" + db
		}

		entry := Entry{
			Title: query.get("title"),
			ID:    query.get("id"),
			Done:  query.getBool("done"),
			Prio:  query.getInt("prio"),
		}

		watchList, err := getDB(db)
		if err != nil {
			http.Error(w, `{"error": "could not fetch db"}`, http.StatusInternalServerError)
			return
		}

		for _, e := range watchList.Entries {
			if e.ID == entry.ID {
				http.Error(w, `{"error": "entry with id already exists"}`, http.StatusBadRequest)
				return
			}
		}

		watchList.Entries = append(watchList.Entries, entry)
		if err := updateDB(db, watchList); err != nil {
			http.Error(w, `{"error": "could not update db"}`, http.StatusInternalServerError)
			return
		}

		w.Write([]byte(`{"status": "ok"}`))
	} else if query.get("action") == "remove" {
		if db == "" {
			http.Error(w, `{"error": "no db specified"}`, http.StatusBadRequest)
			return
		}

		id := query.get("id")
		if id == "" {
			http.Error(w, `{"error": "no id specified"}`, http.StatusBadRequest)
			return
		}

		watchList, err := getDB(db)
		if err != nil {
			http.Error(w, `{"error": "could not fetch db"}`, http.StatusInternalServerError)
			return
		}

		for i, e := range watchList.Entries {
			if e.ID == id {
				watchList.Entries = append(watchList.Entries[:i], watchList.Entries[i+1:]...)
				if err := updateDB(db, watchList); err != nil {
					http.Error(w, `{"error": "could not update db"}`, http.StatusInternalServerError)
					return
				}

				w.WriteHeader(http.StatusOK)
				return
			}
		}

		http.Error(w, `{"error": "entry with id not found"}`, http.StatusBadRequest)
	} else if query.get("action") == "new" {
		dbURL := createNewDB()
		fmt.Fprintln(w, dbURL)
	} else {
		http.Error(w, `{"error": "invalid action"}`, http.StatusBadRequest)
	}
}

func main() {
	http.HandleFunc("/watchlist", WatchListModif)
	http.ListenAndServe(":8080", nil)
}
