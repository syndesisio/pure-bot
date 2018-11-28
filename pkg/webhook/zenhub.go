package webhook

import (
	"fmt"
	"github.com/syndesisio/pure-bot/pkg/config"
	"go.uber.org/zap"
	"log"
	"net/http"
)

func NewZenhubHTTPHandler(cfg config.WebhookConfig, config config.Config, logger *zap.Logger) (http.HandlerFunc, error) {

	return func(w http.ResponseWriter, r *http.Request) {

		//debug(httputil.DumpRequest(r, false))

		zenhub, err := ParseZenhub(r, logger)
		if err != nil {
			logger.Error("Failed to parse zenhub payload", zap.Error(err))
			return
		}

		switch z := zenhub.(type) {
		case *IssueTransfer:
			zenhubMovesIssue(logger, z.IssueNumber, z.From, z.To)
			break
		default:
			logger.Debug("Unregistered zenhub webhook callback type")
			return
		}

	}, nil
}

func zenhubMovesIssue(logger *zap.Logger, issue string, from string, to string) {
	logger.Debug("Zenhub is moving Issue",
		zap.String("id", issue),
		zap.String("from", from),
		zap.String("to", to))
}

func debug(data []byte, err error) {
	if err == nil {
		fmt.Printf("%s\n\n", data)
	} else {
		log.Fatalf("%s\n\n", err)
	}
}
