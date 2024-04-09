package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	backupIndexFile = "backup_index.json"
	dataDir         = "data"
	backupIndex     []BackupEntry
	addrss          = "0.0.0.0"
	port            = 8080
	token           = ""
)

type BackupEntry struct {
	Id    string   `json:"id"`
	Files []string `json:"files"`
}

func main() {

	flag.StringVar(&token, "token", "", "Authorization")
	flag.StringVar(&addrss, "address", "0.0.0.0", "Address to listen on")
	flag.IntVar(&port, "port", 8080, "Port to listen on")
	flag.Parse()

	if token == "" {
		fmt.Println("You need to specify a token that is the same as the client")
		return
	}

	loadBackupIndex()

	http.HandleFunc("/backup", withAuth(handleBackup))
	http.HandleFunc("/file", withAuth(handleFile))
	http.ListenAndServe(fmt.Sprintf("%s:%d", addrss, port), nil)
}

func loadBackupIndex() {
	file, err := os.ReadFile(backupIndexFile)
	if err != nil {
		log.Printf("Error reading backup index file: %v\n", err)
		return
	}

	err = json.Unmarshal(file, &backupIndex)
	if err != nil {
		log.Printf("Error decoding backup index JSON: %v\n", err)
		return
	}
}

func withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		uaHeader := r.Header.Get("User-Agent")
		if uaHeader != "GUI.for.Cores" || authHeader != "Bearer "+token {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func handleBackup(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		idParam := r.URL.Query().Get("id")

		log.Printf("List => %s\n", idParam)

		var response []byte
		result := make([]string, 0)

		if idParam != "" {
			for _, entry := range backupIndex {
				if entry.Id == idParam {
					result = entry.Files
					break
				}
			}

			var err error
			response, err = json.Marshal(result)
			if err != nil {
				http.Error(w, "Failed to marshal JSON response", http.StatusInternalServerError)
				return
			}
		} else {
			for _, entry := range backupIndex {
				result = append(result, entry.Id)
			}

			var err error
			response, err = json.Marshal(result)
			if err != nil {
				http.Error(w, "Failed to marshal JSON response", http.StatusInternalServerError)
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(response)

	case http.MethodDelete:
		ids := r.URL.Query().Get("ids")

		if ids == "" {
			http.Error(w, "Error parameter", http.StatusBadRequest)
			return
		}

		idsToDelete := strings.Split(ids, ",")

		log.Printf("Remove => %s\n", ids)

		updatedBackupIndex := make([]BackupEntry, 0)

		for _, entry := range backupIndex {
			found := false
			for _, id := range idsToDelete {
				if entry.Id == id {
					found = true
					break
				}
			}
			if !found {
				updatedBackupIndex = append(updatedBackupIndex, entry)
			} else {
				filePath := filepath.Join(dataDir, entry.Id)
				os.RemoveAll(filePath)
			}
		}

		backupIndex = updatedBackupIndex

		updateBackupIndexFile()

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleFile(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		pathParam := r.URL.Query().Get("path")

		_path := path.Clean(path.Join(dataDir, pathParam))
		if !strings.HasPrefix(_path, dataDir) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		log.Printf("Sync : %v\n", pathParam)

		body, err := os.ReadFile(_path)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write(body)

	case http.MethodPost:
		var requestBody map[string]string
		err := json.NewDecoder(r.Body).Decode(&requestBody)
		if err != nil {
			http.Error(w, "Failed to parse request body", http.StatusBadRequest)
			return
		}

		id := requestBody["id"]
		filename := requestBody["file"]
		fileContent := requestBody["body"]

		log.Printf("Backup : %v => %v\n", id, filename)

		filePath := filepath.Join(dataDir, id, filename)
		err = os.MkdirAll(filepath.Dir(filePath), 0755)
		if err != nil {
			http.Error(w, "Failed to create directory", http.StatusInternalServerError)
			return
		}

		err = os.WriteFile(filePath, []byte(fileContent), 0644)
		if err != nil {
			http.Error(w, "Failed to write file", http.StatusInternalServerError)
			return
		}

		updateBackupIndex(id, filename)

		w.WriteHeader(http.StatusCreated)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func updateBackupIndex(id, filename string) {
	var found bool
	for i, entry := range backupIndex {
		if entry.Id == id {
			// Check if file already exists in the list
			found = true
			foundFile := false
			for _, file := range entry.Files {
				if file == filename {
					foundFile = true
					break
				}
			}
			// If file not found, append to the list
			if !foundFile {
				backupIndex[i].Files = append(backupIndex[i].Files, filename)
			}
			break
		}
	}
	if !found {
		backupIndex = append(backupIndex, BackupEntry{
			Id:    id,
			Files: []string{filename},
		})
	}

	updateBackupIndexFile()
}

func updateBackupIndexFile() {
	// Update backup index file
	jsonData, err := json.MarshalIndent(backupIndex, "", "    ")
	if err != nil {
		log.Printf("Error marshaling backup index JSON: %v\n", err)
		return
	}

	err = os.WriteFile(backupIndexFile, jsonData, 0644)
	if err != nil {
		log.Printf("Error writing backup index file: %v\n", err)
		return
	}
}
