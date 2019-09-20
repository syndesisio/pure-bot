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
	"regexp"
	"strings"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/syndesisio/pure-bot/pkg/config"
	"go.uber.org/zap"
)

type addReviewUiComment struct{}

func (h *addReviewUiComment) EventTypesHandled() []string {
	return []string{"check_run"}
}

func (h *addReviewUiComment) HandleEvent(eventObject interface{}, gh *github.Client, config config.RepoConfig, logger *zap.Logger) error {
	event, ok := eventObject.(*github.CheckRunEvent)
	if !ok {
		return errors.New("wrong event eventObject type")
	}

	state := strings.ToLower(event.CheckRun.GetConclusion())
	if state != "success" {
		return nil
	}

	owner, repo := event.Repo.Owner.GetLogin(), event.Repo.GetName()

	prNumber := event.CheckRun.PullRequests[0].GetNumber()
	if len(event.CheckRun.PullRequests) == 0 {
		return nil
	}

	summary := event.CheckRun.Output.GetSummary()
	r, _ := regexp.Compile(`\[ui-doc\]\(\D+([0-9]+)`)
	circleCiBuildId := r.FindStringSubmatch(summary)
	if circleCiBuildId == nil {
		return nil
	}

	circleCiUrl := fmt.Sprintf("https://%s-%d-gh.circle-artifacts.com/0/home/circleci/src/app/ui-react/doc/index.html", circleCiBuildId[1], event.Repo.GetID())
	existingComments, _, err := gh.Issues.ListComments(context.Background(), owner, repo, prNumber, &github.IssueListCommentsOptions{
		Sort:      "updated",
		Direction: "desc",
	})
	if err != nil {
		return errors.Wrapf(err, "Failed to retrieve existing comments on PR %s", event.CheckRun.PullRequests[0].GetURL())
	}

	message := fmt.Sprintf("Please review UI for %s [here](%s)", event.CheckRun.GetHeadSHA(), circleCiUrl)

	if commentsContainMessage(existingComments, message) {
		return nil
	}

	_, _, err = gh.Issues.CreateComment(context.Background(), owner, repo, prNumber, &github.IssueComment{
		Body: &message,
	})
	if err != nil {
		return errors.Wrapf(err, "Failed to create comment '%s' on PR %s", message, event.CheckRun.PullRequests[0].GetURL())
	}
}
