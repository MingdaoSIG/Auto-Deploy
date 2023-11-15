package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/acarl005/stripansi"
	"github.com/spf13/viper"
)

type Payload struct {
	Repository struct {
		ID            int    `json:"id"`
		CloneURL      string `json:"clone_url"`
		DefaultBranch string `json:"default_branch"`
	} `json:"repository"`
}

var defaultPort = "8000"
var defaultPath = "/auto-deploy"
var defaultToken = ""

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./")

	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("No config file found. Using default values.")
	} else {
		fmt.Println("Config file found. Using user defined values.")
	}

	port := viper.GetString("port")
	if port == "" {
		port = defaultPort
	}

	path := viper.GetString("path")
	if path == "" {
		path = defaultPath
	}

	token := viper.GetString("token")
	if token == "" {
		token = defaultToken
	}

	http.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		payloadHandler(w, r, token)
	})

	fmt.Println("Server listening on port " + port + " ...")
	fmt.Println("Auto deploy path: " + path)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func payloadHandler(w http.ResponseWriter, r *http.Request, token string) {
	if r.Method == "POST" {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		var payload Payload
		err = json.Unmarshal(body, &payload)
		if err != nil {
			http.Error(w, "Failed to parse JSON body", http.StatusBadRequest)
			return
		}

		if payload.Repository.CloneURL == "" {
			http.Error(w, "Missing CloneURL in payload", http.StatusBadRequest)
			return
		}

		if payload.Repository.DefaultBranch == "" {
			http.Error(w, "Missing DefaultBranch in payload", http.StatusBadRequest)
			return
		}

		repoURL := payload.Repository.CloneURL
		defaultBranch := payload.Repository.DefaultBranch
		repoID := strconv.Itoa(payload.Repository.ID)
		parts := strings.Split(repoURL, "github.com")
		newRepoURL := fmt.Sprintf("%s%s@github.com%s", parts[0], token, parts[1])

		fmt.Println("Repo URL " + repoURL)
		fmt.Println("Default Branch " + defaultBranch)
		fmt.Println("Repo ID " + repoID)
		fmt.Println("Token " + token)
		fmt.Println("Repo URL With Token " + newRepoURL)

		go func() {
			buildContainer := exec.Command("./docker_builder.sh", "REPO_URL="+repoURL, "DEFAULT_BRANCH="+defaultBranch, "REPO_ID="+repoID)

			logFile, buildContainerError := os.Create("./logs/log-" + repoID + ".log")
			if buildContainerError != nil {
				panic(buildContainerError)
			}
			defer logFile.Close()

			buildContainer.Stdout = &cleanupWriter{writer: logFile}
			buildContainer.Stderr = &cleanupWriter{writer: logFile}
			buildContainerError = buildContainer.Start()
			buildContainer.Wait()

			if buildContainerError != nil {
				fmt.Println("Deployment failed for "+repoID+": ", buildContainerError)
			} else {
				fmt.Println("Deployment for " + repoID + " successful")
			}
		}()

		fmt.Fprint(w, "Deployment for "+repoID+" started")
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
}

// cleanupWriter is a custom writer that strips ANSI escape codes before writing to a file
type cleanupWriter struct {
	writer *os.File
}

// Write writes the cleaned output to the file
func (w *cleanupWriter) Write(p []byte) (n int, err error) {
	cleanOutput := stripansi.Strip(string(p))
	_, err = w.writer.WriteString(cleanOutput)
	return len(p), err
}
