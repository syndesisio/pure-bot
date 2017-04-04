// Copyright © 2017 Red Hat iPaaS Authors
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
	"flag"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/redhat-ipaas/pure-bot/pkg/version"
	"github.com/redhat-ipaas/pure-bot/pkg/config"
)

var (
	cfgFile string
	logLevel = zapcore.InfoLevel
	logger *zap.Logger
	botConfig = config.NewWithDefaults()
	v = viper.New()
)

var RootCmd = &cobra.Command{
	Use:   "pure-bot",
	Short: "PuRe Bot - pull request bot",
	Long: `PuRe Bot - pull request bot.`,
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		logger.Fatal("Command failed", zap.Error(err))
	}
}

func init() {
	cobra.OnInitialize(initLogging, printVersion, initConfig)

	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.pure-bot.yaml)")
	RootCmd.PersistentFlags().AddGoFlag(&flag.Flag{
		Name: "log-level",
		Value: &logLevel,
		DefValue: "info",
		Usage: "log level",
	})
}

func initLogging() {
	logConfig := zap.NewProductionConfig()
	logConfig.Level.SetLevel(logLevel)
	logger, _ = logConfig.Build()
}

func printVersion() {
	logger.Info("Build info", zap.String("version", version.AppVersion))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" { // enable ability to specify config file via flag
		v.SetConfigFile(cfgFile)
	}

	v.SetConfigName(".pure-bot") // name of config file (without extension)
	v.AddConfigPath("$HOME")  // adding home directory as first search path
	v.SetEnvPrefix("PUREBOT") // Set env prefix
	v.AutomaticEnv()          // read in environment variables that match

	err := v.ReadInConfig()
	if err != nil {
		if _, ok := err.(viper.ConfigParseError); ok {
			logger.Fatal("Failed to parse config file", zap.Error(err))
		}
		logger.Debug("No config file found")
	} else {
		logger.Info("Using config file", zap.String("file", v.ConfigFileUsed()))
	}

	if err := v.UnmarshalExact(&botConfig); err != nil {
		logger.Fatal("Failed to unmarshal config file", zap.Error(err))
	}

	logger.Debug("Using config", zap.Reflect("config", botConfig))
}
