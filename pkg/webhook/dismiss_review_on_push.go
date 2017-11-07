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
	"strings"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

var (
	dismissReviewHandler Handler = &dismissReview{}

	dismissMessage = "Code changed after review"
)

type dismissReview struct{}

func (h *dismissReview) HandleEvent(w http.ResponseWriter, eventObject interface{}, ghClientFunc GitHubAppsClientFunc) error {
	event, ok := eventObject.(*github.PullRequestEvent)
	if !ok {
		return errors.New("wrong event eventObject type")
	}

	if event.Installation == nil {
		return nil
	}

	if strings.ToLower(event.GetAction()) != "synchronize" {
		return nil
	}

	gh, err := ghClientFunc(event.Installation.GetID())
	if err != nil {
		return errors.Wrap(err, "failed to create a GitHub client")
	}

	reviews, _, err := gh.PullRequests.ListReviews(context.Background(), event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.PullRequest.GetNumber(), &github.ListOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to get pull request")
	}

	var multiErr error
	for _, review := range reviews {
		_, _, err = gh.PullRequests.DismissReview(context.Background(), event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.PullRequest.GetNumber(), review.GetID(), &github.PullRequestReviewDismissalRequest{
			Message: &dismissMessage,
		})
		multiErr = multierr.Combine(multiErr, err)
	}

	if multiErr != nil {
		return errors.Errorf("failed to dismiss reviews: %+v", multiErr)
	}

	return nil
}
