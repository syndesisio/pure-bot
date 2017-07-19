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
	"regexp"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

var autoMergeHandler Handler = &autoMerger{}

const (
	labeledEvent            = "labeled"
	statusEventSuccessState = "success"
)

type autoMerger struct{}

var issueRegex = regexp.MustCompile("[^(]*\\(#(\\d+)\\).*")

func (h *autoMerger) HandleEvent(w http.ResponseWriter, eventObject interface{}, ghClientFunc GitHubIntegrationsClientFunc) error {
	switch event := eventObject.(type) {
	case *github.PullRequestEvent:
		return h.handlePullRequestEvent(w, event, ghClientFunc)
	case *github.StatusEvent:
		return h.handleStatusEvent(w, event, ghClientFunc)
	case *github.PullRequestReviewEvent:
		return h.handlePullRequestReviewEvent(w, event, ghClientFunc)
	default:
		return nil
	}
}

func (h *autoMerger) handlePullRequestReviewEvent(w http.ResponseWriter, event *github.PullRequestReviewEvent, ghClientFunc GitHubIntegrationsClientFunc) error {
	if strings.ToLower(event.Review.GetState()) != approvedReviewState {
		return nil
	}

	return h.mergePRFromPullRequestEvent(event.Installation.GetID(), event.Repo, event.PullRequest, ghClientFunc)
}

func (h *autoMerger) handlePullRequestEvent(w http.ResponseWriter, event *github.PullRequestEvent, ghClientFunc GitHubIntegrationsClientFunc) error {
	if strings.ToLower(event.GetAction()) != labeledEvent {
		return nil
	}

	return h.mergePRFromPullRequestEvent(event.Installation.GetID(), event.Repo, event.PullRequest, ghClientFunc)
}

func (h *autoMerger) mergePRFromPullRequestEvent(installationID int, repo *github.Repository, pullRequest *github.PullRequest, ghClientFunc GitHubIntegrationsClientFunc) error {
	gh, err := ghClientFunc(installationID)
	if err != nil {
		return errors.Wrap(err, "failed to create a GitHub client")
	}

	issue, _, err := gh.Issues.Get(context.Background(), repo.Owner.GetLogin(), repo.GetName(), pullRequest.GetNumber())
	if err != nil {
		return errors.Wrapf(err, "failed to get pull request %s", pullRequest.GetHTMLURL())
	}

	return mergePR(issue, repo.Owner.GetLogin(), repo.GetName(), gh, "")
}

func (h *autoMerger) handleStatusEvent(w http.ResponseWriter, event *github.StatusEvent, ghClientFunc GitHubIntegrationsClientFunc) error {
	if strings.ToLower(event.GetState()) != statusEventSuccessState {
		return nil
	}

	if event.Installation == nil {
		return nil
	}

	gh, err := ghClientFunc(event.Installation.GetID())
	if err != nil {
		return errors.Wrap(err, "failed to create a GitHub client")
	}

	commitSHA := event.GetSHA()
	query := fmt.Sprintf("type:pr state:open repo:%s %s", event.Repo.GetFullName(), commitSHA)
	searchResult, _, err := gh.Search.Issues(context.Background(), query, nil)
	var multiErr error
	for _, issue := range searchResult.Issues {
		if issue.PullRequestLinks == nil {
			continue
		}

		err := mergePR(&issue, event.Repo.Owner.GetLogin(), event.Repo.GetName(), gh, commitSHA)
		if err != nil {
			multiErr = multierr.Combine(multiErr, err)
		}
	}

	return multiErr
}

func containsLabel(labels []github.Label, label string) bool {
	for _, l := range labels {
		if *l.Name == label {
			return true
		}
	}
	return false
}

func mergePR(issue *github.Issue, owner, repository string, gh *github.Client, commitSHA string) error {
	if !containsLabel(issue.Labels, approvedLabel) {
		return nil
	}

	pr, _, err := gh.PullRequests.Get(context.Background(), owner, repository, issue.GetNumber())
	if err != nil {
		return errors.Wrapf(err, "failed to get pull request %s", issue.GetHTMLURL())
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

	commits, _, err := gh.PullRequests.ListCommits(context.Background(), owner, repository, issue.GetNumber(), nil)
	if err != nil {
		return errors.Wrapf(err, "failed fetch pull request commits %s", issue.GetHTMLURL())
	}

	fixes := make([]int, 0, 10)
commit_loop:
	for _, commit := range commits {
		msg := commit.Commit.GetMessage()
		issues := issueRegex.FindStringSubmatch(msg) // we only want the first one
		if issues != nil {
			issue, _ := strconv.Atoi(issues[1])
			for _, existing := range fixes {
				if existing == issue {
					continue commit_loop
				}
			}
			fixes = append(fixes, issue)
		}
	}

	comment := ""
	for _, fix := range fixes {
		if comment == "" {
			comment = fmt.Sprintf("Fixes %d", fix)
		} else {
			comment += fmt.Sprintf(", fixes %d", fix)
		}
	}

	_, _, err = gh.PullRequests.CreateComment(context.Background(), owner, repository, *pr.ID, &github.PullRequestComment{
		Body: &comment,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to create pull request comment %s", issue.GetHTMLURL())
	}

	return nil
}
