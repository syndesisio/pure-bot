// Copyright © 2017 Syndesis Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhook

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/syndesisio/pure-bot/pkg/config"
	"go.uber.org/multierr"
	"go.uber.org/zap"
)

const (
	labeledEvent            = "labeled"
	statusEventSuccessState = "success"
)

type autoMerger struct{}

func (h *autoMerger) EventTypesHandled() []string {
	return []string{"pull_request", "status", "pull_request_review"}
}

func (h *autoMerger) HandleEvent(eventObject interface{}, gh *github.Client, config config.GitHubAppConfig, logger *zap.Logger) error {
	switch event := eventObject.(type) {
	case *github.PullRequestEvent:
		return h.handlePullRequestEvent(event, gh)
	case *github.StatusEvent:
		return h.handleStatusEvent(event, gh)
	case *github.PullRequestReviewEvent:
		return h.handlePullRequestReviewEvent(event, gh)
	default:
		return nil
	}
}

func (h *autoMerger) handlePullRequestReviewEvent(event *github.PullRequestReviewEvent, gh *github.Client) error {
	if strings.ToLower(event.Review.GetState()) != assignedAction {
		return nil
	}

	return h.mergePRFromPullRequestEvent(event.Installation.GetID(), event.Repo, event.PullRequest, gh)
}

func (h *autoMerger) handlePullRequestEvent(event *github.PullRequestEvent, gh *github.Client) error {
	if strings.ToLower(event.GetAction()) != labeledEvent {
		return nil
	}

	return h.mergePRFromPullRequestEvent(event.Installation.GetID(), event.Repo, event.PullRequest, gh)
}

func (h *autoMerger) mergePRFromPullRequestEvent(installationID int64, repo *github.Repository, pullRequest *github.PullRequest, gh *github.Client) error {
	issue, _, err := gh.Issues.Get(context.Background(), repo.Owner.GetLogin(), repo.GetName(), pullRequest.GetNumber())
	if err != nil {
		return errors.Wrapf(err, "failed to get pull request %s", pullRequest.GetHTMLURL())
	}

	return mergePR(issue, pullRequest, repo.Owner.GetLogin(), repo.GetName(), gh, "")
}

func (h *autoMerger) handleStatusEvent(event *github.StatusEvent, gh *github.Client) error {
	if strings.ToLower(event.GetState()) != statusEventSuccessState {
		return nil
	}

	commitSHA := event.GetSHA()
	query := fmt.Sprintf("type:pr state:open repo:%s %s", event.Repo.GetFullName(), commitSHA)
	searchResult, _, err := gh.Search.Issues(context.Background(), query, nil)
	if err != nil {
		return errors.Wrap(err, "failed to search for open issues")
	}
	var multiErr error
	for _, issue := range searchResult.Issues {
		if issue.PullRequestLinks == nil {
			continue
		}

		pr, _, err := gh.PullRequests.Get(context.Background(), event.Repo.Owner.GetLogin(), event.Repo.GetName(), issue.GetNumber())
		if err != nil {
			multiErr = multierr.Combine(multiErr, err)
			continue
		}

		err = mergePR(&issue, pr, event.Repo.Owner.GetLogin(), event.Repo.GetName(), gh, commitSHA)
		if err != nil {
			multiErr = multierr.Combine(multiErr, err)
			continue
		}
	}

	return multiErr
}

func mergePR(issue *github.Issue, pr *github.PullRequest, owner, repository string, gh *github.Client, commitSHA string) error {
	if !containsLabel(issue.Labels, approvedLabel) {
		return nil
	}

	if commitSHA != "" && pr.Head.GetSHA() != commitSHA {
		return nil
	}
	commitSHA = pr.Head.GetSHA()

	statuses, _, err := gh.Repositories.GetCombinedStatus(context.Background(), owner, repository, commitSHA, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to get statuses of pull request %s", issue.GetHTMLURL())
	}

	prStatusMap := make(map[string]bool, len(statuses.Statuses))
	for _, status := range statuses.Statuses {
		prStatusMap[status.GetContext()] = status.GetState() == statusEventSuccessState
	}

	requiredContexts, _, err := gh.Repositories.ListRequiredStatusChecksContexts(context.Background(), owner, repository, pr.Base.GetRef())
	if err != nil {
		if errResp, ok := err.(*github.ErrorResponse); !ok || errResp.Response.StatusCode != http.StatusNotFound {
			return errors.Wrapf(err, "failed to get target branch (%s) protection for pull request %s", pr.Base.GetRef(), issue.GetHTMLURL())
		}
	}

	if len(requiredContexts) == 0 {
		for _, contextStatus := range prStatusMap {
			if !contextStatus {
				return nil
			}
		}
	} else {
		for _, requiredContext := range requiredContexts {
			if success, present := prStatusMap[requiredContext]; !present || !success {
				return nil
			}
		}
	}

	_, _, err = gh.PullRequests.Merge(context.Background(), owner, repository, issue.GetNumber(), "", &github.PullRequestOptions{
		SHA: commitSHA,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to merge pull request %s", issue.GetHTMLURL())
	}

	return nil
}
