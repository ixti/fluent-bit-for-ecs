/*
Copyright © 2025 Alexey Zapparov <alexey@zapparov.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go/aws/arn"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:                   "exec command [args...]",
	Short:                 "Executes a command with ECS task metadata loaded into the environment",
	Args:                  cobra.MinimumNArgs(1),
	DisableFlagsInUseLine: true,
	RunE:                  execCmdRunE,
}

// See: https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task-metadata-endpoint-v4-response.html
type ecsTaskMetadata struct {
	AwsRegion       string
	EcsClusterName  string `json:"Cluster"`     // ECS Cluster Name
	EcsServiceName  string `json:"ServiceName"` // ECS Service Name
	EcsTaskFamily   string `json:"Family"`      // ECS Task Family
	EcsTaskRevision string `json:"Revision"`    // ECS Task Revision
	EcsTaskARN      string `json:"TaskARN"`     // ECS Task ARN
	EcsTaskID       string
}

// Returns the first non-empty string from the provided arguments.
// Returned string is trimmed of leading and trailing whitespace.
func firstNonEmpty(args ...string) string {
	if len(args) == 0 {
		return ""
	}

	for _, arg := range args {
		if arg = strings.TrimSpace(arg); arg != "" {
			return arg
		}
	}
	return ""
}

// Returns true if the string `s` starts with any of the provided prefixes.
// If no prefixes are provided, or none of them is the prefix of the `s`,
// returns false.
func stringStartsWith(s string, prefixes ...string) bool {
	if len(prefixes) == 0 {
		return false
	}

	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func lastArnPart(arn arn.ARN) string {
	parts := strings.Split(arn.Resource, "/")
	return parts[len(parts)-1]
}

func cleanEnviron() []string {
	return slices.DeleteFunc(os.Environ(), func(v string) bool {
		return stringStartsWith(v,
			"AWS_REGION=",
			"ECS_CLUSTER_NAME=",
			"ECS_SERVICE_NAME=",
			"ECS_TASK_FAMILY=",
			"ECS_TASK_REVISION=",
			"ECS_TASK_ARN=",
			"ECS_TASK_ID=",
		)
	})
}

func (m *ecsTaskMetadata) Environ() []string {
	metadataEnviron := []string{
		"AWS_REGION=" + firstNonEmpty(os.Getenv("AWS_REGION"), m.AwsRegion),
		"ECS_CLUSTER_NAME=" + firstNonEmpty(os.Getenv("ECS_CLUSTER_NAME"), m.EcsClusterName),
		"ECS_SERVICE_NAME=" + firstNonEmpty(os.Getenv("ECS_SERVICE_NAME"), m.EcsServiceName),
		"ECS_TASK_FAMILY=" + firstNonEmpty(m.EcsTaskFamily, os.Getenv("ECS_TASK_FAMILY")),
		"ECS_TASK_REVISION=" + firstNonEmpty(m.EcsTaskRevision, os.Getenv("ECS_TASK_REVISION")),
		"ECS_TASK_ARN=" + firstNonEmpty(m.EcsTaskARN, os.Getenv("ECS_TASK_ARN")),
		"ECS_TASK_ID=" + firstNonEmpty(m.EcsTaskID, os.Getenv("ECS_TASK_ID")),
	}

	slog.Debug("Setting environment variables", "metadata", metadataEnviron)

	return append(cleanEnviron(), metadataEnviron...)
}

func getEcsTaskMetadata() (*ecsTaskMetadata, error) {
	metadata := &ecsTaskMetadata{}
	ecsTaskMetadataEndpoint := os.Getenv("ECS_CONTAINER_METADATA_URI_V4")

	if ecsTaskMetadataEndpoint == "" {
		slog.Warn("ECS_CONTAINER_METADATA_URI_V4 environment variable is not set, skipping ECS metadata retrieval")
		return metadata, nil
	}

	req, err := http.NewRequest("GET", ecsTaskMetadataEndpoint+"/task", nil)

	if err != nil {
		return nil, err
	}

	res, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	if err := json.NewDecoder(res.Body).Decode(metadata); err != nil {
		return nil, err
	}

	// Extract Task ID and AWS Region from Task ARN

	taskARN, err := arn.Parse(metadata.EcsTaskARN)

	if err != nil {
		slog.Error("Failed to parse ECS Task ARN", "arn", metadata.EcsTaskARN, "error", err)
	} else {
		metadata.AwsRegion = taskARN.Region
		metadata.EcsTaskID = lastArnPart(taskARN)
	}

	// Per documentation, the Cluster field can be either an ARN or a short name.

	if strings.Contains(metadata.EcsClusterName, "/") {
		clusterARN, err := arn.Parse(metadata.EcsClusterName)

		if err != nil {
			slog.Error("Failed to parse ECS Cluster ARN", "arn", metadata.EcsClusterName, "error", err)
		} else {
			metadata.EcsClusterName = lastArnPart(clusterARN)
		}
	}

	return metadata, nil
}

func execCmdRunE(cmd *cobra.Command, args []string) error {
	argv0, err := exec.LookPath(args[0])

	if err != nil {
		slog.Error("Can't find command", "command", args[0], "error", err)
		return err
	}

	argv := make([]string, 0, len(args))
	argv = append(argv, argv0)
	argv = append(argv, args[1:]...)

	metadata, err := getEcsTaskMetadata()

	if err != nil {
		slog.Error("Can't retrieve ECS task metadata", "error", err)
		return err
	}

	slog.Debug("Executing command", "command", argv)

	if err := unix.Exec(argv0, argv, metadata.Environ()); err != nil {
		slog.Error("Command execution failed", "command", args[0], "error", err)
		return err
	}

	return nil
}

func init() {
	rootCmd.AddCommand(execCmd)

	execCmd.Flags().SetInterspersed(false)
}
