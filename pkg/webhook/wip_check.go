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

	"context"
	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/syndesisio/pure-bot/pkg/config"
	"go.uber.org/zap"
)

const (
	wipContext = "pure-bot/wip"
)

type wip struct{}

func (h *wip) EventTypesHandled() []string {
	return []string{"pull_request"}
}

func (h *wip) HandleEvent(eventObject interface{}, gh *github.Client, config config.RepoConfig, logger *zap.Logger) error {

	event, ok := eventObject.(*github.PullRequestEvent)
	if !ok {
		return errors.New("wrong event eventObject type")
	}

	// Not configured, not check
	if config.WipPatterns == nil && config.Labels.Wip == nil {
		return nil
	}

	if !h.actionTypeRequiresHandling(event.GetAction()) {
		return nil
	}

	if event.PullRequest == nil {
		return nil
	}

	if wipPatternMatched := titleMatchesWipExpression(config, event.PullRequest.GetTitle()); wipPatternMatched != "" {
		return createContextWithSpecifiedStatus(wipContext, pendingStatus, "Pending - title marked as work in progress with '"+wipPatternMatched+"'", event.Repo, event.PullRequest, gh)
	}

	wipLabelFound, err := prIsLabelledWithOneOfSpecifiedLabels(event.PullRequest, config.Labels.Wip, event.Repo, gh)
	if err != nil {
		return errors.Wrapf(err, "failed to check for WIP labels on PR %s", event.PullRequest.GetHTMLURL())
	}
	if wipLabelFound != "" {
		return createContextWithSpecifiedStatus(wipContext, pendingStatus, "Pending - labelled as work in progress with '"+wipLabelFound+"'", event.Repo, event.PullRequest, gh)
	}

	// All good
	return createContextWithSpecifiedStatus(wipContext, successStatus, "OK - this is not a work in progress", event.Repo, event.PullRequest, gh)
}

func titleMatchesWipExpression(config config.RepoConfig, title string) string {
	if len(config.WipPatterns) == 0 {
		return ""
	}
	for _, pattern := range config.WipPatterns {
		var wipRE = regexp.MustCompile(`(?i)\b(?:` + pattern + `)\b`)
		if found := wipRE.FindString(title); found != "" {
			return found
		}
	}
	return ""
}

func prIsLabelledWithOneOfSpecifiedLabels(pr *github.PullRequest, specifiedLabels []string, repo *github.Repository, gh *github.Client) (string, error) {
	labels, _, err := gh.Issues.ListLabelsByIssue(
		context.Background(),
		repo.Owner.GetLogin(),
		repo.GetName(),
		pr.GetNumber(),
		nil,
	)
	if err != nil {
		return "", errors.Wrapf(err, "failed to list labels for PR %s", pr.GetHTMLURL())
	}

	for _, label := range specifiedLabels {
		if labelsContainsLabel(labels, label) {
			return label, nil
		}
	}
	return "", nil
}

func (h *wip) actionTypeRequiresHandling(action string) bool {
	a := strings.ToLower(action)
	return a == "opened" || a == "reopened" || a == "labeled" || a == "unlabeled" || a == "edited" || a == "synchronize"
}
