// @license
// Copyright 2021 Dynatrace LLC
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package download

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/internal/log"
	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/internal/version"
	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/pkg/client"
	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/pkg/manifest"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/Dynatrace/Dynatrace-Config-Manager/one-topology/cmd/monaco/runner/completion"
)

func GetDownloadCommand(fs afero.Fs, command Command) (downloadCmd *cobra.Command) {

	downloadCmd = &cobra.Command{
		Use:   "download",
		Short: "Download configuration from Dynatrace",
		Long: `Download configuration from Dynatrace

Either downloading based on an existing manifest, or by defining environment URL and API token via the 'direct' sub-command.

To download entities, use download entities`,
		Example: `- monaco download manifest manifest.yaml some_environment_from_manifest
- monaco download direct https://environment.live.dynatrace.com API_TOKEN_ENV_VAR_NAME`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("'direct' or 'manifest' sub-command is required")
		},
	}

	getDownloadConfigsCommand(fs, command, downloadCmd)
	getDownloadEntitiesCommand(fs, command, downloadCmd)

	return downloadCmd
}

func getDownloadConfigsCommand(fs afero.Fs, command Command, downloadCmd *cobra.Command) {
	var project, outputFolder string
	var forceOverwrite bool
	var specificApis []string
	var specificSettings []string
	var onlyAPIs bool
	var onlySettings bool
	var flatDump bool

	manifestDownloadCmd := &cobra.Command{
		Use:     "manifest [manifest file] [environment to download]",
		Aliases: []string{"m"},
		Short:   "Download configuration from Dynatrace via a manifest file",
		Example: `monaco download manifest.yaml some_environment_from_manifest`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 || args[0] == "" || args[1] == "" {
				return fmt.Errorf(`manifest and environment name have to be provided as positional arguments`)
			}
			return nil
		},
		ValidArgsFunction: completion.DownloadManifestCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			m := args[0]
			specificEnvironment := args[1]
			options := manifestDownloadOptions{
				manifestFile:            m,
				specificEnvironmentName: specificEnvironment,
				downloadCommandOptions: downloadCommandOptions{
					downloadCommandOptionsShared: downloadCommandOptionsShared{
						projectName:    project,
						outputFolder:   outputFolder,
						forceOverwrite: forceOverwrite,
					},
					specificAPIs:    specificApis,
					specificSchemas: specificSettings,
					onlyAPIs:        onlyAPIs,
					onlySettings:    onlySettings,
					flatDump:        flatDump,
				},
			}

			return command.DownloadConfigsBasedOnManifest(fs, options)
		},
	}

	directDownloadCmd := &cobra.Command{
		Use:     "direct [URL] [TOKEN_NAME]",
		Aliases: []string{"d"},
		Short:   "Download configuration from a Dynatrace environment specified on the command line",
		Example: `monaco download direct https://environment.live.dynatrace.com API_TOKEN_ENV_VAR_NAME`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 || args[0] == "" || args[1] == "" {
				return fmt.Errorf(`url and token have to be provided as positional argument`)
			}
			return nil
		},
		ValidArgsFunction: completion.DownloadDirectCompletion,
		PreRun: func(cmd *cobra.Command, args []string) {
			serverVersion, err := client.GetDynatraceVersion(client.NewTokenAuthClient(os.Getenv(args[1])), args[0])
			if err != nil {
				log.Error("Unable to determine server version %q: %w", args[0], err)
			} else if serverVersion.SmallerThan(version.Version{Major: 1, Minor: 262}) {
				logUploadToSameEnvironmentWarning()
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			tokenEnvVar := args[1]
			options := directDownloadOptions{
				environmentUrl: url,
				envVarName:     tokenEnvVar,
				downloadCommandOptions: downloadCommandOptions{
					downloadCommandOptionsShared: downloadCommandOptionsShared{
						projectName:    project,
						outputFolder:   outputFolder,
						forceOverwrite: forceOverwrite,
					},
					specificAPIs:    specificApis,
					specificSchemas: specificSettings,
					onlyAPIs:        onlyAPIs,
					onlySettings:    onlySettings,
					flatDump:        flatDump,
				},
			}
			return command.DownloadConfigs(fs, options)

		},
	}

	setupSharedConfigsFlags(manifestDownloadCmd, &project, &outputFolder, &forceOverwrite, &specificApis, &specificSettings, &onlyAPIs, &onlySettings, &flatDump)
	setupSharedConfigsFlags(directDownloadCmd, &project, &outputFolder, &forceOverwrite, &specificApis, &specificSettings, &onlyAPIs, &onlySettings, &flatDump)

	downloadCmd.AddCommand(manifestDownloadCmd)
	downloadCmd.AddCommand(directDownloadCmd)
}

