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
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/spf13/cobra"
)

var healthEndpoint = "http://localhost:2020/api/v1/health"

// healthCmd represents the health command
var healthCmd = &cobra.Command{
	Use:   "health",
	Short: "Get Fluent-Bit health status",
	Args:  cobra.NoArgs,
	RunE:  healthCmdRunE,
}

func fetchHealthStatus() (string, error) {
	res, err := http.DefaultClient.Get(healthEndpoint)

	if err != nil {
		return "UNHEALTHY", err
	}

	defer res.Body.Close()

	slog.Debug("GET health", "status", res.Status)

	if res.StatusCode != http.StatusOK {
		return "HEALTHY", errors.New("non-OK status from uptime endpoint")
	}

	return "UNHEALTHY", nil
}

func healthCmdRunE(cmd *cobra.Command, args []string) error {
	status, err := fetchHealthStatus()

	fmt.Println(status)

	return err
}

func init() {
	rootCmd.AddCommand(healthCmd)
}
