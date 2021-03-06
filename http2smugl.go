package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func main() {
	var (
		// root options
		timeout     time.Duration
		connectAddr string
		// request subcommand options
		bodyFile, bodyStr string
		requestMethod     string
		noAutoHeaders     bool
		bodyToSend        []byte
		bodyLines         int
		// detect subcommand options
		silent      bool
		threads     int
		targetsFile string
	)

	requestCmd := &cobra.Command{
		Use:     "request url [header [header...]]",
		Short:   "make one request with custom headers",
		Example: "request https://example.com/ \"transfer-encoding : chunked\"",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if noAutoHeaders && requestMethod != "GET" {
				return fmt.Errorf("cannot combine --method and --no-auto-headers")
			}
			if bodyFile != "" {
				if bodyStr != "" {
					return errors.New("both --body and --body-str specified")
				}
				data, err := ioutil.ReadFile(bodyFile)
				if err != nil {
					return err
				}
				bodyToSend = data
				return nil
			}
			bodyToSend = []byte(unquoteArg(bodyStr))

			target := args[0]
			var headers []Header
			for _, h := range args[1:] {
				parts := strings.SplitN(h, ":", 2)
				if len(parts) != 2 {
					return fmt.Errorf("invalid header: %#v", h)
				}
				if parts[0] == "" && strings.ContainsRune(parts[1], ':') {
					parts = strings.SplitN(parts[1], ":", 2)
					parts[0] = ":" + parts[0]
				}
				headers = append(headers, Header{unquoteArg(parts[0]), unquoteArg(parts[1])})
			}

			doAndPrintRequest(&RequestParams{
				Target:        target,
				Method:        unquoteArg(requestMethod),
				ConnectAddr:   connectAddr,
				Headers:       headers,
				NoAutoHeaders: noAutoHeaders,
				Body:          bodyToSend,
				Timeout:       timeout,
			}, bodyLines)
			return nil
		},
	}

	detectCmd := &cobra.Command{
		Use:   "detect [flags] [url [url...]]",
		Short: "detect if an url is vulnerable",
		RunE: func(cmd *cobra.Command, args []string) error {
			targets := args
			if targetsFile != "" {
				data, err := ioutil.ReadFile(targetsFile)
				if err != nil {
					return err
				}
				for _, line := range strings.Split(string(data), "\n") {
					line = strings.TrimSpace(line)
					if line != "" {
						targets = append(targets, line)
					}
				}
			}
			return detectMultipleTargets(targets, connectAddr, threads, timeout, !silent)
		},
	}

	var rootCmd = &cobra.Command{}

	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 10*time.Second, "timeout to all requests")
	rootCmd.PersistentFlags().StringVar(&connectAddr, "connect-to", "", "override target ip")
	requestCmd.Flags().StringVar(&requestMethod, "method", "GET", "request method")
	requestCmd.Flags().StringVar(&bodyStr, "body-str", "", "send this string to body (escape seqs like \\r \\n are supported)")
	requestCmd.Flags().StringVar(&bodyFile, "body-file", "", "read request body from this file")
	requestCmd.Flags().BoolVar(&noAutoHeaders, "no-auto-headers", false, "don't send pseudo-headers automatically")
	requestCmd.Flags().IntVar(&bodyLines, "body-lines", 10, "how many body lines to print (-1 means no limit)")

	detectCmd.Flags().BoolVar(&silent, "silent", false, "be more silent")
	detectCmd.Flags().IntVar(&threads, "threads", 100, "number of threads")
	detectCmd.Flags().StringVar(&targetsFile, "targets", "", "read targets list from this file")

	rootCmd.AddCommand(requestCmd, detectCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func unquoteArg(s string) string {
	if decoded, err := strconv.Unquote(`"` + s + `"`); err == nil {
		return decoded
	}
	return s
}

func doAndPrintRequest(params *RequestParams, bodyLines int) {
	headers, body, err := DoRequest(params)
	if err != nil {
		fmt.Printf("Error is %v\n", err)
	}
	for _, h := range headers {
		fmt.Printf("%s: %s\n", h.Name, h.Value)
	}
	fmt.Println()
	lines := bytes.Split(body, []byte{'\n'})
	for i, l := range lines {
		if bodyLines < 0 || i < bodyLines {
			fmt.Println(string(l))
		} else {
			break
		}
	}
}
