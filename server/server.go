package server

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/prudhvideep/docengine/util"
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

		err = conn.WriteMessage(websocket.TextMessage, []byte("Go to Url : "+"http://localhost:8080/docs/"+repoName+".md"))
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
