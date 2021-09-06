package main

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

type Issue struct {
	GameObject
	dir Direction
}

func NewIssue(x, y int, dir Direction, text string, game *Game) *Issue {
	// I disabled the background because i didn't like how spaces left behind had gray behind them while spaces between issues were black. mainly i just want it to be consistent.
	style := game.Style.Foreground(tcell.ColorWhite) //.Background(tcell.ColorBlack)
	return &Issue{
		dir: dir,
		GameObject: GameObject{
			x:             x,
			y:             y,
			w:             len(text),
			h:             1,
			Sprite:        text,
			Game:          game,
			StyleOverride: &style,
		},
	}
}

func (i *Issue) Update() {
	i.Transform(int(i.dir), 0)
	if i.dir > 0 && i.x > 5+i.Game.MaxWidth {
		// hoping this is enough for GC to claim
		i.Game.Destroy(i)
	}

	if i.dir < 0 && i.x < -5-len(i.Sprite) {
		i.Game.Destroy(i)
	}
}

func (i *Issue) LetterAt(x int) rune {
	return rune(i.Sprite[x])
}

func (i *Issue) DestroyLetterAt(x int) {
	newSprite := ""
	for ix := 0; ix < i.w; ix++ {
		if ix == x {
			newSprite += " "
		} else {
			newSprite += string(i.Sprite[ix])
		}
	}
	i.Sprite = newSprite
}

// would be nice to just call "spawn" at random intervals but have the spawner lock itself if it's already got something still going
// how should it track if it's active?
type IssueSpawner struct {
	GameObject
	issues    []string
	countdown int
}

func NewIssueSpawner(x, y int, game *Game) *IssueSpawner {
	return &IssueSpawner{
		GameObject: GameObject{
			x:    x,
			y:    y,
			Game: game,
		},
	}
}

func (is *IssueSpawner) Update() {
	if is.countdown > 0 {
		is.countdown--
	}
}

func (is *IssueSpawner) Spawn() {
	if is.countdown > 0 || len(is.issues) == 0 {
		// TODO eventually, notice if all spawners are empty and trigger win condition
		//is.Game.Debugf("%s is dry", is)
		return
	}

	issueText := is.issues[0]
	is.issues = is.issues[1:]

	is.countdown = len(issueText) + 3 // add arbitrary cool off

	// is.x is either 0 or maxwidth
	x := is.x
	var dir Direction
	dir = -1
	if is.x == 0 {
		x = 0 - len(issueText) + 1
		dir = 1
	}

	is.Game.AddDrawable(NewIssue(x, is.y, dir, issueText, is.Game))
}

func (is *IssueSpawner) AddIssue(issue string) {
	is.issues = append(is.issues, issue)
}

type Burst struct {
	GameObject
	life int
}

