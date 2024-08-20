package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

const uploadDir = "./uploads"

var (
	password     string
	passwordLock sync.RWMutex
	isProtected  bool
)

func main() {
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	http.HandleFunc("/", authMiddleware(indexHandler))
	http.HandleFunc("/upload", authMiddleware(uploadHandler))
	http.HandleFunc("/files/", authMiddleware(fileHandler))
	http.HandleFunc("/rename", authMiddleware(renameHandler))
	http.HandleFunc("/delete", authMiddleware(deleteHandler))
	http.HandleFunc("/create-folder", authMiddleware(createFolderHandler))
	http.HandleFunc("/set-password", setPasswordHandler)
	http.HandleFunc("/toggle-protection", toggleProtectionHandler)
	http.HandleFunc("/list", authMiddleware(listHandler))

	fmt.Println("Server is running on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		passwordLock.RLock()
		currentIsProtected := isProtected
		currentPassword := password
		passwordLock.RUnlock()

		if currentIsProtected {
			_, pass, ok := r.BasicAuth()
			if !ok || pass != currentPassword {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Error getting file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	dir := r.FormValue("dir")
	filename := filepath.Join(uploadDir, dir, header.Filename)
	out, err := os.Create(filename)
	if err != nil {
		http.Error(w, "Error creating file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		http.Error(w, "Error copying file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func fileHandler(w http.ResponseWriter, r *http.Request) {
	filename := filepath.Join(uploadDir, r.URL.Path[7:])
	http.ServeFile(w, r, filename)
}

func renameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var renameRequest struct {
		OldPath string `json:"oldPath"`
		NewPath string `json:"newPath"`
	}

	err := json.NewDecoder(r.Body).Decode(&renameRequest)
	if err != nil {
		http.Error(w, "Error decoding request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	oldPath := filepath.Join(uploadDir, renameRequest.OldPath)
	newPath := filepath.Join(uploadDir, renameRequest.NewPath)

	err = os.Rename(oldPath, newPath)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func setPasswordHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var passwordRequest struct {
		Password string `json:"password"`
	}

	err := json.NewDecoder(r.Body).Decode(&passwordRequest)
	if err != nil {
		http.Error(w, "Error decoding request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	passwordLock.Lock()
	password = passwordRequest.Password
	passwordLock.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func toggleProtectionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	passwordLock.Lock()
	isProtected = !isProtected
	currentIsProtected := isProtected
	passwordLock.Unlock()

	json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "isProtected": currentIsProtected})
}

func deleteHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var deleteRequest struct {
		Path string `json:"path"`
	}

	err := json.NewDecoder(r.Body).Decode(&deleteRequest)
	if err != nil {
		http.Error(w, "Error decoding request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(uploadDir, deleteRequest.Path)
	err = os.RemoveAll(fullPath)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

func createFolderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var folderRequest struct {
		Path string `json:"path"`
	}

	err := json.NewDecoder(r.Body).Decode(&folderRequest)
	if err != nil {
		http.Error(w, "Error decoding request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	folderPath := filepath.Join(uploadDir, folderRequest.Path)
	err = os.MkdirAll(folderPath, os.ModePerm)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "error": err.Error()})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{"success": true})
}

type FileInfo struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	dir := r.URL.Query().Get("dir")
	files, err := getFileList(dir)
	if err != nil {
		http.Error(w, "Error getting file list: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(files)
}

func getFileList(dir string) ([]FileInfo, error) {
	var files []FileInfo
	fullPath := filepath.Join(uploadDir, dir)
	err := filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(uploadDir, path)
		if relPath != "." {
			files = append(files, FileInfo{
				Name:  info.Name(),
				Path:  relPath,
				IsDir: info.IsDir(),
			})
		}
		return nil
	})
	return files, err
}
