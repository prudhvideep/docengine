package server

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prudhvideep/docengine/util"
	"github.com/yuin/goldmark"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func HandleDocGen(rw http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(rw, r, nil)
	if err != nil {
		log.Println("Error upgrading the http connection")
		return
	}

	defer conn.Close()

	for {
		var url string = ""

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Println("Error Reading Message", err)
			break
		}
		log.Println("Received Message ---> ", string(message))

		url = string(message)

		CloneRepo(url, conn)

		repoName, err := util.GetRepoName(url)
		if err != nil {
			log.Println("Error fetching the repo name ", err)
			return
		}

		log.Println("Repo name ", repoName)

		err = conn.WriteMessage(websocket.TextMessage, []byte("Repo Name "+repoName))
		if err != nil {
			log.Println("Error sending the message")
		}

		util.PreprocessRepo(repoName, conn)

		wd, err := os.Getwd()
		if err != nil {
			log.Println("Error fetching the cwd ", err)
			return
		}

		geminiUrl := "https://generativelanguage.googleapis.com/v1beta/models/gemini-1.5-flash-latest:generateContent?key=AIzaSyAKSSCovhrY5hp44vjWXIkYv-jmzddyYug"

		err = conn.WriteMessage(websocket.TextMessage, []byte("Preparing the documentation"))
		if err != nil {
			log.Println("Error getting the summary")
		}

		util.PostPrompt(conn, repoName, geminiUrl, filepath.Join(wd, "prompt.txt"))

		err = conn.WriteMessage(websocket.TextMessage, []byte("Docs generated"))
		if err != nil {
			log.Println("Error sending the message")
		}

		err = conn.WriteMessage(websocket.TextMessage, []byte("Go to Url : "+"http://localhost:8080/docs?repo="+repoName+".md"))
		if err != nil {
			log.Println("Error sending the message")
		}

		conn.WriteMessage(websocket.TextMessage, []byte("Done"))

		if strings.Compare(string(message), "Stop") == 0 {
			conn.Close()
		}

	}
}

func HandleGeneralRoute(rw http.ResponseWriter, r *http.Request) {
	rw.Write([]byte("Welcome to docgen"))
}

func CloneRepo(repo string, conn *websocket.Conn) {
	cleanRepo(conn)

	time.Sleep(100 * time.Millisecond)

	err := conn.WriteMessage(websocket.TextMessage, []byte("Processing the repo "+repo))
	if err != nil {
		log.Println("Error sending the message")
	}

	conn.WriteMessage(websocket.TextMessage, []byte("Cloning the repo "))

	if err := os.Mkdir("repo", os.ModePerm); err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error creating the directory"))
		return
	}

	cmd := exec.Command("git", "clone", repo)
	cmd.Dir = "./repo"
	output, err := cmd.CombinedOutput()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error cloning the repo"+string(output)))
		conn.WriteMessage(websocket.TextMessage, []byte("Done"))
		log.Println("Error executing command:", err)
		return
	}

	conn.WriteMessage(websocket.TextMessage, []byte(output))

}

func cleanRepo(conn *websocket.Conn) {
	path := "./repo"

	err := os.RemoveAll(path)

	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("Error removing the files"))
		return
	}
}

func ServeFormattedDoc(w http.ResponseWriter, r *http.Request) {
	repoName := r.URL.Query().Get("repo")
	if repoName == "" {
		http.Error(w, "Missing repo name", http.StatusBadRequest)
		return
	}

	docPath := filepath.Join("./docs", repoName)

	// Read the markdown file
	content, err := os.ReadFile(docPath)
	if err != nil {
		http.Error(w, "Error reading file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if len(content) == 0 {
		http.Error(w, "Markdown file is empty", http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer

	// Convert markdown to HTML
	err = goldmark.New().Convert(content, &buf)
	if err != nil {
		log.Println("Error converting the markdown ", err)
	}
	repoName = strings.Split(repoName, ".")[0]
	// Use a template to wrap the rendered HTML
	tmpl := `
	<!DOCTYPE html>
<html>
<head>
    <title>Documentation - {{.RepoName}}</title>
    <meta charset="UTF-8">
    <style>
        body {
            font-family: Arial, sans-serif;
            line-height: 1.6;
            margin: 0;
            padding: 0;
            background-color: #121212; /* Deep dark background */
            color: #e0e0e0; /* Light gray text for contrast */
        }
        .container {
            max-width: 800px;
            margin: auto;
            padding: 20px;
        }
        h1 {
            color: #bb86fc; /* Soft purple for headings */
            border-bottom: 1px solid #373737; /* Subtle separator */
            padding-bottom: 10px;
        }
        pre {
            background-color: #1e1e1e; /* Slightly lighter than body for code blocks */
            color: #e0e0e0; /* Light text color */
            padding: 15px;
            border-radius: 4px;
            overflow-x: auto;
            border: 1px solid #373737; /* Subtle border */
        }
        a {
            color: #03dac6; /* Teal for links */
            text-decoration: none;
        }
        a:hover {
            text-decoration: underline;
            color: #bb86fc; /* Same purple as headings for hover state */
        }
        /* Optional: Add scrollbar styling for dark mode */
        ::-webkit-scrollbar {
            width: 12px;
        }
        ::-webkit-scrollbar-track {
            background: #1e1e1e;
        }
        ::-webkit-scrollbar-thumb {
            background-color: #555;
            border-radius: 6px;
        }
        ::-webkit-scrollbar-thumb:hover {
            background-color: #777;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>Documentation for {{.RepoName}}</h1>
        {{.Content}}
    </div>
</body>
</html>`

	t := template.Must(template.New("doc").Parse(tmpl))

	data := struct {
		RepoName string
		Content  template.HTML
	}{
		RepoName: repoName,
		Content:  template.HTML(buf.String()),
	}

	// Render the template
	if err := t.Execute(w, data); err != nil {
		http.Error(w, "Error rendering template: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
