package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"github.com/controlled-opposition/billboard/internal/output"
)

func newAPICmd(e *env) *cobra.Command {
	var expr string
	var q *output.JQ
	cmd := &cobra.Command{
		Use:   "api <path>",
		Short: "GET a Blackboard REST path with the stored session",
		Long: `GET a path on blackboard.wm.edu with the stored session and write the raw
response body to stdout. Renewal, pacing, the request cap, and exit codes
work as in every other command. Paths outside the Blackboard host are
refused, so the session cookie never leaves it.

Examples:
  bb api /learn/api/public/v1/users/me
  bb api /learn/api/public/v1/terms --jq '.results[].name'`,
		Args: positional(cobra.ExactArgs(1)),
		PreRunE: func(*cobra.Command, []string) error {
			if expr == "" {
				return nil
			}
			var err error
			if q, err = output.ParseJQ(expr); err != nil {
				return usageError{err.Error()}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := apiPath(args[0])
			if err != nil {
				return err
			}
			body, err := e.client().Raw(cmd.Context(), http.MethodGet, path)
			if err != nil {
				return err
			}
			defer body.Close()
			if q != nil {
				return q.Run(body, e.streams.Out)
			}
			_, err = io.Copy(e.streams.Out, body)
			return err
		},
	}
	cmd.Flags().StringVarP(&expr, "jq", "q", "", "filter the response with a jq `expression`")
	return cmd
}

func apiPath(arg string) (string, error) {
	u, err := url.Parse(arg)
	if err != nil || u.Scheme != "" || u.Host != "" || strings.HasPrefix(arg, "//") {
		return "", usageError{fmt.Sprintf("api takes a path on the Blackboard host, not a URL: %q", arg)}
	}
	if !strings.HasPrefix(arg, "/") {
		arg = "/" + arg
	}
	return arg, nil
}
