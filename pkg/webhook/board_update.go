package webhook

import (
	"context"
	"fmt"
	"github.com/go-resty/resty"
	"github.com/google/go-github/github"
	"github.com/syndesisio/pure-bot/pkg/config"
	"go.uber.org/zap"
	"regexp"
	"strconv"
)

type boardUpdate struct{}

func (h *boardUpdate) EventTypesHandled() []string {
	return []string{"issues", "pull_request"}
}

type column struct {
	name string
	id   string
}

var stateMapping = map[string]column{}

var zenHubApi = "https://api.zenhub.io"

var regex = regexp.MustCompile("(?mi)(?:clos(?:e[sd]?|ing)|fix(?:e[sd]|ing))[^\\s]*\\s+#(?P<issue>[0-9]+)")

func (h *boardUpdate) HandleEvent(eventObject interface{}, gh *github.Client, config config.RepoConfig, logger *zap.Logger) error {

	// initialise from config if needed
	if len(stateMapping) == 0 {
		for _, col := range config.Board.Columns {
			c := column{col.Name, col.Id}
			for _, event := range col.Events {
				logger.Info("Mapping " + event + " to " + col.Name)
				stateMapping[event] = c
			}
		}
	}

	switch event := eventObject.(type) {
	case *github.IssuesEvent:
		return h.handleIssuesEvent(event, gh, config, logger)
	case *github.PullRequestEvent:
		return h.handlePullRequestEvent(event, gh, config, logger)
	default:
		return nil
	}
}

func (h *boardUpdate) handleIssuesEvent(event *github.IssuesEvent, gh *github.Client, config config.RepoConfig, logger *zap.Logger) error {

	var messageType = "issues"

	number := strconv.Itoa(*event.Issue.Number)
	logger.Debug("Handling issuesEvent for issue " + number)
	logger.Debug("Issue Action: " + *event.Action)

	eventKey := messageType + "_" + *event.Action
	col, ok := stateMapping[eventKey]
	if ok {
		return moveIssueOnBoard(config, number, col, logger)
	} else {
		logger.Debug("Ignore unmapped event: " + eventKey)
	}

	return nil
}

func (h *boardUpdate) handlePullRequestEvent(event *github.PullRequestEvent, gh *github.Client, config config.RepoConfig, logger *zap.Logger) error {

	var messageType = "pull_request"

	prNumber := strconv.Itoa(*event.PullRequest.Number)
	logger.Info("Handling pullReqestEvent for PR " + prNumber)
	logger.Info("PR Action: " + *event.Action)

	commits, _, err := gh.PullRequests.ListCommits(context.Background(), event.Repo.Owner.GetLogin(), event.Repo.GetName(),
		*event.PullRequest.Number, nil)

	if err != nil {
		logger.Error("Failed to retrieve commits")
		return nil
	}

	for _, commit := range commits {

		message := *commit.Commit.Message
		logger.Debug("Processing commit message: " + message)

		// get issue id from commit message

		match := regex.Match([]byte(message))
		logger.Debug("regex matches: " + strconv.FormatBool(match))

		issues := extractIssueNumbers(message)

		for _, issue := range issues {
			eventKey := messageType + "_" + *event.Action
			col, ok := stateMapping[eventKey]
			if ok {
				return moveIssueOnBoard(config, issue, col, logger)
			} else {
				logger.Debug("Ignore ummapped event: " + eventKey)
			}
		}
	}

	return nil
}

func extractIssueNumbers(commitMessage string) []string {
	groupNames := regex.SubexpNames()
	issues := []string{}
	for _, match := range regex.FindAllStringSubmatch(commitMessage, -1) {
		for groupIdx, _ := range match {
			name := groupNames[groupIdx]

			if name == "issue" {
				issues = append(issues, match[1])
			}
		}
	}

	return issues
}

func moveIssueOnBoard(config config.RepoConfig, issue string, col column, logger *zap.Logger) error {

	fmt.Println("Moving #" + issue + " to `" + col.name + "`")

	url := zenHubApi + "/p1/repositories/" + config.Board.GithubRepo + "/issues/" + issue + "/moves"
	response, err := resty.R().
		SetHeader("X-Authentication-Token", config.Board.ZenhubToken).
		SetHeader("Content-Type", "application/json").
		SetBody(`{"pipeline_id":"` + col.id + `", "position": "top"}`).
		Post(url)

	if err != nil {
		return err
	}

	if response.StatusCode() > 400 {
		logger.Warn("API call unsuccessful: HTTP " + strconv.Itoa(response.StatusCode()) + " from " + url)
	}

	return nil
}
