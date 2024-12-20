package util

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/websocket"
)

const promptLimit = 30000

var validFileExtensions = map[string]bool{
	".go":   true,
	".py":   true,
	".html": true,
	".js":   true,
	".jsx":  true,
	".ts":   true,
	".tsx":  true,
	".css":  true,
	".java": true,
	".cpp":  true,
	".c":    true,
	".rb":   true,
	".sh":   true,
}

func GetRepoName(url string) (string, error) {
	splistrs := strings.Split(url, "/")

	if len(splistrs) > 0 {
		repoName := splistrs[len(splistrs)-1]
		repoName = strings.Split(repoName, ".")[0]
		return repoName, nil
	} else {
		return "", errors.New("not a valid url")
	}
}

func PreprocessRepo(reponame string, conn *websocket.Conn) error {
	promptname := "prompt.txt"
	promptLen := 0
	basepath := "./repo"
	repoPath := filepath.Join(basepath, reponame)

	originalDir, err := os.Getwd()
	if err != nil {
		conn.WriteMessage(websocket.TextMessage, []byte("could not get current working directory "))
		return fmt.Errorf("could not get current working directory: %w", err)
	}

	err = resetPromptTextFile(filepath.Join(originalDir, promptname))
	if err != nil {
		return fmt.Errorf("error resetting the prompt query", err)
	}

	err = os.Chdir(repoPath)
	if err != nil {
		return fmt.Errorf("could not change directory to %s: %w", repoPath, err)
	}
	defer os.Chdir(originalDir)
	filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {

		//Skipping git ignore files
		if d.IsDir() && (d.Name() == ".git") {
			return filepath.SkipDir
		}

		fileName := filepath.Base(path)
		if !d.IsDir() && isValidExt(path) {

			log.Println("Processing:", path, fileName)
			conn.WriteMessage(websocket.TextMessage, []byte("Processing File "+path))

			wc, err := getWordCount(path)
			if err != nil {
				log.Println("Error fetching the word count")
			}
			log.Println("Word Count:", wc)

			if wc+promptLen < promptLimit {
				e := appendToPrompt(path, fileName, filepath.Join(originalDir, promptname))
				if e != nil {
					return e
				}

				promptLen += wc
			}
		}

		return nil
	})

	return nil
}

func appendToPrompt(path string, fileName string, prompt string) error {
	f, err := os.Open(path)

	if err != nil {
		return err
	}

	pf, err := os.OpenFile(prompt, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0666)
	if err != nil {
		return err
	}

	reader := bufio.NewReader(f)
	writer := bufio.NewWriter(pf)

	writer.WriteString(fileName + "\n")
	writer.Flush()

	for {
		line, _, err := reader.ReadLine()

		if err != nil {
			break
		}

		writer.WriteString(string(line))

	}
	writer.WriteString("\n")
	writer.Flush()

	return nil
}

func isValidExt(path string) bool {
	fileName := filepath.Base(path)

	if strings.EqualFold("DOCKERFILE", fileName) {
		return true
	}

	ext := filepath.Ext(path)

	return validFileExtensions[ext]

}

func resetPromptTextFile(path string) error {

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	log.Println("File content has been reset.")

	writer := bufio.NewWriter(file)
	_, err = writer.WriteString("Generate comprehensive documentation in markdown format for the following project. The documentation should include:\n")
	writer.WriteString("A Table of Contents that links to different modules of the project.\n")
	writer.WriteString("An overview of each module with a brief description. Ignore this for various config files. Also the modules should be genric not always the filenames\n")
	writer.WriteString("Give a broad overview regarding key functions, including their purpose, and examples of usage.\n")
	writer.WriteString("Also add the steps users can follow to implement or deploy this project\n")
	writer.WriteString("Also can you highlight any vulnerabilites in security and any possible bugs in the code\n")

	if err != nil {
		return fmt.Errorf("could not write to file: %w", err)
	}

	return writer.Flush()

}

func getWordCount(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanWords)

	wc := 0
	for scanner.Scan() {
		wc++
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return wc, nil
}
