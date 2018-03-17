package webhook

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/syndesisio/pure-bot/pkg/config"
)

var (
	labelNewIssueHandler Handler = &labelNewIssue{}
)

type labelNewIssue struct{}

func (h *labelNewIssue) HandleEvent(w http.ResponseWriter, eventObject interface{}, ghClientFunc GitHubAppsClientFunc, config config.GitHubAppConfig) error {
	event, ok := eventObject.(*github.IssuesEvent)
	if !ok {
		return errors.New("wrong event eventObject type")
	}

	if event.Installation == nil {
		return nil
	}

	if config.NewIssueLabels == nil {
		return nil
	}

	if strings.ToLower(event.GetAction()) != "opened" {
		return nil
	}

	gh, err := ghClientFunc(event.Installation.GetID())
	if err != nil {
		return errors.Wrap(err, "failed to create a GitHub client")
	}

	_, _, err = gh.Issues.AddLabelsToIssue(context.Background(), event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.Issue.GetNumber(), config.NewIssueLabels)
	return err
}
