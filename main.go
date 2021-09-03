package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cli/safeexec"
	"github.com/spf13/cobra"
)

type mcOpts struct {
	Repository string
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

	return cmd
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

func runMC(opts contributeOpts) error {
	return nil
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
