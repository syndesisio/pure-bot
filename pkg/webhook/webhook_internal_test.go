package webhook

import (
	"strings"
	"testing"
)

func TestIssueRegex(t *testing.T) {

	// key words
	issues := []string{}

	extractIssueNumbers(&issues, "Fixes #19, Fixed #19, Fixing #19")
	if len(issues) != 3 {
		t.Error("Invalid number of matches")
	}

	issues = issues[:0]
	extractIssueNumbers(&issues,"Closed #19, Closed #19, Closing #19")
	if len(issues) != 3 {
		t.Error("Invalid number of matches")
	}

	// single matches
	issues = issues[:0]
	extractIssueNumbers(&issues,"Fixes #19")
	if len(issues) != 1 {
		t.Error("Invalid number of matches")
	}

	if strings.Compare("19", issues[0]) != 0 {
		t.Error("Invalid value " + issues[0])
	}

	// multiple matches
	issues = issues[:0]
	extractIssueNumbers(&issues,"Fixes #19, Closes #20")

	if len(issues) != 2 {
		t.Error("Invalid number of matches")
	}

	if strings.Compare("19", issues[0]) != 0 {
		t.Error("Invalid value " + issues[0])
	}

	if strings.Compare("20", issues[1]) != 0 {
		t.Error("Invalid value " + issues[1])
	}

}