func getDownloadEntitiesCommand(fs afero.Fs, command Command, downloadCmd *cobra.Command) {
	var project, outputFolder string
	var forceOverwrite bool
	var specificEntitiesTypes []string
	var timeFromMinutes int
	var timeToMinutes int
	var entityPageSize int

	downloadEntitiesCmd := &cobra.Command{
		Use:   "entities",
		Short: "Download entities configuration from Dynatrace",
		Long: `Download entities configuration from Dynatrace

Either downloading based on an existing manifest, or by defining environment URL and API token via the 'direct' sub-command.`,
		Example: `- monaco download entities manifest manifest.yaml some_environment_from_manifest
- monaco download entities direct https://environment.live.dynatrace.com API_TOKEN_ENV_VAR_NAME`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("'direct' or 'manifest' sub-command is required")
		},
	}

	manifestDownloadCmd := &cobra.Command{
		Use:     "manifest [manifest file] [environment to download]",
		Aliases: []string{"m"},
		Short:   "Download configuration from Dynatrace via a manifest file",
		Example: `monaco download entities manifest.yaml some_environment_from_manifest`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 || args[0] == "" || args[1] == "" {
				return fmt.Errorf(`manifest and environment name have to be provided as positional arguments`)
			}
			return nil
		},
		ValidArgsFunction: completion.DownloadManifestCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			m := args[0]
			specificEnvironment := args[1]
			options := entitiesManifestDownloadOptions{
				manifestFile:            m,
				specificEnvironmentName: specificEnvironment,
				entitiesDownloadCommandOptions: entitiesDownloadCommandOptions{
					downloadCommandOptionsShared: downloadCommandOptionsShared{
						projectName:    project,
						outputFolder:   outputFolder,
						forceOverwrite: forceOverwrite,
					},
					specificEntitiesTypes: specificEntitiesTypes,
					listEntitiesOptions: listEntitiesOptions{
						timeFromMinutes: timeFromMinutes,
						timeToMinutes:   timeToMinutes,
						entityPageSize:  entityPageSize,
					},
				},
			}
			return command.DownloadEntitiesBasedOnManifest(fs, options)
		},
	}

	directDownloadCmd := &cobra.Command{
		Use:     "direct [URL] [TOKEN_NAME]",
		Aliases: []string{"d"},
		Short:   "Download configuration from a Dynatrace environment specified on the command line",
		Example: `monaco download entities direct https://environment.live.dynatrace.com API_TOKEN_ENV_VAR_NAME`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 || args[0] == "" || args[1] == "" {
				return fmt.Errorf(`url and token have to be provided as positional argument`)
			}
			return nil
		},
		ValidArgsFunction: completion.DownloadDirectCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			url := args[0]
			tokenEnvVar := args[1]
			options := entitiesDirectDownloadOptions{
				environmentUrl: url,
				envVarName:     tokenEnvVar,
				entitiesDownloadCommandOptions: entitiesDownloadCommandOptions{
					downloadCommandOptionsShared: downloadCommandOptionsShared{
						projectName:    project,
						outputFolder:   outputFolder,
						forceOverwrite: forceOverwrite,
					},
					specificEntitiesTypes: specificEntitiesTypes,
					listEntitiesOptions: listEntitiesOptions{
						timeFromMinutes: timeFromMinutes,
						timeToMinutes:   timeToMinutes,
						entityPageSize:  entityPageSize,
					},
				},
			}
			return command.DownloadEntities(fs, options)

		},
	}

	setupSharedEntitiesFlags(manifestDownloadCmd, &project, &outputFolder, &forceOverwrite, &specificEntitiesTypes, &timeFromMinutes, &timeToMinutes, &entityPageSize)
	setupSharedEntitiesFlags(directDownloadCmd, &project, &outputFolder, &forceOverwrite, &specificEntitiesTypes, &timeFromMinutes, &timeToMinutes, &entityPageSize)

	downloadEntitiesCmd.AddCommand(manifestDownloadCmd)
	downloadEntitiesCmd.AddCommand(directDownloadCmd)

	downloadCmd.AddCommand(downloadEntitiesCmd)
}