func NewBurst(x, y int, g *Game) *Burst {
	style := g.Style.Foreground(tcell.ColorYellow)
	return &Burst{
		life: 3,
		GameObject: GameObject{
			x:             x - 1,
			y:             y - 1,
			w:             3,
			h:             3,
			Game:          g,
			StyleOverride: &style,
			Sprite: `\ /

/ \`,
		},
	}
}

func NewBigBurst(x, y int, g *Game) *Burst {
	style := g.Style.Foreground(tcell.ColorPink)
	return &Burst{
		life: 3,
		GameObject: GameObject{
			x:             x - 6,
			y:             y - 3,
			w:             13,
			h:             6,
			Game:          g,
			StyleOverride: &style,
			Sprite: `*     *
    \   /
*)___\ /___(*
     / \
    /   \
   *     *`,
		},
	}
}

func (b *Burst) Update() {
	if b.life == 0 {
		b.Game.Destroy(b)
		return
	}
	b.life--
}

type CommitLauncher struct {
	GameObject
	cooldown     int // prevents double shooting which make bullets collide
	Shas         []string
	rainbowIndex int
}

type CommitShot struct {
	GameObject
	life int
	sha  string
}

func (cs *CommitShot) Update() {
	if cs.life == 0 {
		cs.Game.Destroy(cs)
	}
	cs.life--
}

func (cs *CommitShot) LetterAt(y int) rune {
	return rune(cs.sha[y])
}

func NewCommitShot(g *Game, x, y int, sha string) *CommitShot {
	sprite := ""
	for i := len(sha) - 1; i >= 0; i-- {
		sprite += string(sha[i]) + "\n"
	}
	return &CommitShot{
		life: 3,
		sha:  sha,
		GameObject: GameObject{
			Sprite: sprite,
			x:      x,
			y:      y,
			w:      1,
			h:      len(sha),
			Game:   g,
		},
	}
}

func (cl *CommitLauncher) Update() {
	if cl.cooldown > 0 {
		cl.cooldown--
	}
}

func (cl *CommitLauncher) ColorForShot(sha string) tcell.Style {
	style := cl.Game.Style
	switch cl.rainbowIndex {
	case 0:
		style = style.Foreground(tcell.ColorRed)
	case 1:
		style = style.Foreground(tcell.ColorOrange)
	case 2:
		style = style.Foreground(tcell.ColorYellow)
	case 3:
		style = style.Foreground(tcell.ColorGreen)
	case 4:
		style = style.Foreground(tcell.ColorBlue)
	case 5:
		style = style.Foreground(tcell.ColorIndigo)
	case 6:
		style = style.Foreground(tcell.ColorPurple)
	}
	cl.rainbowIndex++
	cl.rainbowIndex %= 7
	return style
}

func (cl *CommitLauncher) Launch() {
	if cl.cooldown > 0 {
		return
	}
	cl.cooldown = 4

	if len(cl.Shas) == 1 {
		// TODO need to signal game over
		return
	}
	sha := cl.Shas[0]
	cl.Shas = cl.Shas[1:]
	shotX := cl.x + 3
	shotY := cl.y - len(sha)
	shot := NewCommitShot(cl.Game, shotX, shotY, sha)
	style := cl.ColorForShot(sha)
	shot.StyleOverride = &style
	// TODO add ToRay to CommitShot
	ray := &Ray{}
	for i := 0; i < len(sha); i++ {
		ray.AddPoint(shotX, shotY+i)
	}
	cl.Game.DetectHits(ray, shot)
	cl.Game.AddDrawable(shot)
}

func NewCommitLauncher(g *Game, shas []string) *CommitLauncher {
	style := g.Style.Foreground(tcell.ColorPurple)
	return &CommitLauncher{
		Shas: shas,
		GameObject: GameObject{
			Sprite:        "-=$^$=-",
			w:             7,
			h:             1,
			Game:          g,
			StyleOverride: &style,
		},
	}
}

type CommitCounter struct {
	cl *CommitLauncher
	GameObject
}

func NewCommitCounter(x, y int, cl *CommitLauncher, game *Game) *CommitCounter {
	style := game.Style.Background(tcell.ColorCornflowerBlue)
	return &CommitCounter{
		cl: cl,
		GameObject: GameObject{
			x:             x,
			y:             y,
			h:             1,
			Sprite:        fmt.Sprintf("%d commits remain", len(cl.Shas)),
			Game:          game,
			StyleOverride: &style,
		},
	}
}

func (cc *CommitCounter) Update() {
	sprite := fmt.Sprintf("%d commits remain", len(cc.cl.Shas))
	cc.Sprite = sprite
	cc.w = len(sprite)
}

type Score struct {
	GameObject
	score int
}

func (s *Score) Add(i int) {
	s.score += i
}

func NewScore(x, y int, game *Game) *Score {
	style := game.Style.Foreground(tcell.ColorGold)
	text := fmt.Sprintf("SCORE: %d", 0)
	return &Score{
		GameObject: GameObject{
			x:             x,
			y:             y,
			w:             len(text),
			h:             1,
			Game:          game,
			Sprite:        text,
			StyleOverride: &style,
		},
	}
}

func (s *Score) Update() {
	text := fmt.Sprintf("SCORE: %d", s.score)
	s.Sprite = text
	s.w = len(text)
}

func NewLegend(x, y int, game *Game) *GameObject {
	return &GameObject{
		x:    x,
		y:    y,
		Game: game,
		Sprite: `move:  ← →
space: fire
q:     quit`,
	}
}

func NewHighScores(x, y int, g *Game) *GameObject {
	sprite := "~* high scores *~"
	highScores, ok := g.State.HighScores[g.Repo]
	if ok {
		for x := len(highScores) - 1; x >= 0; x-- {
			sprite += fmt.Sprintf("\n%s %d", highScores[x].Name, highScores[x].Score)
		}
	}
	return &GameObject{
		x:      x,
		y:      y,
		Game:   g,
		Sprite: sprite,
	}
}

type ScoreLog struct {
	GameObject
	log []string
}

func NewScoreLog(x, y int, game *Game) *ScoreLog {
	style := game.Style.Foreground(tcell.ColorBlack).Background(tcell.ColorWhite)
	return &ScoreLog{
		GameObject: GameObject{
			x:             x,
			y:             y,
			Game:          game,
			StyleOverride: &style,
		},
	}
}

func (sl *ScoreLog) Update() {
	sl.Sprite = strings.Join(sl.log, "\n")
	sl.h = len(sl.log)
	sl.w = 15
}

func (sl *ScoreLog) Log(value int, get bool) {
	msg := fmt.Sprintf("%d points!", value)
	if get {
		msg = fmt.Sprintf("%d points BONUS GET!", value)
	}
	sl.log = append(sl.log, msg)
	if len(sl.log) > 5 {
		sl.log = sl.log[1:]
	}
}
