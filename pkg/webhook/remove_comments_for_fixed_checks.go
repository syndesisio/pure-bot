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
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/syndesisio/pure-bot/pkg/config"
	"go.uber.org/multierr"
	"net/http"
)

var (
	removeCommentsForFixedChecksHandler Handler = &removeCommentForFixedChecks{}
)

type removeCommentForFixedChecks struct{}

func (h *removeCommentForFixedChecks) HandleEvent(w http.ResponseWriter, eventObject interface{}, ghClientFunc GitHubAppsClientFunc, config config.GitHubAppConfig) error {

	event, ok := eventObject.(*github.StatusEvent)
	if !ok {
		return errors.New("wrong event eventObject type")
	}

	if !checkStatusCheckPreconditions(event) {
		return nil
	}

	state := extractState(event)
	if state != "success" {
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

		comments, err := listIssueComments(gh, &pr)
		if err != nil {
			multiErr = multierr.Combine(multiErr, errors.Wrapf(err, "failed to retrieve existing comments on PR %s", pr.GetHTMLURL()))
			continue
		}

		for _, comment := range comments {
			if containsStatusCommentCheck(comment, event) {
				if err := deleteIssueComment(gh, &pr, comment); err != nil {
					multiErr = multierr.Combine(multiErr, errors.Wrapf(err, "failed to delete comment %s on PR %s", comment.GetID(), pr.GetHTMLURL()))
				}
			}
		}
	}

	return multiErr
}
