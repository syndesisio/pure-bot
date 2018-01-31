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
	"strings"
	"unicode"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
)

// Returns true if the `comments` slice contains `commentBody`, ignoring whitespace due
// to the way that GitHub returns comment bodies, stripping new lines and potentially other
// whitespace (weird!).
func commentsContainMessage(comments []*github.IssueComment, commentBody string) bool {
	for _, comment := range comments {
		if stripSpaces(comment.GetBody()) == stripSpaces(commentBody) {
			return true
		}
	}
	return false
}

func stripSpaces(str string) string {
	return strings.Map(func(r rune) rune {
		// If the character is a space ('\t', '\n', '\v', '\f', '\r', ' ', U+0085 (NEL), U+00A0 (NBSP)) as per unicode spec, drop it.
		if unicode.IsSpace(r) {
			return -1
		}
		// Else keep it in the string.
		return r
	}, str)
}

func containsLabel(labels []github.Label, label string) bool {
	for _, l := range labels {
		if strings.EqualFold(l.GetName(), label) {
			return true
		}
	}
	return false
}

func labelsContainsLabel(labels []*github.Label, label string) bool {
	for _, l := range labels {
		if strings.EqualFold(l.GetName(), label) {
			return true
		}
	}
	return false
}

func doesPRNeedReview(pr *github.PullRequest, repo *github.Repository, gh *github.Client) (bool, error) {
	reviewers, _, err := gh.PullRequests.ListReviewers(
		context.Background(),
		repo.Owner.GetLogin(),
		repo.GetName(),
		pr.GetNumber(),
		nil,
	)
	if err != nil {
		return false, errors.Wrapf(err, "failed to list reviewers for PR %s", pr.GetHTMLURL())
	}

	reviews, _, err := gh.PullRequests.ListReviews(
		context.Background(),
		repo.Owner.GetLogin(),
		repo.GetName(),
		pr.GetNumber(),
		nil,
	)
	if err != nil {
		return false, errors.Wrapf(err, "failed to list reviews for PR %s", pr.GetHTMLURL())
	}

	// No review requested..
	if len(reviewers.Users) == 0 && len(reviewers.Teams) == 0 {
		return false, nil
	}

	reviewerIds := make(map[int]bool)
	for _, u := range reviewers.Users {
		if u.ID != nil {
			reviewerIds[*u.ID] = true
		}
	}
	for _, t := range reviewers.Teams {
		if t.ID != nil {
			reviewerIds[*t.ID] = true
		}
	}
	for _, r := range reviews {
		if r.User.ID != nil && r.State != nil && *r.State == "APPROVED" {
			if _, ok := reviewerIds[*r.User.ID]; ok {
				// One of our requested reviewers approved.
				return false, nil
			}
		}
	}

	// Still waiting for a requested reviewer to approve.
	return true, nil
}

func prIsLabelledWithOneOfSpecifiedLabels(pr *github.PullRequest, specifiedLabels []string, repo *github.Repository, gh *github.Client) (bool, error) {
	labels, _, err := gh.Issues.ListLabelsByIssue(
		context.Background(),
		repo.Owner.GetLogin(),
		repo.GetName(),
		pr.GetNumber(),
		nil,
	)
	if err != nil {
		return false, errors.Wrapf(err, "failed to list labels for PR %s", pr.GetHTMLURL())
	}

	for _, label := range specifiedLabels {
		if labelsContainsLabel(labels, label) {
			return true, nil
		}
	}
	return false, nil
}

type commitStatus string

var (
	pendingStatus commitStatus = "pending"
	successStatus commitStatus = "success"
)

func createContextWithSpecifiedStatus(contextName string, status commitStatus, description string, event *github.PullRequestEvent, gh *github.Client) error {
	if _, _, err := gh.Repositories.CreateStatus(
		context.Background(),
		event.Repo.Owner.GetLogin(),
		event.Repo.GetName(),
		event.PullRequest.Head.GetSHA(),
		&github.RepoStatus{
			State:       (*string)(&status),
			Context:     &contextName,
			Description: &description,
		},
	); err != nil {
		return errors.Wrapf(
			err,
			"failed to set PR %s context %s to status %s",
			event.PullRequest.GetHTMLURL(),
			contextName,
			status,
		)
	}

	return nil
}
