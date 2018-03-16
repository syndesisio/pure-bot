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
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"
	"go.uber.org/multierr"

	"github.com/syndesisio/pure-bot/pkg/config"
	"github.com/syndesisio/pure-bot/pkg/github/apps"
	"go.uber.org/zap"
)

type GitHubAppsClientFunc func(installationID int) (*github.Client, error)

type Handler interface {
	HandleEvent(w http.ResponseWriter, eventObject interface{}, f GitHubAppsClientFunc) error
}

var (
	handlers = map[string][]Handler{
		//"pull_request": {dismissReviewHandler},
		"issues":              {addLabelTriageHandler},
		"pull_request_review": {addLabelOnReviewApprovalHandler, autoMergeHandler},
		"pull_request":        {autoMergeHandler, wipHandler},
		"status":              {autoMergeHandler},
	}
)

func newGitHubClientFunc(appID int, privateKeyFile string) (GitHubAppsClientFunc, error) {
	key, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read private key file")
	}

	return func(installationID int) (*github.Client, error) {
		return apps.Client(appID, installationID, key)
	}, nil
}

func NewHTTPHandler(cfg config.WebhookConfig, appCfg config.GitHubAppConfig, logger *zap.Logger) (http.HandlerFunc, error) {
	newGHClientF, err := newGitHubClientFunc(appCfg.AppID, appCfg.PrivateKeyFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create apps client func")
	}
	webhookSecret := ([]byte)(cfg.Secret)
	return func(w http.ResponseWriter, r *http.Request) {
		var payload []byte
		if cfg.Secret != "" {
			pl, err := github.ValidatePayload(r, webhookSecret)
			if err != nil {
				logger.Error("webhook payload validation failed", zap.Error(err))
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			payload = pl
		} else {
			pl, err := ioutil.ReadAll(r.Body)
			if err != nil {
				logger.Error("failed to read paylad", zap.Error(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			payload = pl
		}
		messageType := github.WebHookType(r)
		event, err := github.ParseWebHook(messageType, payload)
		for _, wh := range handlers[messageType] {
			err = multierr.Combine(err, wh.HandleEvent(w, event, newGHClientF))
		}
		if err != nil {
			logger.Error("webhook handler failed", zap.String("error", fmt.Sprintf("%+v", err)))
			w.WriteHeader(http.StatusInternalServerError)
		}
	}, nil
}
