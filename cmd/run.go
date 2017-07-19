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

package cmd

import (
	gohttp "net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/syndesisio/pure-bot/pkg/http"
	"github.com/syndesisio/pure-bot/pkg/webhook"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs pure-bot",
	Long:  `Runs pure-bot.`,
	Run: func(cmd *cobra.Command, args []string) {
		webhookHandler, err := webhook.NewHTTPHandler(botConfig.Webhook, botConfig.GitHubIntegration, logger.Named("webhook"))
		if err != nil {
			logger.Fatal("failed to create webhook handler", zap.Error(err))
		}

		srv := http.New(botConfig.HTTP, webhookHandler)
		c := make(chan os.Signal, 2)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := srv.Start(); err != nil {
				if errors.Cause(err) != gohttp.ErrServerClosed {
					logger.Fatal("web server failed", zap.Error(err))
				}
			}
		}()
		go func() {
			<-c
			if err := srv.Stop(); err != nil {
				logger.Fatal("failed to stop web server", zap.Error(err))
			}
		}()
		wg.Wait()
	},
}

func init() {
	RootCmd.AddCommand(runCmd)

	runCmd.Flags().String("webhook-secret", "", "Secret to validate incoming webhooks")
	v.BindPFlag("webhook.secret", runCmd.Flags().Lookup("webhook-secret"))
	runCmd.Flags().String("bind-address", "", "Address to bind to")
	v.BindPFlag("http.address", runCmd.Flags().Lookup("bind-address"))
	runCmd.Flags().Int("bind-port", 8080, "Port to bind to")
	v.BindPFlag("http.port", runCmd.Flags().Lookup("bind-port"))
	runCmd.Flags().String("tls-cert", "", "TLS cert file")
	v.BindPFlag("http.tlsCert", runCmd.Flags().Lookup("tls-cert"))
	runCmd.Flags().String("tls-key", "", "TLS key file")
	v.BindPFlag("http.tlsKey", runCmd.Flags().Lookup("tls-key"))
	runCmd.Flags().Int("github-integration-id", 0, "GitHub integration ID")
	v.BindPFlag("github.integrationId", runCmd.Flags().Lookup("github-integration-id"))
	runCmd.Flags().String("github-integration-private-key", "", "GitHub integration private key file")
	v.BindPFlag("github.privateKey", runCmd.Flags().Lookup("github-integration-private-key"))
}
