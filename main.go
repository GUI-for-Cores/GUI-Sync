package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

var (
	SavePath = "data"
	Addrss   = "0.0.0.0"
	Port     = 8080
	Token    = ""
	Cert     = ""
	Key      = ""
)

type BackupEntry struct {
	Id    string            `json:"id"`
	Tag   string            `json:"tag"`
	Files map[string]string `json:"files"`
}

func main() {
	flag.StringVar(&Token, "token", "", "Authorization")
	flag.StringVar(&Addrss, "address", "0.0.0.0", "Address to listen on")
	flag.StringVar(&Cert, "cert", "", "Cert file path")
	flag.StringVar(&Key, "key", "", "Key file path")
	flag.IntVar(&Port, "port", 8080, "Port to listen on")
	flag.Parse()

	if Token == "" {
		fmt.Println("You need to specify a token that is the same as the client")
		return
	}

	os.MkdirAll(SavePath, os.ModePerm)

	http.HandleFunc("/backup", withAuth(handleBackup))
	http.HandleFunc("/sync", withAuth(handleSync))

	var err error
	if Cert != "" && Key != "" {
		err = http.ListenAndServeTLS(fmt.Sprintf("%s:%d", Addrss, Port), Cert, Key, nil)
	} else {
		err = http.ListenAndServe(fmt.Sprintf("%s:%d", Addrss, Port), nil)
	}
	if err != nil {
		log.Print(err.Error())
	}
}

func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		uaHeader := r.Header.Get("User-Agent")
		if uaHeader != "GUI.for.Cores" || authHeader != "Bearer "+Token {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleBackup(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tag := r.URL.Query().Get("tag")
		p := path.Join(SavePath, tag)

		log.Printf("List => where tag = %s\n", tag)

		if !strings.HasPrefix(path.Clean(p), SavePath) {
			http.Error(w, "403", http.StatusForbidden)
			return
		}

		dirs, err := os.ReadDir(p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		result := make([]string, 0)
		for _, file := range dirs {
			if !file.IsDir() {
				result = append(result, file.Name())
			}
		}

		response, err := json.Marshal(result)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response)

	case http.MethodDelete:
		tag := r.URL.Query().Get("tag")
		ids := r.URL.Query().Get("ids")

		p := path.Join(SavePath, tag)
		idsToDelete := strings.Split(ids, ",")

		log.Printf("Remove => where tag = %s and id in %s\n", tag, ids)

		if !strings.HasPrefix(path.Clean(p), SavePath) {
			http.Error(w, "403", http.StatusForbidden)
			return
		}

		for _, id := range idsToDelete {
			os.RemoveAll(path.Join(p, id))
		}

		w.WriteHeader(http.StatusOK)

	case http.MethodPost:
		var body BackupEntry
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		p := path.Join(SavePath, body.Tag, body.Id)

		log.Printf("Backup : id = %s, tag = %s, files.length = %v\n", body.Id, body.Tag, len(body.Files))

		if !strings.HasPrefix(path.Clean(p), SavePath) {
			http.Error(w, "403", http.StatusForbidden)
			return
		}

		b, err := json.Marshal(body)
		if err != nil {
			log.Printf("Backup err %v", err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		os.MkdirAll(path.Join(SavePath, body.Tag), os.ModePerm)

		os.WriteFile(p+".json", b, 0644)

		w.WriteHeader(http.StatusCreated)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleSync(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tag := r.URL.Query().Get("tag")
		id := r.URL.Query().Get("id")

		p := path.Join(SavePath, tag, id)

		if !strings.HasPrefix(p, SavePath) {
			http.Error(w, "403", http.StatusForbidden)
			return
		}

		b, err := os.ReadFile(p)
		if err != nil {
			http.Error(w, "403", http.StatusForbidden)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(b)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
