package webhook

import (
	"strings"
	"testing"
)

func TestIssueRegex(t *testing.T) {

	// single matches
	issues := ExtractIssueNumbers("Fixes #19")
	if len(issues) != 1 {
		t.Error("Invalid number of matches")
	}

	if strings.Compare("19", issues[0]) != 0 {
		t.Error("Invalid value " + issues[0])
	}

	// multiple matches
	issues = ExtractIssueNumbers("Fixes #19, Closes #20")

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
