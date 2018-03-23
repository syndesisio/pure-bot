package webhook

import (
	"context"
	"strings"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/syndesisio/pure-bot/pkg/config"
	"go.uber.org/zap"
)

type newIssueLabel struct{}

func (h *newIssueLabel) EventTypesHandled() []string {
	return []string{"issues"}
}

func (h *newIssueLabel) HandleEvent(eventObject interface{}, gh *github.Client, config config.RepoConfig, logger *zap.Logger) error {
	event, ok := eventObject.(*github.IssuesEvent)
	if !ok {
		return errors.New("wrong event eventObject type")
	}

	labelConfig := config.Labels

	if labelConfig.NewIssues == nil {
		return nil
	}

	if strings.ToLower(event.GetAction()) != "opened" {
		return nil
	}

	_, _, err := gh.Issues.AddLabelsToIssue(context.Background(), event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.Issue.GetNumber(), labelConfig.NewIssues)
	return err
}
