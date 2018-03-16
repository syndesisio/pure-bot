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
	"net/http"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
)

const triageLabel = "notif/triage"

var addLabelTriageHandler Handler = &addLabelTriage{}

type addLabelTriage struct{}

func (h *addLabelTriage) HandleEvent(w http.ResponseWriter, eventObject interface{}, ghClientFunc GitHubAppsClientFunc) error {
	event, ok := eventObject.(*github.IssuesEvent)
	if !ok {
		return errors.New("wrong event eventObject type, expecting IssuesEvent")
	}

	if event.Issue == nil {
		return nil
	}

	if *event.Action != "opened" {
		return nil
	}

	owner, repo, issueNumber, issueURL := event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.Issue.GetNumber(), event.Issue.GetHTMLURL()

	gh, err := ghClientFunc(event.Installation.GetID())
	if err != nil {
		return errors.Wrap(err, "failed to create a GitHub client")
	}

	_, _, err = gh.Issues.AddLabelsToIssue(context.Background(), owner, repo, issueNumber, []string{triageLabel})
	if err != nil {
		return errors.Wrapf(err, "failed to add label '%s' to issue %s", triageLabel, issueURL)
	}

	return nil
}
