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
	"regexp"
	"strings"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/syndesisio/pure-bot/pkg/config"
	"go.uber.org/zap"
)

const (
	wipLabel          = "wip"
	doNotMergeLabel   = "do not merge"
	labelStatusPrefix = "status/"

	wipContext = "pure-bot/wip"
)

var wipRE = regexp.MustCompile(`(?i)\b(?:` + wipLabel + `|` + doNotMergeLabel + `)\b`)

type wip struct{}

func (h *wip) EventTypesHandled() []string {
	return []string{"pull_request"}
}

func (h *wip) HandleEvent(eventObject interface{}, gh *github.Client, config config.GitHubAppConfig, logger *zap.Logger) error {

	event, ok := eventObject.(*github.PullRequestEvent)
	if !ok {
		return errors.New("wrong event eventObject type")
	}

	if !h.actionTypeRequiresHandling(event.GetAction()) {
		return nil
	}

	if event.PullRequest == nil {
		return nil
	}

	if wipRE.MatchString(event.PullRequest.GetTitle()) {
		return createContextWithSpecifiedStatus(wipContext, pendingStatus, "Pending - titled as work in progress", event.Repo, event.PullRequest, gh)
	}

	labelledAsWIP, err := prIsLabelledWithOneOfSpecifiedLabels(event.PullRequest, []string{wipLabel, doNotMergeLabel, labelStatusPrefix + wipLabel}, event.Repo, gh)
	if err != nil {
		return errors.Wrapf(err, "failed to check for WIP labels on PR %s", event.PullRequest.GetHTMLURL())
	}
	if labelledAsWIP {
		return createContextWithSpecifiedStatus(wipContext, pendingStatus, "Pending - labelled as work in progress", event.Repo, event.PullRequest, gh)
	}

	// All good
	return createContextWithSpecifiedStatus(wipContext, successStatus, "OK - this is not a work in progress", event.Repo, event.PullRequest, gh)
}


func (h *wip) actionTypeRequiresHandling(action string) bool {
	a := strings.ToLower(action)
	return a == "opened" || a == "reopened" || a == "labeled" || a == "unlabeled" || a == "edited" || a == "synchronize"
}
