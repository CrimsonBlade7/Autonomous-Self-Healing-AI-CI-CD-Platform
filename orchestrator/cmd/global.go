package main

import "github.com/moby/moby/client"

var (
	secret     string
	port       string
	repoDir    string = "./temp_repos"
	jobQueue   chan Job
	mobyClient *client.Client
)

type PullRequest struct {
	Action  string `json:"action"`
	Url     string `json:"url"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	HeadSHA string `json:"headsha"`
	BaseSHA string `json:"basesha"`
}

type Job struct {
	PullReq PullRequest
}
