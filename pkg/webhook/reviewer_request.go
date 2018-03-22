package webhook

import (
	"context"
	"strings"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"github.com/syndesisio/pure-bot/pkg/config"
	"go.uber.org/zap"
)

const (
	prReviewContext = "pure-bot/pr-review"
)

type reviewerRequest struct{}

func (h *reviewerRequest) EventTypesHandled() []string {
	return []string{"pull_request", "pull_request_review"}
}

func (h *reviewerRequest) HandleEvent(eventObject interface{}, gh *github.Client, config config.GitHubAppConfig, logger *zap.Logger) error {

	// Only run when label has been configured
	label := config.ReviewRequestedLabel
	if label == "" {
		return nil
	}

	switch event := eventObject.(type) {
	case *github.PullRequestEvent:
		if err := h.checkLabel(event, gh, label, logger); err != nil {
			return err
		}
		return updateReviewStatus(event.PullRequest, event.Repo, gh, label, logger)
	case *github.PullRequestReviewEvent:
		return updateReviewStatus(event.PullRequest, event.Repo, gh, label, logger)
	default:
		return errors.Errorf("wrong event eventObject type %v", event)
	}
}

func (h *reviewerRequest) checkLabel(event *github.PullRequestEvent, gh *github.Client, label string, logger *zap.Logger) error {
	pr, err := fetchPullRequest(event, gh)
	if err != nil {
		return errors.Wrapf(err, "failed to get PR %s", event.PullRequest.GetHTMLURL())
	}

	switch strings.ToLower(*event.Action) {
	case "review_requested":
		return handleReviewRequested(event, pr, gh, label, logger)
	case "review_request_removed":
		return handleReviewRequestRemoved(event, pr, gh, label, logger)
	default:
		return nil
	}
}

func handleReviewRequested(event *github.PullRequestEvent, pr *github.PullRequest, gh *github.Client, label string,  logger *zap.Logger) error {

	logger.Debug("Handling reviewer assignment", zap.Int64("user", *event.RequestedReviewer.ID))
	if hasLabel(pr, label) {
		logger.Debug("Label " + label + "already exists")
		return nil
	}
	return addLabel(event, gh, label, logger)
}


func handleReviewRequestRemoved(event *github.PullRequestEvent, pr *github.PullRequest, gh *github.Client, label string, logger *zap.Logger) error {
	logger.Debug("Handling reviewer unassignment", zap.Int64("user", *event.RequestedReviewer.ID))

	if ! hasLabel(pr, label) {
		logger.Debug("No label assignment, so nothing to remove")
		return nil
	}

	found, err := hasReviewersRequestedOrAlreadyReviews(event, gh)

	if err != nil {
		return err
	}

	if found {
		logger.Debug("Still reviewers assigned or reviews exist, so not removing label")
		return nil
	}

	return removeLabel(event, gh, label, logger)
}

func updateReviewStatus(pr *github.PullRequest, repo *github.Repository, gh *github.Client, label string, logger *zap.Logger) error {

	if !hasLabel(pr,label)  {
		logger.Debug("No review requested", zap.Bool("pass", true))
		return createContextWithSpecifiedStatus(prReviewContext, successStatus, "OK - No review requested", repo, pr, gh)
	}

	reviews, err := listReviews(pr, repo, gh)
	if err != nil {
		return err
	}

	if len(reviews) == 0 {
		logger.Debug("Review requested but none found", zap.Bool("pass", false))
		return createContextWithSpecifiedStatus(prReviewContext, pendingStatus, "Pending - Reviews requested but none provided", repo, pr, gh)
	}

	logger.Debug("Review requested and reviews found", zap.Bool("pass", true), zap.Int("nrReviews", len(reviews)))
	return createContextWithSpecifiedStatus(prReviewContext, successStatus, "OK - Review requested and at least one provided", repo, pr, gh)
}

// ==============================================================================================

func addLabel(event *github.PullRequestEvent, gh *github.Client, label string, logger *zap.Logger) error {
	owner, repo, prNumber, prURL := event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.PullRequest.GetNumber(), event.PullRequest.GetHTMLURL()

	_, _, err := gh.Issues.AddLabelsToIssue(context.Background(), owner, repo, prNumber, []string{label})
	if err != nil {
		return errors.Wrapf(err, "failed to add label '%s' to PR %s", label, prURL)
	}
	logger.Debug("Added label", zap.Int("pr", prNumber), zap.String("label", label))
	return nil
}

func removeLabel(event *github.PullRequestEvent, gh *github.Client, label string, logger *zap.Logger) error {

	owner, repo, prNumber, prURL := event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.PullRequest.GetNumber(), event.PullRequest.GetHTMLURL()

	_, err := gh.Issues.RemoveLabelForIssue(context.Background(), owner, repo, prNumber, label)
	if err != nil {
		return errors.Wrapf(err, "failed to remove label '%s' from PR %s", label, prURL)
	}
	logger.Debug("Removed label", zap.Int("pr", prNumber), zap.String("label", label))
	return nil
}

func hasLabel(pr *github.PullRequest, label string) bool {
	for _, l := range pr.Labels {
		if *l.Name == label {
			return true
		}
	}
	return false
}

func listReviews(pr *github.PullRequest, repo *github.Repository, gh *github.Client) ([]*github.PullRequestReview, error)  {
	reviews, _, err := gh.PullRequests.ListReviews(
		context.Background(),
		repo.Owner.GetLogin(),
		repo.GetName(),
		pr.GetNumber(),
		nil,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list reviews for PR %s", pr.GetHTMLURL())
	}

	return reviews, nil
}

func listReviewers(pr *github.PullRequest, repo *github.Repository, gh *github.Client) ([]*github.User, error) {
	reviewers, _, err := gh.PullRequests.ListReviewers(
		context.Background(),
		repo.Owner.GetLogin(),
		repo.GetName(),
		pr.GetNumber(),
		nil,
	)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list reviewers for PR %s", pr.GetHTMLURL())
	}

	users := reviewers.Users
	if users == nil {
		users = []*github.User{}
	}
	return users, nil
}


func hasReviewersRequestedOrAlreadyReviews(event *github.PullRequestEvent, gh *github.Client) (bool, error) {
	reviews, err := listReviews(event.PullRequest, event.Repo, gh)
	if err != nil {
		return false, err
	}

	reviewers, err := listReviewers(event.PullRequest, event.Repo, gh)
	if err != nil {
		return false, err
	}

	return len(reviewers) != 0 || len(reviews) != 0, nil
}

func fetchPullRequest(event *github.PullRequestEvent, gh *github.Client) (*github.PullRequest, error) {
	owner, repo, prNumber := event.Repo.Owner.GetLogin(), event.Repo.GetName(), event.PullRequest.GetNumber()
	pr, _, err := gh.PullRequests.Get(context.Background(), owner, repo, prNumber)
	return pr, err
}



