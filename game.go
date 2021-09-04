package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/mattn/go-runewidth"
	"gopkg.in/yaml.v3"
)

const (
	XDG_STATE_HOME = "XDG_STATE_HOME"
	LOCAL_APP_DATA = "LocalAppData"
)

type Drawable interface {
	Draw()
	Update()
}

type GameObject struct {
	x             int
	y             int
	w             int
	h             int
	Sprite        string
	Game          *Game
	StyleOverride *tcell.Style
}

func (g *GameObject) Update() {}

func (g *GameObject) Transform(x, y int) {
	g.x += x
	g.y += y
}

func (g *GameObject) Draw() {
	screen := g.Game.Screen
	style := g.Game.Style
	if g.StyleOverride != nil {
		style = *g.StyleOverride
	}
	lines := strings.Split(g.Sprite, "\n")
	for i, l := range lines {
		drawStr(screen, g.x, g.y+i, style, l)
	}
}

type Direction int // either -1 or 1

type Game struct {
	debug     bool
	drawables []Drawable
	Screen    tcell.Screen
	Style     tcell.Style
	MaxWidth  int
	Logger    *log.Logger
	State     map[string]interface{}
}

func (g *Game) Debugf(format string, v ...interface{}) {
	if g.debug == false {
		return
	}
	g.Logger.Printf(format, v...)
}

var stateFilename string = "mc.yml"

func dirExists(path string) bool {
	f, err := os.Stat(path)
	return err == nil && f.IsDir()
}

// State path precedence
// 1. XDG_CONFIG_HOME
// 2. LocalAppData (windows only)
// 3. HOME
func stateDir() string {
	var path string
	if a := os.Getenv(XDG_STATE_HOME); a != "" {
		path = filepath.Join(a, "gh")
	} else if b := os.Getenv(LOCAL_APP_DATA); runtime.GOOS == "windows" && b != "" {
		path = filepath.Join(b, "GitHub CLI")
	} else {
		c, _ := os.UserHomeDir()
		path = filepath.Join(c, ".local", "state", "gh")
	}

	// If the path does not exist try migrating state from default paths
	if !dirExists(path) {
		_ = os.MkdirAll(path, 0755)
	}

	return path
}

func (g *Game) LoadState() error {
	stateFilePath := filepath.Join(stateDir(), stateFilename)
	g.State = map[string]interface{}{}
	g.State["HighScores"] = map[string]int{}

	content, err := ioutil.ReadFile(stateFilePath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(content, &g.State)
	if err != nil {
		return err
	}

	return nil
}

func (g *Game) SaveState() error {
	// TODO

	return nil
}

func (g *Game) AddDrawable(d Drawable) {
	g.drawables = append(g.drawables, d)
}

func (g *Game) Destroy(d Drawable) {
	newDrawables := []Drawable{}
	for _, dd := range g.drawables {
		if dd == d {
			continue
		}
		newDrawables = append(newDrawables, dd)
	}
	g.drawables = newDrawables
}

func (g *Game) Update() {
	for _, gobj := range g.drawables {
		gobj.Update()
	}
}

func (g *Game) Draw() {
	for _, gobj := range g.drawables {
		gobj.Draw()
	}
}

func (g *Game) FindGameObject(fn func(Drawable) bool) Drawable {
	for _, gobj := range g.drawables {
		if fn(gobj) {
			return gobj
		}
	}
	return nil
}

func (g *Game) FilterGameObjects(fn func(Drawable) bool) []Drawable {
	out := []Drawable{}
	for _, gobj := range g.drawables {
		if fn(gobj) {
			out = append(out, gobj)
		}
	}
	return out
}

func (g *Game) DetectHits(r *Ray, shot *CommitShot) {
	score := g.FindGameObject(func(gobj Drawable) bool {
		_, ok := gobj.(*Score)
		return ok
	})
	scoreLog := g.FindGameObject(func(gobj Drawable) bool {
		_, ok := gobj.(*ScoreLog)
		return ok
	})
	if score == nil {
		panic("could not find score game object")
	}
	if scoreLog == nil {
		panic("could not find score log game object")
	}
	thisShot := 0
	matchesMultiplier := 1

	// TODO dirty to do side effects in a filter, consider renaming/tweaking
	_ = g.FilterGameObjects(func(gobj Drawable) bool {
		issue, ok := gobj.(*Issue)
		if !ok {
			return false
		}
		shotX := r.Points[0].X
		shotY := r.Points[len(r.Points)-1].Y
		if shotX < issue.x || shotX >= issue.x+issue.w {
			return false
		}

		r := issue.LetterAt(shotX - issue.x)
		if r == ' ' {
			return false
		}

		thisShot++

		issue.DestroyLetterAt(shotX - issue.x)

		var burst *Burst

		if r == shot.LetterAt(shotY-issue.y) {
			g.Debugf("OMG CHARACTER HIT %s\n", string(r))
			matchesMultiplier *= 2
			burst = NewBigBurst(shotX, issue.y, g)
		} else {
			burst = NewBurst(shotX, issue.y, g)
		}
		g.AddDrawable(burst)

		return true
	})

	bonus := false
	if thisShot == 10 {
		matchesMultiplier *= 2
	}
	if matchesMultiplier > 1 {
		bonus = true
		thisShot *= matchesMultiplier
	}

	if thisShot > 0 {
		scoreLog.(*ScoreLog).Log(thisShot, bonus)
		score.(*Score).Add(thisShot)
	}
}

type Point struct {
	X int
	Y int
}

func (p Point) String() string {
	return fmt.Sprintf("<%d, %d>", p.X, p.Y)
}

type Ray struct {
	Points []Point
}

func (r *Ray) AddPoint(x, y int) {
	r.Points = append(r.Points, Point{X: x, Y: y})
}

func drawStr(s tcell.Screen, x, y int, style tcell.Style, str string) {
	// TODO put this into Game
	for _, c := range str {
		var comb []rune
		w := runewidth.RuneWidth(c)
		if w == 0 {
			comb = []rune{c}
			c = ' '
			w = 1
		}
		s.SetContent(x, y, c, comb, style)
		x += w
	}
}
