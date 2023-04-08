package routes

import (
	"encoding/json"
	"fmt"
	"github.com/liamrlawrence/sigil-rest_api/internal/logging"
	"github.com/liamrlawrence/sigil-rest_api/internal/server"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

func ParseJenkinsConsoleForStdout(s string) string {
	lines := strings.Split(s, "\n")
	startIndex := -1
	endIndex := -1

	for i, line := range lines {
		if strings.Contains(line, "[Pipeline] sh") {
			startIndex = i
		} else if strings.Contains(line, "[Pipeline] }") {
			endIndex = i
			break
		}
	}

	if startIndex != -1 && endIndex != -1 {
		return strings.Join(lines[startIndex+1:endIndex], "\n")
	}

	return "Could not find the start and end markers in the input."
}

func GetJenkinsCrumb() []string {
	jenkinsURL := "https://cthulhu.cloud/jenkins"
	username := "grimoire-api"
	apiToken := os.Getenv("JENKINS_API_TOKEN")

	// Get the CSRF crumb
	crumbURL := fmt.Sprintf("%s/crumbIssuer/api/xml?xpath=concat(//crumbRequestField,\":\",//crumb)", jenkinsURL)
	req, _ := http.NewRequest("GET", crumbURL, nil)
	req.SetBasicAuth(username, apiToken)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error fetching CSRF crumb:", err)
		return nil
	}
	defer resp.Body.Close()

	crumb, _ := ioutil.ReadAll(resp.Body)
	crumbHeader := strings.Split(string(crumb), ":")

	return crumbHeader
}

func HandlerRouteBotoLogs(s *server.Server) func(w http.ResponseWriter, r *http.Request) {
	type JobStatus struct {
		Building bool `json:"building"`
	}

	type ConsoleText struct {
		Text string `json:"text"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		jenkinsURL := "https://cthulhu.cloud/jenkins"
		jobName := "api-botoroboto-logs"
		username := "grimoire-api"
		apiToken := os.Getenv("JENKINS_API_TOKEN")
		crumbHeader := GetJenkinsCrumb()

		logging.APIEndpoint(r, "GET", "/api/boto/logs")

		// Start the job
		buildURL := fmt.Sprintf("%s/job/%s/build", jenkinsURL, jobName)
		req, _ := http.NewRequest("POST", buildURL, nil)
		req.SetBasicAuth(username, apiToken)
		req.Header.Set(crumbHeader[0], crumbHeader[1])

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("error triggering build: %v", err)
		}
		defer resp.Body.Close()

		// Poll the job status until it's finished
		statusURL := fmt.Sprintf("%s/job/%s/lastBuild/api/json", jenkinsURL, jobName)
		for {
			req, _ = http.NewRequest("GET", statusURL, nil)
			req.SetBasicAuth(username, apiToken)
			req.Header.Set(crumbHeader[0], crumbHeader[1])

			resp, err = client.Do(req)
			if err != nil {
				fmt.Printf("error fetching job status: %v", err)
			}
			defer resp.Body.Close()

			var status JobStatus
			err = json.NewDecoder(resp.Body).Decode(&status)
			if err != nil {
				fmt.Printf("error decoding job status JSON: %v", err)
			}

			if !status.Building {
				break
			}

			time.Sleep(2 * time.Second)
		}

		// Get the console text
		consoleTextURL := fmt.Sprintf("%s/job/%s/lastBuild/consoleText", jenkinsURL, jobName)
		req, _ = http.NewRequest("GET", consoleTextURL, nil)
		req.SetBasicAuth(username, apiToken)

		resp, err = client.Do(req)
		if err != nil {
			fmt.Printf("error fetching console text: %v", err)
		}
		defer resp.Body.Close()

		consoleTextBytes, _ := ioutil.ReadAll(resp.Body)
		consoleText := ParseJenkinsConsoleForStdout(strings.TrimSpace(string(consoleTextBytes)))

		// Return the response
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		fmt.Fprintf(w, `{
	"status": "success",
	"message": "got logs for Boto",
	"data": "%v"
}`, strings.Replace(strings.Replace(consoleText, "\n", "\\n", -1), "\"", "\\\"", -1))
		return
	}
}
