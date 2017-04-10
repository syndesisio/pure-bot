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
	"net/http"

	"fmt"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
	"strings"
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
	var multiErr error
	for _, issue := range searchResult.Issues {
		if issue.PullRequestLinks == nil {
			continue
		}

		msg := fmt.Sprintf("Status check _%s_ returned **%s**.", event.GetContext(), state)
		if event.GetTargetURL() != "" {
			msg += fmt.Sprintf(" See %s for more details.", event.GetTargetURL())
		}
		_, _, err := gh.Issues.CreateComment(context.Background(), event.Repo.Owner.GetLogin(), event.Repo.GetName(), issue.GetNumber(), &github.IssueComment{
			Body: &msg,
		})
		if err != nil {
			multiErr = multierr.Combine(multiErr, err)
		}
	}

	return multiErr
}
