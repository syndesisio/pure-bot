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

	"github.com/imdario/mergo"
	"github.com/syndesisio/pure-bot/pkg/config"
	"github.com/syndesisio/pure-bot/pkg/github/apps"
	"go.uber.org/zap"
	"reflect"
)

type GitHubAppsClientFunc func(installationID int) (*github.Client, error)

type Handler interface {
	HandleEvent(eventObject interface{}, client *github.Client, config config.RepoConfig, logger *zap.Logger) error

	EventTypesHandled() []string
}

var (
	// List of all handlers used
	handlers = []Handler{
		&addLabelOnReviewApproval{},
		&reviewerRequest{},
		&autoMerger{},
		&wip{},
		&newIssueLabel{},
		//		&dismissReview{},
		//		&failedStatusCheckAddComment{},
	}
	handlerMap map[string][]Handler
)

func init() {
	handlerMap = make(map[string][]Handler)

	// Register handlers per event type
	for _, handler := range handlers {
		for _, eventType := range handler.EventTypesHandled() {
			if handlerMap[eventType] == nil {
				handlerMap[eventType] = make([]Handler, 0)
			}
			handlerMap[eventType] = append(handlerMap[eventType], handler)
		}
	}
}

func newGitHubClient(appID int64, privateKeyFile string, installationID int64) (*github.Client, error) {
	key, err := ioutil.ReadFile(privateKeyFile)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read private key file")
	}

	return apps.Client(appID, installationID, key)
}

func createClient(appCfg config.GitHubAppConfig, event interface{}) (*github.Client, error) {

	val := reflect.Indirect(reflect.ValueOf(event))
	// Find installation via inspection
	if _, found := val.Type().FieldByName("Installation"); !found {
		return nil, errors.New("event does not contain an installation ID, cannot create github client")
	}
	installation := val.FieldByName("Installation").Interface().(*github.Installation)
	if installation == nil {
		return nil, errors.Errorf("no installation in event %v found, so no GitHub client could be created", event)
	}
	client, err := newGitHubClient(appCfg.AppID, appCfg.PrivateKeyFile, *installation.ID)
	if err != nil {
		return nil, errors.New("cannot create github client")
	}
	return client, nil
}

func NewHTTPHandler(cfg config.WebhookConfig, config config.Config, logger *zap.Logger) (http.HandlerFunc, error) {
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
				logger.Error("failed to read payload", zap.Error(err))
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			payload = pl
		}

		messageType := github.WebHookType(r)
		event, err := github.ParseWebHook(messageType, payload)
		if err != nil {
			logger.Error("failed to parse webhook", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		repo := extractRepository(event)
		repoConfig := extractRepoConfigWithDefaults(repo, config)
		if repo != nil {
			logger.Debug("Processing event", zap.String("repo", *repo.Name))
		}
		if repoConfig.Disabled {
			logger.Debug("Disabled by configuration", zap.String("repo", *repo.Name))
			return
		}

		client, err := createClient(config.GitHubApp, event)
		if err != nil {
			logger.Error("failed to create GitHub client", zap.Error(err))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// ========================================================================
		// Call all handlers
		for _, wh := range handlerMap[messageType] {
			logger.Debug("call handler", zap.String("type", messageType), zap.String("handler", reflect.TypeOf(wh).String()))
			err = multierr.Combine(err, wh.HandleEvent(event, client, *repoConfig, logger))
		}

		// =========================================================================

		if err != nil {
			logger.Error("webhook handler failed", zap.String("error", fmt.Sprintf("%+v", err)))
			w.WriteHeader(http.StatusInternalServerError)
		}
	}, nil
}

func extractRepoConfigWithDefaults(repo *github.Repository, fullConfig config.Config) *config.RepoConfig {

	ret := &config.RepoConfig{
		Disabled: false,
	}

	if repo == nil {
		return ret
	}

	var repoSpecificConfig config.RepoConfig
	if len(fullConfig.Repos) > 0 {
		repoSpecificConfig = fullConfig.Repos[*repo.Name]
	}

	mergo.Merge(ret, fullConfig.DefaultRepo, mergo.WithOverride)
	mergo.Merge(ret, repoSpecificConfig, mergo.WithOverride)
	return ret
}

func extractRepository(event interface{}) *github.Repository {
	val := reflect.Indirect(reflect.ValueOf(event))
	if _, found := val.Type().FieldByName("Repo"); !found {
		return nil
	}
	return val.FieldByName("Repo").Interface().(*github.Repository)
}
