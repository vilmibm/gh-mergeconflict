package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"

	"github.com/cli/safeexec"
)

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
	cmdArgs := []string{
		"api",
		fmt.Sprintf("repos/%s/commits", repo),
		"--paginate",
		"--cache", "24h",
		"--jq", ".[]|.sha",
	}

	sout, _, err := gh(cmdArgs...)
	if err != nil {
		return nil, fmt.Errorf("gh call failed: %w", err)
	}

	split := strings.Split(sout.String(), "\n")

	out := []string{}

	for _, l := range split {
		if l == "" {
			continue
		}
		out = append(out, l)
	}

	return out, nil
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
