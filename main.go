package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

var (
	SavePath = "data"
	Addrss   = "0.0.0.0"
	Port     = 8080
	Token    = ""
	Cert     = ""
	Key      = ""
	Secret   = ""
)

type BackupEntry struct {
	Id    string            `json:"id"`
	Tag   string            `json:"tag"`
	Files map[string]string `json:"files"`
}

type DeployEntry struct {
	ConfigPath  string `json:"configPath"`
	ServiceName string `json:"serviceName"`
	Content     string `json:"content"`
	Timeout     int    `json:"timeout"`
}

func main() {
	flag.StringVar(&Token, "token", "", "Authorization")
	flag.StringVar(&Addrss, "address", "0.0.0.0", "Address to listen on")
	flag.StringVar(&Cert, "cert", "", "Cert file path")
	flag.StringVar(&Key, "key", "", "Key file path")
	flag.StringVar(&Secret, "secret", "", "Secret")
	flag.IntVar(&Port, "port", 8080, "Port to listen on")
	flag.Parse()

	if Token == "" {
		fmt.Println("You need to specify a token that is the same as the client")
		return
	}

	os.MkdirAll(SavePath, os.ModePerm)

	http.HandleFunc("/backup", withAuth(handleBackup))
	http.HandleFunc("/sync", withAuth(handleSync))
	http.HandleFunc("/deploy", withAuth(handleDeploy))

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

func handleDeploy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var body DeployEntry
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		configPath := body.ConfigPath
		serviceName := body.ServiceName
		content := body.Content
		timeout := body.Timeout

		log.Printf("Deploy : configPath = %s, serviceName = %s\n", configPath, serviceName)

		if Secret == "" {
			http.Error(w, "The secret parameter is missing on the server side", http.StatusInternalServerError)
			return
		}

		config, err := AesDecrypt(content, Secret)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		backup, backupError := os.ReadFile(configPath)
		os.WriteFile(configPath, config, 0644)
		exec.Command("systemctl", "restart", serviceName).Run()

		isTimeout := false

		for {
			time.Sleep(1 * time.Second)
			output, _ := exec.Command("systemctl", "is-active", serviceName).CombinedOutput()
			if strings.TrimSpace(string(output)) == "active" {
				break
			}
			timeout--
			if timeout < 0 {
				isTimeout = true
				break
			}
		}

		if isTimeout && backupError == nil {
			os.WriteFile(body.ConfigPath, backup, 0644)
			exec.Command("systemctl", "restart", body.ServiceName).Run()
			http.Error(w, "Restarting the service failed and has been restored", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func EvpBytesToKey(password []byte, salt []byte, keyLen, ivLen int) ([]byte, []byte) {
	var result []byte
	hash := md5.New()
	var prev []byte
	totalLen := keyLen + ivLen

	for len(result) < totalLen {
		hash.Reset()
		hash.Write(prev)
		hash.Write(password)
		hash.Write(salt)
		prev = hash.Sum(nil)
		result = append(result, prev...)
	}
	return result[:keyLen], result[keyLen:totalLen]
}

func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("pkcs7: data is empty")
	}
	padLength := int(data[len(data)-1])
	if padLength > len(data) {
		return nil, errors.New("pkcs7: invalid padding")
	}
	for i := 0; i < padLength; i++ {
		if data[len(data)-1-i] != byte(padLength) {
			return nil, errors.New("pkcs7: invalid padding")
		}
	}
	return data[:len(data)-padLength], nil
}

func AesDecrypt(ciphertextBase64 string, passphrase string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < 16 || string(ciphertext[:8]) != "Salted__" {
		return nil, errors.New("the ciphertext format is incorrect and salted__overseas is missing")
	}
	salt := ciphertext[8:16]
	encryptedData := ciphertext[16:]

	password := []byte(passphrase)

	key, iv := EvpBytesToKey(password, salt, 32, 16)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(encryptedData)%aes.BlockSize != 0 {
		return nil, errors.New("the ciphertext length is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(encryptedData, encryptedData)

	unpadded, err := pkcs7Unpad(encryptedData)
	if err != nil {
		return nil, err
	}
	return unpadded, nil
}
