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

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"go.uber.org/multierr"
)

var (
	dismissReviewHandler Handler = &dismissReview{}

	dismissMessage = "Code changed after review"
)

type dismissReview struct{}

func (h *dismissReview) HandleEvent(w http.ResponseWriter, payload interface{}, ghClientFunc GitHubIntegrationsClientFunc) error {
	event, ok := payload.(*github.PullRequestEvent)
	if !ok {
		return errors.New("wrong event payload type")
	}

	if event.Installation == nil {
		return nil
	}

	if event.Action == nil {
		return nil
	}
	action := *event.Action
	if action != "synchronize" {
		return nil
	}

	gh, err := ghClientFunc(*event.Installation.ID)
	if err != nil {
		return errors.Wrap(err, "failed to create a GitHub client")
	}

	reviews, _, err := gh.PullRequests.ListReviews(context.Background(), *event.Repo.Owner.Login, *event.Repo.Name, *event.PullRequest.Number)
	if err != nil {
		return errors.Wrap(err, "failed to get pull request")
	}

	var multiErr error
	for _, review := range reviews {
		_, _, err = gh.PullRequests.DismissReview(context.Background(), *event.Repo.Owner.Login, *event.Repo.Name, *event.PullRequest.Number, *review.ID, &github.PullRequestReviewDismissalRequest{
			Message: &dismissMessage,
		})
		multiErr = multierr.Combine(multiErr, err)
	}

	if multiErr != nil {
		return errors.Errorf("failed to dismiss reviews: %+v", multiErr)
	}

	return nil
}
