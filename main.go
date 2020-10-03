package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-tools/go-steputils/stepconf"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AuthToken     stepconf.Secret `env:"github_token,required"`
	Comment       string `env:"comment,required"`
	RepositoryURL string `env:"repository_url,required"`
	BranchName    string `env:"branch_name,required"`
	APIBaseURL    string `env:"api_base_url,required"`
	PullRequestId string `env:"pull_request_id"`
	Commit        string `env:"commit"`
}

type Payload struct {
	Body string `json:"body"`
}

type Head struct {
	Sha string
}

type PullRequest struct {
	State string
	Number json.Number
	Head  Head
}

func findIssueByBranchName(config Config, owner string, repo string) (int64, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls", config.APIBaseURL, owner, repo)
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "token "+ string(config.AuthToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	q := req.URL.Query()
	q.Add("head", owner+":"+ string(config.BranchName))
	req.URL.RawQuery = q.Encode()

	if err != nil {
		log.Errorf("Error: %s\n", err)
		return -1, err
	}

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		log.Errorf("Error: %s\n", err)
		return -1, err
	}

	respBody, _ := ioutil.ReadAll(resp.Body)

	var p = make([]PullRequest, 0)
	err = json.Unmarshal(respBody, &p)

	if err != nil {
		log.Errorf("Error: %s\n", err)
		return -1, err
	}

	for _, el := range p {
		if el.State == "open" && el.Head.Sha == config.Commit {
			return el.Number.Int64()
		}
	}
	return -1, fmt.Errorf("failed to found PR")
}

func ownerAndRepo(url string) (string, string) {
	url = strings.TrimPrefix(strings.TrimPrefix(url, "https://"), "git@")
	paths := strings.FieldsFunc(url, func(r rune) bool { return r == '/' || r == ':' })
	return paths[1], strings.TrimSuffix(paths[2], ".git")
}

func main() {
	var conf Config

	if err := stepconf.Parse(&conf); err != nil {
		log.Errorf("Error: %s\n", err)
		os.Exit(1)
	}

	owner, repo := ownerAndRepo(conf.RepositoryURL)

	if conf.PullRequestId == "" {
		pr, err := findIssueByBranchName(conf, owner, repo)
		if err != nil {
			log.Errorf("Error: %s\n", err)
			os.Exit(1)
		}
		conf.PullRequestId = strconv.FormatInt(pr, 10)
	}


	stepconf.Print(conf)

	// Post Comment
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%s/comments", conf.APIBaseURL, owner, repo, conf.PullRequestId)
	fmt.Println(url)
	data := Payload{conf.Comment}

	payloadBytes, err := json.Marshal(data)

	if err != nil {
		log.Errorf("Error: %s\n", err)
		os.Exit(1)
	}

	body := bytes.NewReader(payloadBytes)

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		log.Errorf("Error: %s\n", err)
		os.Exit(1)
	}
	req.Header.Set("Authorization", "token " + string(conf.AuthToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Errorf("Error: %s\n", err)
		os.Exit(1)
	}
	respBody, _ := ioutil.ReadAll(resp.Body)
	log.Successf("Success: %s\n", respBody)
	defer resp.Body.Close()

	os.Exit(0)
}
