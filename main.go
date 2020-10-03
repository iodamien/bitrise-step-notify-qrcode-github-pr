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
	ApkPath       string `env:"apk_path,required"`
	RepositoryURL string `env:"repository_url,required"`
	BranchName    string `env:"branch_name,required"`
	APIBaseURL    string `env:"api_base_url,required"`
	PullRequestId string `env:"pull_request_id"`
	Commit        string `env:"commit"`
}

type User struct {
	Id json.Number `json:"id"`
	Login string `json:"login"`
}

type IssueComment struct {
	Id json.Number `json:"id"`
	Body string `json:"body"`
	User User
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

	defer resp.Body.Close()

	return -1, fmt.Errorf("failed to found PR")
}

func getUserId(config Config, user *User) error {
	url := fmt.Sprintf("%s/user", config.APIBaseURL)
	req, err := http.NewRequest("GET", url, nil)
	log.Debugf("url: %s", url)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "token "+ string(config.AuthToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return err
	}

	respBody, _ := ioutil.ReadAll(res.Body)
	log.Debugf("get user: %s", respBody)
	err = json.Unmarshal(respBody, user)

	if err != nil {
		return err
	}

	defer res.Body.Close()

	return nil
}

func deletePreviousComment(config Config, owner string, repo string, currentUserId int64, newCommentId int64)  {

	// Get all comments on the issue
	getCommentsURL := fmt.Sprintf("%s/repos/%s/%s/issues/%s/comments", config.APIBaseURL, owner, repo, config.PullRequestId)
	req, err := http.NewRequest("GET", getCommentsURL, nil)

	if err != nil {
		log.Errorf("Error: %s\n", err)
		return
	}

	req.Header.Set("Authorization", "token "+ string(config.AuthToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		log.Errorf("Error: %s\n", err)
		return
	}

	respBody, _ := ioutil.ReadAll(res.Body)
	var comments = make([]IssueComment, 0)
	err = json.Unmarshal(respBody, &comments)
	for _, el := range comments {
		userId, err := el.User.Id.Int64()
		if  err == nil {
			commentId, _ := el.Id.Int64()
			if userId == currentUserId && commentId != newCommentId{
				deleteComment(config, owner, repo, commentId)
			}
		} else {
			log.Errorf("Error: %s\n", err)
		}
	}
}

func deleteComment(config Config, owner string, repo string, commentId int64)  {
	url := fmt.Sprintf("%s/repos/%s/%s/issues/comments/%d", config.APIBaseURL, owner, repo, commentId)
	req, err := http.NewRequest("DELETE", url, nil)

	if err != nil {
		log.Errorf("Error: %s\n", err)
		return
	}

	req.Header.Set("Authorization", "token "+ string(config.AuthToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	http.DefaultClient.Do(req)
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
	stepconf.Print(conf)

	owner, repo := ownerAndRepo(conf.RepositoryURL)

	if conf.PullRequestId == "" {
		pr, err := findIssueByBranchName(conf, owner, repo)
		if err != nil {
			log.Errorf("Error: %s\n", err)
			os.Exit(1)
		}
		conf.PullRequestId = strconv.FormatInt(pr, 10)
	}

	// Post Comment
	url := fmt.Sprintf("%s/repos/%s/%s/issues/%s/comments", conf.APIBaseURL, owner, repo, conf.PullRequestId)
	fmt.Println(url)
	var googleAPiQRCode = fmt.Sprintf("http://chart.apis.google.com/chart?cht=qr&chs=200x200&chld=L|0&chl=%s", conf.ApkPath)
	var qrCodeMarkDown = fmt.Sprintf("![QrCode](%s)", googleAPiQRCode)
	data := Payload{ qrCodeMarkDown }

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

	var comment IssueComment
	err = json.Unmarshal(respBody, &comment)
	commentId, _ := comment.Id.Int64()

	defer resp.Body.Close()

	// Delete previous comment
	var user User
	err = getUserId(conf, &user)

	if err == nil {
		userId, err := user.Id.Int64()
		if err == nil {
			deletePreviousComment(conf, owner, repo, userId, commentId)
		} else {
			log.Errorf("Error: %s\n", err)
		}
	} else {
		log.Errorf("Error: %s\n", err)
	}

	os.Exit(0)
}
