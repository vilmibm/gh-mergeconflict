package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/cli/safeexec"
	"github.com/spf13/cobra"
)

type mcOpts struct {
	Repository string
	Debug      bool
}

func rootCmd() *cobra.Command {
	opts := mcOpts{}
	cmd := &cobra.Command{
		Use:           "mergeconflict",
		Short:         "play a game about open source triage in your terminal",
		Args:          cobra.ExactArgs(0),
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Repository == "" {
				repo, err := resolveRepository()
				if err != nil {
					return err
				}
				opts.Repository = repo
			}
			return runMC(opts)
		},
	}

	cmd.Flags().StringVarP(&opts.Repository, "repo", "R", "", "Repository to play in")
	cmd.Flags().BoolVarP(&opts.Debug, "debug", "d", false, "enable logging")

	return cmd
}

func runMC(opts mcOpts) error {
	debug := opts.Debug

	var logger *log.Logger
	if debug {
		f, _ := os.Create("mclog.txt")
		logger = log.New(f, "", log.Lshortfile)
		logger.Println("mc logging")
	}

	rand.Seed(time.Now().UTC().UnixNano())

	issues, err := getIssues(opts.Repository)
	if err != nil {
		return fmt.Errorf("failed to get issues for %s: %w", opts.Repository, err)
	}

	fmt.Printf("DBG %#v\n", issues)

	shas, err := getSHAs(opts.Repository)
	if err != nil {
		return fmt.Errorf("failed to get shas for %s: %w", opts.Repository, err)
	}

	fmt.Printf("DBG %#v\n", shas)

	rand.Shuffle(len(issues), func(i, j int) {
		issues[i], issues[j] = issues[j], issues[i]
	})

	return nil
}

func resolveRepository() (string, error) {
	sout, eout, err := gh("repo", "view")
	if err != nil {
		if strings.Contains(eout.String(), "not a git repository") {
			return "", errors.New("Try running this command from inside a git repository or with the -R flag")
		}
		return "", err
	}
	viewOut := strings.Split(sout.String(), "\n")[0]
	repo := strings.TrimSpace(strings.Split(viewOut, ":")[1])

	return repo, nil
}
func main() {
	rc := rootCmd()

	if err := rc.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}

// gh shells out to gh, returning STDOUT/STDERR and any error
func gh(args ...string) (sout, eout bytes.Buffer, err error) {
	ghBin, err := safeexec.LookPath("gh")
	if err != nil {
		err = fmt.Errorf("could not find gh. Is it installed? error: %w", err)
		return
	}

	cmd := exec.Command(ghBin, args...)
	cmd.Stderr = &eout
	cmd.Stdout = &sout

	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("failed to run gh. error: %w, stderr: %s", err, eout.String())
		return
	}

	return
}

func getSHAs(repo string) ([]string, error) {
	// TODO
	return []string{}, nil
}

func getIssues(repo string) ([]string, error) {
	query := `
		query GetIssuesForMC($owner: String!, $repo: String!, $endCursor: String) {
			repository(owner: $owner, name: $repo) {
				hasIssuesEnabled
				issues(first: 100, after: $endCursor, states: [OPEN]) {
					nodes {
						number
						title
					}
					pageInfo {
						hasNextPage
						endCursor
					}
				}
			}
		}`
	parts := strings.Split(repo, "/")
	owner := parts[0]
	name := parts[1]

	cmdArgs := []string{
		"api", "graphql",
		"--paginate",
		"--cache", "24h",
		"-f", fmt.Sprintf("query=%s", query),
		"-f", fmt.Sprintf("owner=%s", owner),
		"-f", fmt.Sprintf("repo=%s", name),
		"--jq", ".[]",
	}

	sout, _, err := gh(cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("gh call failed: %w", err)
	}

	type Doc struct {
		Repository struct {
			HasIssuesEnabled bool
			Issues           struct {
				Nodes []struct {
					Number int
					Title  string
				}
				PageInfo struct {
					HasNextPage bool
					EndCursor   string
				}
			}
		}
	}

	out := []string{}

	dec := json.NewDecoder(strings.NewReader(sout.String()))
	for {
		var doc Doc

		err := dec.Decode(&doc)
		if err == io.EOF {
			// all done
			break
		}
		if err != nil {
			log.Fatal(err)
		}

		if !doc.Repository.HasIssuesEnabled {
			return nil, errors.New("can only play in repositories with issues enabled")
		}

		for _, issue := range doc.Repository.Issues.Nodes {
			out = append(out, fmt.Sprintf("#%d %s", issue.Number, issue.Title))
		}

	}
	return out, nil
}
