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
	"go.uber.org/multierr"
)

var (
	failedStatusCheckAddCommentHandler Handler = &failedStatusCheckAddComment{}
)

type failedStatusCheckAddComment struct{}

func (h *failedStatusCheckAddComment) HandleEvent(w http.ResponseWriter, eventObject interface{}, ghClientFunc GitHubIntegrationsClientFunc) error {
	event, ok := eventObject.(*github.StatusEvent)
	if !ok {
		return errors.New("wrong event eventObject type")
	}

	state := strings.ToLower(event.GetState())
	if state == "pending" || state == "success" {
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
	if err != nil {
		return errors.Wrapf(err, "failed to find PR using query %s", query)
	}

	owner, repo := event.Repo.Owner.GetLogin(), event.Repo.GetName()

	var multiErr error
	for _, issue := range searchResult.Issues {
		if issue.PullRequestLinks == nil {
			continue
		}

		prNumber, prURL := issue.GetNumber(), issue.GetHTMLURL()

		message := fmt.Sprintf("Status check _%s_ returned **%s**.", event.GetContext(), state)
		if event.GetTargetURL() != "" {
			message += fmt.Sprintf(" See %s for more details.", event.GetTargetURL())
		}

		existingComments, _, err := gh.Issues.ListComments(context.Background(), owner, repo, prNumber, &github.IssueListCommentsOptions{
			Sort:      "updated",
			Direction: "desc",
		})
		if err != nil {
			return errors.Wrapf(err, "failed to retrieve existing comments on PR %s", prURL)
		}

		if commentsContainMessage(existingComments, message) {
			continue
		}

		_, _, err = gh.Issues.CreateComment(context.Background(), owner, repo, prNumber, &github.IssueComment{
			Body: &message,
		})
		if err != nil {
			multiErr = multierr.Combine(multiErr, err)
		}
	}

	return multiErr
}
