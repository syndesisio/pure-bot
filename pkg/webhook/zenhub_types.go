package webhook

import (
	"errors"
	"github.com/mholt/binding"
	"go.uber.org/zap"
	"net/http"
)

type Zenhub interface {
	GetType() string
	GetIssue() string
}

type ZenhubAction struct {
	Type        string
	IssueNumber string
}

func (z ZenhubAction) GetType() string {
	return z.Type
}

func (z ZenhubAction) GetIssue() string {
	return z.IssueNumber
}

type IssueTransfer struct {
	ZenhubAction
	From string
	To   string
}

// {"plus_ones":[],"pipeline":{"name":"New Issues"},"is_epic":false}
type ZenhubIssue struct {
	Pipeline ZenhubPipeline `json:"pipeline"`
	IsEpic   bool           `json:"is_epic"`
}

type ZenhubPipeline struct {
	Name string `json:"name"`
}

func (i *IssueTransfer) FieldMap(req *http.Request) binding.FieldMap {
	return binding.FieldMap{
		&i.Type:        "type",
		&i.IssueNumber: "issue_number",
		&i.From:        "from_pipeline_name",
		&i.To:          "to_pipeline_name",
	}
}

func ParseZenhub(r *http.Request, logger *zap.Logger) (Zenhub, error) {

	r.ParseForm()

	/*for k, v := range r.Form {
		fmt.Print("key:", k)
		fmt.Println(" => ", strings.Join(v, ""))
	}*/

	var z Zenhub = nil
	switch action := r.Form.Get("type"); action {
	case "issue_transfer":
		issueTransfer := new(IssueTransfer)
		if errs := binding.Bind(r, issueTransfer); errs != nil {
			logger.Error("binding failed", zap.Error(errs))
			break
		}
		z = issueTransfer
	default:
		return nil, errors.New("No binding for " + action)
	}

	return z, nil
}
