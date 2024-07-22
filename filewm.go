package main

import (
	"encoding/json"
	"fmt"
	"html/template"
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
	http.HandleFunc("/set-password", setPasswordHandler)
	http.HandleFunc("/toggle-protection", toggleProtectionHandler)

	fmt.Println("Server is running on http://localhost:80")
	log.Fatal(http.ListenAndServe(":80", nil))
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
	files, err := getFileList()
	if err != nil {
		http.Error(w, "Error getting file list: "+err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl := `
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>企业文件管理器</title>
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 800px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f4f4f4;
        }
        .container {
            background-color: #fff;
            padding: 20px;
            border-radius: 5px;
            box-shadow: 0 2px 5px rgba(0,0,0,0.1);
        }
        h1, h2 {
            color: #2c3e50;
            border-bottom: 2px solid #3498db;
            padding-bottom: 10px;
        }
        form {
            margin-bottom: 20px;
        }
        input[type="file"], input[type="text"], input[type="password"] {
            margin-right: 10px;
        }
        input[type="submit"], button {
            background-color: #3498db;
            color: #fff;
            border: none;
            padding: 5px 10px;
            cursor: pointer;
            border-radius: 3px;
        }
        input[type="submit"]:hover, button:hover {
            background-color: #2980b9;
        }
        ul {
            list-style-type: none;
            padding: 0;
        }
        li {
            background-color: #ecf0f1;
            margin-bottom: 5px;
            padding: 10px;
            border-radius: 3px;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        a {
            color: #2980b9;
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
        }
        #progress-bar {
            width: 100%;
            background-color: #f0f0f0;
            padding: 3px;
            border-radius: 3px;
            box-shadow: inset 0 1px 3px rgba(0, 0, 0, .2);
        }
        #progress-bar-fill {
            display: block;
            height: 22px;
            background-color: #3498db;
            border-radius: 3px;
            transition: width 500ms ease-in-out;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>企业文件管理器</h1>
        <form id="upload-form" enctype="multipart/form-data">
            <input type="file" name="file" id="file-input">
            <input type="submit" value="上传文件">
        </form>
        <div id="progress-bar" style="display:none;">
            <span id="progress-bar-fill" style="width: 0%"></span>
        </div>
        <h2>文件列表：</h2>
        <ul id="file-list">
        {{range .}}
            <li>
                <a href="/files/{{.}}">{{.}}</a>
                <div>
                    <input type="text" id="new-name-{{.}}" placeholder="新文件名">
                    <button onclick="renameFile('{{.}}')">重命名</button>
                </div>
            </li>
        {{else}}
            <li>暂无文件</li>
        {{end}}
        </ul>
        <h2>密码保护：</h2>
        <form id="password-form">
            <input type="password" id="password" placeholder="设置新密码">
            <button type="button" onclick="setPassword()">设置密码</button>
        </form>
        <button onclick="toggleProtection()">切换密码保护</button>
    </div>
    <script>
        document.getElementById('upload-form').addEventListener('submit', function(e) {
            e.preventDefault();
            var formData = new FormData(this);
            var xhr = new XMLHttpRequest();
            xhr.open('POST', '/upload', true);
            xhr.upload.onprogress = function(e) {
                if (e.lengthComputable) {
                    var percentComplete = (e.loaded / e.total) * 100;
                    document.getElementById('progress-bar').style.display = 'block';
                    document.getElementById('progress-bar-fill').style.width = percentComplete + '%';
                }
            };
            xhr.onload = function() {
                if (xhr.status === 200) {
                    location.reload();
                } else {
                    alert('Upload failed. Please try again.');
                }
            };
            xhr.send(formData);
        });

        function renameFile(oldName) {
            var newName = document.getElementById('new-name-' + oldName).value;
            if (!newName) {
                alert('Please enter a new name');
                return;
            }
            fetch('/rename', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({oldName: oldName, newName: newName}),
            })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    location.reload();
                } else {
                    alert('Rename failed: ' + data.error);
                }
            })
            .catch((error) => {
                console.error('Error:', error);
                alert('An error occurred while renaming the file');
            });
        }

        function setPassword() {
            var password = document.getElementById('password').value;
            fetch('/set-password', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({password: password}),
            })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    alert('Password set successfully');
                } else {
                    alert('Failed to set password: ' + data.error);
                }
            })
            .catch((error) => {
                console.error('Error:', error);
                alert('An error occurred while setting the password');
            });
        }

        function toggleProtection() {
            fetch('/toggle-protection', {
                method: 'POST',
            })
            .then(response => response.json())
            .then(data => {
                if (data.success) {
                    alert('Password protection ' + (data.isProtected ? 'enabled' : 'disabled'));
                } else {
                    alert('Failed to toggle protection: ' + data.error);
                }
            })
            .catch((error) => {
                console.error('Error:', error);
                alert('An error occurred while toggling protection');
            });
        }
    </script>
</body>
</html>
`

	t, err := template.New("index").Parse(tmpl)
	if err != nil {
		http.Error(w, "Error parsing template: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := t.Execute(w, files); err != nil {
		http.Error(w, "Error executing template: "+err.Error(), http.StatusInternalServerError)
	}
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

	filename := filepath.Join(uploadDir, header.Filename)
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
	filename := filepath.Join(uploadDir, filepath.Base(r.URL.Path))
	http.ServeFile(w, r, filename)
}

func renameHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var renameRequest struct {
		OldName string `json:"oldName"`
		NewName string `json:"newName"`
	}

	err := json.NewDecoder(r.Body).Decode(&renameRequest)
	if err != nil {
		http.Error(w, "Error decoding request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	oldPath := filepath.Join(uploadDir, renameRequest.OldName)
	newPath := filepath.Join(uploadDir, renameRequest.NewName)

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

func getFileList() ([]string, error) {
	var files []string
	err := filepath.Walk(uploadDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, info.Name())
		}
		return nil
	})
	return files, err
}