func setupSharedConfigsFlags(cmd *cobra.Command, project, outputFolder *string, forceOverwrite *bool, specificApis *[]string, specificSettings *[]string, onlyAPIs, onlySettings *bool, flatDump *bool) {
	setupSharedFlags(cmd, project, outputFolder, forceOverwrite)
	// flags always available
	cmd.Flags().StringSliceVarP(specificApis, "api", "a", make([]string, 0), "One or more APIs to download (flag can be repeated or value defined as comma-separated list)")
	cmd.Flags().StringSliceVarP(specificSettings, "settings-schema", "s", make([]string, 0), "One or more settings 2.0 schemas to download (flag can be repeated or value defined as comma-separated list)")
	cmd.Flags().BoolVar(onlyAPIs, "only-apis", false, "Only download config APIs, skip downloading settings 2.0 objects")
	cmd.Flags().BoolVar(onlySettings, "only-settings", false, "Only download settings 2.0 objects, skip downloading config APIs")
	cmd.Flags().BoolVar(flatDump, "flat-dump", false, "Dump results in a big unformatted json array for processing/cache purposes")
	cmd.MarkFlagsMutuallyExclusive("settings-schema", "only-apis")
	cmd.MarkFlagsMutuallyExclusive("api", "only-settings")
	cmd.MarkFlagsMutuallyExclusive("only-apis", "only-settings")

	err := cmd.RegisterFlagCompletionFunc("api", completion.AllAvailableApis)
	if err != nil {
		log.Fatal("failed to setup CLI %v", err)
	}
}

func setupSharedEntitiesFlags(cmd *cobra.Command, project, outputFolder *string, forceOverwrite *bool, specificEntitiesTypes *[]string, timeFromMinutes *int, timeToMinutes *int, entityPageSize *int) {
	setupSharedFlags(cmd, project, outputFolder, forceOverwrite)
	cmd.Flags().StringSliceVarP(specificEntitiesTypes, "specific-types", "s", make([]string, 0), "List of entity type IDs specifying which entity types to download")
	cmd.Flags().IntVarP(timeFromMinutes, "time-from-minutes", "b", client.DefaultEntityMinutesTimeframeFrom, fmt.Sprintf("How many minutes behind do we want to get entities From, defaults to %d weeks, or %d minutes", client.DefaultEntityWeeksTimeframeFrom, client.DefaultEntityMinutesTimeframeFrom))
	cmd.Flags().IntVarP(timeToMinutes, "time-to-minutes", "t", client.DefaultEntityMinutesTimeframeTo, fmt.Sprintf("How many minutes behind do we want to get entities To, defaults to %d minutes", client.DefaultEntityMinutesTimeframeTo))
	cmd.Flags().IntVarP(entityPageSize, "entity-page-size", "e", client.DefaultPageSizeEntitiesInt, fmt.Sprintf("How many entities per call to download, defaults to %d minutes", client.DefaultPageSizeEntitiesInt))

}
func setupSharedFlags(cmd *cobra.Command, project, outputFolder *string, forceOverwrite *bool) {
	// flags always available
	cmd.Flags().StringVarP(project, "project", "p", "project", "Project to create within the output-folder")
	cmd.Flags().StringVarP(outputFolder, "output-folder", "o", "", "Folder to write downloaded configs to")
	cmd.Flags().BoolVarP(forceOverwrite, "force", "f", false, "Force overwrite any existing manifest.yaml, rather than creating an additional manifest_{timestamp}.yaml. Manifest download: additionally never append source environment name to project folder name")
	err := cmd.MarkFlagDirname("output-folder")
	if err != nil {
		log.Fatal("failed to setup CLI %v", err)
	}
}

// printUploadToSameEnvironmentWarning function may display a warning message on the console,
// notifying the user that downloaded objects cannot be uploaded to the same environment.
// It verifies the version of the tenant and, depending on the result, it may or may not display the warning.
func printUploadToSameEnvironmentWarning(env manifest.EnvironmentDefinition) {
	var serverVersion version.Version
	var err error

	var httpClient *http.Client
	if env.Type == manifest.Classic {
		httpClient = client.NewTokenAuthClient(env.Auth.Token.Value)
	} else {

		credentials := client.OauthCredentials{
			ClientID:     env.Auth.OAuth.ClientID.Value,
			ClientSecret: env.Auth.OAuth.ClientSecret.Value,
			TokenURL:     env.Auth.OAuth.GetTokenEndpointValue(),
		}
		httpClient = client.NewOAuthClient(context.TODO(), credentials)
	}

	serverVersion, err = client.GetDynatraceVersion(httpClient, env.URL.Value)
	if err != nil {
		log.Warn("Unable to determine server version %q: %w", env.URL.Value, err)
	} else if serverVersion.SmallerThan(version.Version{Major: 1, Minor: 262}) {
		logUploadToSameEnvironmentWarning()
	}
}

func logUploadToSameEnvironmentWarning() {
	log.Warn("Uploading Settings 2.0 objects to the same environment is not possible due to your cluster version " +
		"being below 1.262.0, which Monaco does not support for reliably updating downloaded settings without having " +
		"duplicate configurations. Consider upgrading to 1.262+")
}
