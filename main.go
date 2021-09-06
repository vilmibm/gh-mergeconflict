package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/gdamore/tcell/v2"
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

	shas, err := getSHAs(opts.Repository)
	if err != nil {
		return fmt.Errorf("failed to get shas for %s: %w", opts.Repository, err)
	}

	rand.Shuffle(len(issues), func(i, j int) {
		issues[i], issues[j] = issues[j], issues[i]
	})

	style := tcell.StyleDefault

	s, err := tcell.NewScreen()
	if err != nil {
		return err
	}
	if err = s.Init(); err != nil {
		return err
	}
	s.SetStyle(style)

	game := &Game{
		Repo:     opts.Repository,
		debug:    debug,
		Screen:   s,
		Style:    style,
		MaxWidth: 80,
		Logger:   logger,
	}

	err = game.LoadState()
	if err != nil {
		game.Debugf("failed to load state: %s", err)
	}

	// TODO enforce game dimensions, don't bother supporting resizes

	issueSpawners := []*IssueSpawner{}
	y := 2
	x := 0
	for i := 0; i < 10; i++ {
		if i%2 == 0 {
			x = 0
		} else {
			x = game.MaxWidth
		}

		is := NewIssueSpawner(x, y+i, game)

		issueSpawners = append(issueSpawners, is)
		game.AddDrawable(is)
	}

	for ix, issueText := range issues {
		spawnerIx := ix % len(issueSpawners)
		issueSpawners[spawnerIx].AddIssue(issueText)
	}

	cl := NewCommitLauncher(game, shas)
	cl.Transform(37, 13)
	game.AddDrawable(cl)

	cc := NewCommitCounter(35, 14, cl, game)
	game.AddDrawable(cc)

	score := NewScore(38, 18, game)
	game.AddDrawable(score)

	scoreLog := NewScoreLog(15, 15, game)
	game.AddDrawable(scoreLog)

	game.AddDrawable(NewLegend(1, 15, game))

	highScores := NewHighScores(60, 15, game)
	game.AddDrawable(highScores)

	quit := make(chan struct{})
	go func() {
		for {
			ev := s.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:
				switch ev.Rune() {
				case ' ':
					cl.Launch()
				case 'q':
					close(quit)
					return
				}
				switch ev.Key() {
				case tcell.KeyEscape:
					close(quit)
					return
				case tcell.KeyCtrlL:
					s.Sync()
				case tcell.KeyLeft:
					if cl.x > 0 {
						cl.Transform(-1, 0)
					}
				case tcell.KeyRight:
					if cl.w+cl.x < game.MaxWidth {
						cl.Transform(1, 0)

					}
				}
			case *tcell.EventResize:
				s.Sync()
			}
		}
	}()

	// TODO UI
	// - high score listing
	// - "now playing" note
	// TODO high score saving/loading

loop:
	for {
		select {
		case <-quit:
			break loop
		case <-time.After(time.Millisecond * 100):
		}

		s.Clear()
		spawner := issueSpawners[rand.Intn(len(issueSpawners))]
		spawner.Spawn()
		game.Update()
		game.Draw()
		titleStyle := style.Foreground(tcell.ColorBlack).Background(tcell.ColorWhite)
		drawStr(s, 25, 0, titleStyle, "!!! M E R G E  C O N F L I C T !!!")
		s.Show()
	}

	s.Fini()

	// TODO this following code is very bad, abstract to function and clean up
	// TODO GetState helper on Game
	_, ok := game.State.HighScores[opts.Repository]
	if !ok {
		game.State.HighScores[opts.Repository] = []scoreEntry{}
	}

	game.Debugf("%#v\n", game.State.HighScores)

	maxScore := 0
	for _, v := range game.State.HighScores[opts.Repository] {
		if v.Score > maxScore {
			maxScore = v.Score
		}
	}

	game.Debugf("%#v\n", maxScore)
	game.Debugf("%#v\n", score.score)

	if score.score >= maxScore && score.score > 0 {
		answer := false
		// TODO switch to plain survey
		err = survey.AskOne(
			&survey.Confirm{
				Message: "new high score! save it?",
			}, &answer)
		if err == nil && answer {
			answer := ""
			err = survey.AskOne(
				&survey.Input{
					Message: "name",
				}, &answer)
			if err == nil {
				game.Debugf("ABOUT TO SET HIGH SCORE")
				game.Debugf("%#v %s %d", game.State, answer, score.score)
				game.State.HighScores[opts.Repository] = append(game.State.HighScores[opts.Repository], scoreEntry{
					Name:  answer,
					Score: score.score,
				})
				err = game.SaveState()
				game.Debugf("%#v", game.State)
				if err != nil {
					game.Debugf("failed to save state: %s", err)
				}
			}
		}
	}

	return nil
}

func main() {
	rc := rootCmd()

	if err := rc.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
}
