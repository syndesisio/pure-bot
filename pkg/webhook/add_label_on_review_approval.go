// Copyright © 2017 Red Hat iPaaS Authors
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
)

var addLabelOnReviewApprovalHandler Handler = &addLabelOnReviewApproval{}

const (
	approvedLabel       = "approved"
	approvedReviewState = "approved"
)

type addLabelOnReviewApproval struct{}

func (h *addLabelOnReviewApproval) HandleEvent(w http.ResponseWriter, eventObject interface{}, ghClientFunc GitHubIntegrationsClientFunc) error {
	event, ok := eventObject.(*github.PullRequestReviewEvent)
	if !ok {
		return errors.New("wrong event eventObject type")
	}

	if event.Installation == nil {
		return nil
	}

	if strings.ToLower(event.Review.GetState()) != approvedReviewState {
		return nil
	}

	gh, err := ghClientFunc(event.Installation.GetID())
	if err != nil {
		return errors.Wrap(err, "failed to create a GitHub client")
	}

	owner, repo, prNumber, prURL := event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.PullRequest.GetNumber(), event.PullRequest.GetHTMLURL()

	pr, _, err := gh.Issues.Get(context.Background(), owner, repo, prNumber)
	if err != nil {
		return errors.Wrapf(err, "failed to get PR %s", prURL)
	}
	for _, label := range pr.Labels {
		if *label.Name == approvedLabel {
			return nil
		}
	}

	message := fmt.Sprintf("Pull request [approved](%s) by @%s - applying _%s_ label", event.Review.GetHTMLURL(), event.Review.User.GetLogin(), approvedLabel)
	_, _, err = gh.Issues.CreateComment(context.Background(), owner, repo, prNumber, &github.IssueComment{
		Body: &message,
	})
	if err != nil {
		return errors.Wrapf(err, "failed to add comment '%s' to PR %s", message, prURL)
	}

	_, _, err = gh.Issues.AddLabelsToIssue(context.Background(), owner, repo, prNumber, []string{approvedLabel})
	if err != nil {
		return errors.Wrapf(err, "failed to add label '%s' to PR %s", approvedLabel, prURL)
	}

	return nil
}
