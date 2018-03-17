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
	"fmt"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/syndesisio/pure-bot/pkg/config"
	"go.uber.org/multierr"
	"strings"
)

var (
	failedStatusCheckAddCommentHandler Handler = &failedStatusCheckAddComment{}
)

type failedStatusCheckAddComment struct{}

func (h *failedStatusCheckAddComment) HandleEvent(w http.ResponseWriter, eventObject interface{}, ghClientFunc GitHubAppsClientFunc, config config.GitHubAppConfig) error {
	event, ok := eventObject.(*github.StatusEvent)
	if !ok {
		return errors.New("wrong event eventObject type")
	}

	if !checkStatusCheckPreconditions(event) {
		return nil
	}

	if event.Installation == nil {
		return nil
	}

	state := extractState(event)
	if state == "pending" || state == "success" {
		return nil
	}

	// CodeCov and Codacy already comment so let's just ignore those status checks... Noisy otherwise...
	statusContext := event.GetContext()
	if strings.HasPrefix(statusContext, "codecov/") || strings.HasPrefix(statusContext, "codacy/") {
		return nil
	}

	gh, err := ghClientFunc(event.Installation.GetID())
	if err != nil {
		return errors.Wrap(err, "failed to create a GitHub client")
	}

	searchResult, err := searchPullRequestsForCommit(gh, event)
	if err != nil {
		return err
	}

	var multiErr error
	for _, pr := range searchResult.Issues {
		if pr.PullRequestLinks == nil {
			continue
		}
		existingComments, err := listIssueComments(gh, &pr)
		if err != nil {
			multiErr = multierr.Combine(multiErr, errors.Wrapf(err, "failed to retrieve existing comments on PR %s", pr.GetHTMLURL()))
			continue
		}

		message := createMessage(event, state)
		if commentsContainMessage(existingComments, message) {
			continue
		}

		err = createIssueComment(gh, &pr, message)
		if err != nil {
			multiErr = multierr.Combine(multiErr, errors.Wrapf(err, "failed to create comment on PR %s", pr.GetHTMLURL()))
			continue
		}
	}

	return multiErr
}

func createMessage(event *github.StatusEvent, state string) string {
	message := fmt.Sprintf("%s\n:warning: Status check _%s_ returned **%s**.", getStatusCheckMarker(event), event.GetContext(), state)
	if event.GetDescription() != "" {
		message += "\n\n" + event.GetDescription()
	}
	if event.GetTargetURL() != "" {
		message += fmt.Sprintf("\n\nSee %s for more details.", event.GetTargetURL())
	}
	return message
}
