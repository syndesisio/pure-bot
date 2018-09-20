package webhook

import (
	"context"
	"github.com/go-resty/resty"
	"github.com/google/go-github/github"
	"github.com/syndesisio/pure-bot/pkg/config"
	"go.uber.org/zap"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type boardUpdate struct{}

func (h *boardUpdate) EventTypesHandled() []string {
	return []string{"issues", "pull_request"}
}

type column struct {
	name                string
	id                  string
	isPostMergePipeline bool
}

var stateMapping = map[string]column{}

var postProcessing = make(map[string]column)

var zenHubApi = "https://api.zenhub.io"

var regex = regexp.MustCompile("(?mi)(?:clos(?:e[sd]?|ing)|fix(?:e[sd]|ing))[^\\s]*\\s+#(?P<issue>[0-9]+)")

var doneColumn = &column{}

func (h *boardUpdate) HandleEvent(eventObject interface{}, gh *github.Client, config config.RepoConfig, logger *zap.Logger) error {

	if "<repo>" == config.Board.GithubRepo {
		logger.Warn("Repo not configured, ignore event")
		return nil
	}

	// initialise from config if needed
	if len(stateMapping) == 0 {

		logger.Info("Initialising state mappings ...")

		for _, col := range config.Board.Columns {
			c := column{col.Name, col.Id, col.PostMergePipeline}

			if c.isPostMergePipeline { // the last one flagged as post process will act as doneColumn
				doneColumn = &c
			}

			for _, event := range col.Events {
				logger.Info("Mapping " + event + " to " + col.Name)
				stateMapping[event] = c
			}
		}

		if doneColumn == nil || doneColumn.id == "" {
			logger.Warn("Missing column definition for `Done`")
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

	// post processing (from previous event cycle)
	// takes precedence, the event will no be processed further
	if "issues_closed" == eventKey {
		_, issue_scheduled := postProcessing[number]

		if issue_scheduled {
			delete(postProcessing, number)
			go postProcess(event, gh, config, logger)
			return nil
		}
	} else if "issues_reopened" == eventKey && event.GetIssue().GetLocked() {
		// move
		err := moveIssueOnBoard(config, number, *doneColumn, logger)

		// update progress/* label
		changeProgressLabel(gh, event.Repo, *event.Issue, doneColumn.name)

		if err != nil {
			logger.Error("Post processing failed: Cannot move issue")
		}
	}

	// cleanup post processing markers, but skip actions
	if event.GetIssue().GetLocked() {
		gh.Issues.Unlock(context.Background(), event.Repo.Owner.GetLogin(), event.Repo.GetName(), *event.Issue.Number)
		logger.Debug("Unlock and ignore event for: " + number)
		return nil
	}

	// regular processing
	col, ok := stateMapping[eventKey]
	if ok {
		err := moveIssueOnBoard(config, number, col, logger)

		if nil == err {
			// update progress/* label

			changeProgressLabel(gh, event.Repo, *event.Issue, col.name)
		}

		return err
	} else {
		logger.Debug("Ignore unmapped Issue event: " + eventKey)
	}

	return nil
}

func changeProgressLabel(gh *github.Client, repo *github.Repository, issue github.Issue, newLabel string) {

	labels := []string{"progress/" + newLabel}

	for _, label := range issue.Labels {
		if strings.HasPrefix(*label.Name, "progress/") {
			gh.Issues.RemoveLabelForIssue(context.Background(), repo.Owner.GetLogin(), repo.GetName(),
				issue.GetNumber(), *label.Name)

		}
	}

	gh.Issues.AddLabelsToIssue(context.Background(), repo.Owner.GetLogin(), repo.GetName(),
		issue.GetNumber(), labels)
}

func postProcess(event *github.IssuesEvent, gh *github.Client, config config.RepoConfig, logger *zap.Logger) {

	number := strconv.Itoa(*event.Issue.Number)

	logger.Debug("Handle grace time ... ")
	time.Sleep(10 * time.Second)

	_, e := gh.Issues.Lock(context.Background(), event.Repo.Owner.GetLogin(), event.Repo.GetName(), *event.Issue.Number, &github.LockIssueOptions{LockReason: "resolved"})
	if e != nil {
		logger.Error("Locking issue failed: " + number)
	}

	// re-open
	state := "open"
	_, _, err := gh.Issues.Edit(context.Background(), event.Repo.Owner.GetLogin(), event.Repo.GetName(), *event.Issue.Number, &github.IssueRequest{State: &state})
	if err != nil {
		logger.Error("Post processing failed ")
	}
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

			// schedule post processing if needed
			// closed & merged = comitted
			if "pull_request_closed" == eventKey &&
				event.GetPullRequest().GetMerged() &&
				doneColumn.isPostMergePipeline {

				// schedule completion with next event
				logger.Debug("Schedule post processing for #" + issue)
				postProcessing[issue] = *doneColumn
				continue
			}

			// regular PR processing
			col, ok := stateMapping[eventKey]
			if ok {
				err := moveIssueOnBoard(config, issue, col, logger)

				i, _ := strconv.Atoi(issue)
				item, _, _ := gh.Issues.Get(context.Background(), event.Repo.Owner.GetLogin(), event.Repo.GetName(), i)

				if nil == err {
					changeProgressLabel(gh, event.Repo, *item, col.name)
				}
				return err
			} else {
				logger.Debug("Ignore ummapped PR event: " + eventKey)
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

	logger.Info("Moving #" + issue + " to `" + col.name + "`")

	url := zenHubApi + "/p1/repositories/" + config.Board.GithubRepo + "/issues/" + issue + "/moves"
	response, err := resty.R().
		SetHeader("X-Authentication-Token", config.Board.ZenhubToken).
		SetHeader("Content-Type", "application/json").
		SetBody(`{"pipeline_id":"` + col.id + `", "position": "top"}`).
		Post(url)

	logger.Debug("Zenhub call status: HTTP " + strconv.Itoa(response.StatusCode()) + " from " + url)

	if err != nil {
		return err
	}

	if response.StatusCode() > 400 {
		logger.Warn("Zenhub call unsuccessful: HTTP " + strconv.Itoa(response.StatusCode()) + " from " + url)
	}

	return nil
}
