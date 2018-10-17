package webhook

import (
	"strings"
	"testing"
	"strconv"
)

func TestIssueRegex(t *testing.T) {

	// key words
	issues := []string{}

	extractIssueNumbers(&issues, "Fixes #19, Fixed #19, Fixing #19")
	if len(issues) != 3 {
		t.Error("Invalid number of matches")
	}

	issues = issues[:0]
	extractIssueNumbers(&issues, "Closed #19, Closed #19, Closing #19")
	if len(issues) != 3 {
		t.Error("Invalid number of matches")
	}

	// single matches
	issues = issues[:0]
	extractIssueNumbers(&issues, "Fixes #19")
	if len(issues) != 1 {
		t.Error("Invalid number of matches")
	}

	if strings.Compare("19", issues[0]) != 0 {
		t.Error("Invalid value " + issues[0])
	}

	// multiple matches
	issues = issues[:0]
	extractIssueNumbers(&issues, "Fixes #19, Closes #20")

	if len(issues) != 2 {
		t.Error("Invalid number of matches")
	}

	if strings.Compare("19", issues[0]) != 0 {
		t.Error("Invalid value " + issues[0])
	}

	if strings.Compare("20", issues[1]) != 0 {
		t.Error("Invalid value " + issues[1])
	}

	// issue #56

	s := "Fixes #3803 \r\nFixes #3814 \r\n\r\nNotes: This changes the input and output datashape by using only a subset of the fields of an event"

	issues = issues[:0]
	extractIssueNumbers(&issues, s)

	if len(issues) != 2 {
		t.Error("Invalid number of matches: " + strconv.Itoa(len(issues)))
	}

	if strings.Compare("3803", issues[0]) != 0 {
		t.Error("Invalid value " + issues[0])
	}

	if strings.Compare("3814", issues[1]) != 0 {
		t.Error("Invalid value " + issues[1])
	}

	s = "Fixes https://github.com/syndesisio/syndesis/issues/3405\r\n\r\nIn the PR review I'd like to understand what the difference is between the deploymentVersion and the version. The deploymentVersion is undefined, but the version contains the correct version. Using that instead fixes the issue but I'm unclear why we have two pieces of data that seem to contain the same information. Maybe I'm missing something, or maybe there is larger issue.\r\n\r\n@gashcrumb, @seanforyou23 maybe one of you can shed a light on this?"
	issues = issues[:0]
	extractIssueNumbers(&issues, s)

	if len(issues) != 1 {
		t.Error("Invalid number of matches: " + strconv.Itoa(len(issues)))
	}

	if strings.Compare("3405", issues[0]) != 0 {
		t.Error("Invalid value " + issues[0])
	}

}
