package types

import (
	"encoding/json"
	"fmt"
)

type PullRequest struct {
	RepoName string `json:"name"`
	Number   uint   `json:"number"`
	Action   string `json:"action"`
	Url      string `json:"url"`
	Branch   string `json:"branch"`
	Title    string `json:"title"`
	Body     string `json:"body"`
	HeadSHA  string `json:"headsha"`
	BaseSHA  string `json:"basesha"`
}

// Populates fields from a byte slice
func (pr *PullRequest) UnmarshalJSON(data []byte) error {
	var temp struct {
		Repository struct {
			Name string `json:"name"`
		} `json:"repository"`
		Action      string `json:"action"`
		Number      uint   `json:"number"`
		PullRequest struct {
			Title string `json:"title"`
			Body  string `json:"body"`
			Head  struct {
				Ref  string `json:"ref"`
				Sha  string `json:"sha"`
				Repo struct {
					CloneUrl string `json:"clone_url"`
				} `json:"repo"`
			} `json:"head"`
			Base struct {
				Sha string `json:"sha"`
			} `json:"base"`
		} `json:"pull_request"`
	}

	err := json.Unmarshal(data, &temp)
	if err != nil {
		return fmt.Errorf("Failed to unmarshal json data: %w", err)
	}

	name := temp.Repository.Name
	if name == "" {
		return fmt.Errorf("Name is empty")
	}
	pr.RepoName = name

	number := temp.Number
	if number == 0 {
		return fmt.Errorf("Number is empty")
	}
	pr.Number = number

	action := temp.Action
	if action == "" {
		return fmt.Errorf("Action is empty")
	}
	pr.Action = action

	url := temp.PullRequest.Head.Repo.CloneUrl
	if url == "" {
		return fmt.Errorf("Url is empty")
	}
	pr.Url = url

	branch := temp.PullRequest.Head.Ref
	if branch == "" {
		return fmt.Errorf("Branch is empty")
	}
	pr.Branch = branch

	title := temp.PullRequest.Title
	if title == "" {
		return fmt.Errorf("Title is empty")
	}
	pr.Title = title

	body := temp.PullRequest.Body
	if body == "" {
		return fmt.Errorf("Body is empty")
	}
	pr.Body = body

	headSHA := temp.PullRequest.Head.Sha
	if headSHA == "" {
		return fmt.Errorf("HeadSHA is empty")
	}
	pr.HeadSHA = headSHA

	baseSHA := temp.PullRequest.Base.Sha
	if baseSHA == "" {
		return fmt.Errorf("BaseSHA is empty")
	}
	pr.BaseSHA = baseSHA

	return nil
}
