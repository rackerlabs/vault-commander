package ui

import (
	"fmt"
	"log"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/rackerlabs/vault-commander/api"
)

func init() {
	le = lineEditor{gocui.DefaultEditor}
}

func MainScreen(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	if v, err := g.SetView("side", 1, 1, 30, maxY-10); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.Frame = true
		v.Title = "Mounts"
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		for _, mount := range api.ListMounts() {
			fmt.Fprintln(v, mount)
		}
	}
	if v, err := g.SetView("main", 30, 1, maxX-1, maxY-10); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = false
		v.Frame = true
		v.Title = "Keys"
		v.Wrap = true
	}
	if v, err := g.SetView("legend", maxX-24, maxY-9, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = false
		v.Frame = true
		v.Title = "Legend"
	}
	if v, err := g.SetView("log", 1, maxY-9, maxX-25, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = false
		v.Frame = true
		v.Autoscroll = true
		v.Title = "Log"
	}
	return nil
}

func InitScreen(g *gocui.Gui, mp string) error {
	MainScreen(g)
	if mp != "" {
		initMount(g, mp)
	} else {
		g.SetCurrentView("side")
		UpdateLegend(g, sidelegend)
	}

	return nil
}

func initMount(g *gocui.Gui, mp string) error {
	v, _ := g.View("side")

	cx, cy := v.Cursor()
	for i := 0; ; i++ {
		line, _ := v.Line(cy + i)
		if line == "" {
			log.Panicln("Unable to find mount point")
		}
		if line == fmt.Sprintf("%s/", mp) {
			v.SetCursor(cx, cy+i)
			break
		}
	}

	v, _ = g.View("main")
	v.Highlight = true
	v.SelBgColor = gocui.ColorGreen
	v.SelFgColor = gocui.ColorBlack
	v.Clear()
	var keys []string
	api.ListAllKeys(keys, fmt.Sprintf("%s/", mp), "", v)
	g.SetCurrentView("main")
	UpdateLegend(g, mainlegend)
	UpdateLog(g, fmt.Sprintf("Viewing secrets on %s mount", mp))
	return nil
}

func UpdateLegend(g *gocui.Gui, legend string) {
	v, _ := g.View("legend")
	v.Clear()
	fmt.Fprintln(v, legend)
}

func UpdateLog(g *gocui.Gui, log string) {
	v, _ := g.View("log")
	t := time.Now()
	longForm := "2006-01-02 3:04:05pm (MST)"
	fmt.Fprintln(v, t.Format(longForm), log)
}

func CreateView(g *gocui.Gui, viewName string, x1 int, y1 int, x2 int, y2 int) *gocui.View {
	var v *gocui.View
	var err error

	if v, err = g.SetView(viewName, x1, y1, x2, y2); err != nil {
		if err != gocui.ErrUnknownView {
			log.Panicln(err)
		}

		p := vp[viewName]
		v.Autoscroll = p.autoscroll
		v.Editable = p.editable
		v.Editor = p.editor
		v.Frame = p.frame
		v.Title = p.title
		v.Wrap = p.wrap

		if _, err := g.SetCurrentView(viewName); err != nil {
			log.Panicln(err)
		}
	}
	return v
}

type viewProperties struct {
	autoscroll bool
	editable   bool
	editor     gocui.Editor
	frame      bool
	title      string
	wrap       bool
}

var vp = map[string]viewProperties{
	"secret": {
		autoscroll: false,
		editable:   false,
		editor:     gocui.DefaultEditor,
		frame:      false,
		title:      "",
		wrap:       true,
	},
	"editsecret": {
		autoscroll: false,
		editable:   true,
		editor:     gocui.DefaultEditor,
		frame:      false,
		title:      "",
		wrap:       true,
	},
	"deletekeyprompt": {
		autoscroll: false,
		editable:   false,
		editor:     gocui.DefaultEditor,
		frame:      true,
		title:      "WARNING",
		wrap:       false,
	},
	"addkeyprompt": {
		autoscroll: false,
		editable:   true,
		editor:     &le,
		frame:      true,
		title:      "Insert Key Name",
		wrap:       false,
	},
	"saveprompt": {
		autoscroll: false,
		editable:   false,
		editor:     gocui.DefaultEditor,
		frame:      true,
		title:      "Save Changes",
		wrap:       false,
	},
}

func Keybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("side", gocui.KeyTab, gocui.ModNone, NextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyEnter, gocui.ModNone, GetLine); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyEnter, gocui.ModNone, ViewSecret); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyTab, gocui.ModNone, NextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("deletekeyprompt", 'y', gocui.ModNone, DeleteKey); err != nil {
		return err
	}
	if err := g.SetKeybinding("deletekeyprompt", 'n', gocui.ModNone, HomeView); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", 'd', gocui.ModNone, DeleteKeyPrompt); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", 'a', gocui.ModNone, AddKeyPrompt); err != nil {
		return err
	}
	if err := g.SetKeybinding("addkeyprompt", gocui.KeyEnter, gocui.ModNone, EditSecret); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyArrowDown, gocui.ModNone, CursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyArrowUp, gocui.ModNone, CursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyArrowDown, gocui.ModNone, CursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeySpace, gocui.ModNone, PageDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyArrowUp, gocui.ModNone, CursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, Quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("secret", 'q', gocui.ModNone, MainView); err != nil {
		return err
	}
	if err := g.SetKeybinding("editsecret", gocui.KeyCtrlX, gocui.ModNone, CancelEdit); err != nil {
		return err
	}
	if err := g.SetKeybinding("editsecret", gocui.KeyCtrlL, gocui.ModNone, OpenEditor); err != nil {
		return err
	}
	if err := g.SetKeybinding("saveprompt", 'y', gocui.ModNone, SaveSecret); err != nil {
		return err
	}
	if err := g.SetKeybinding("saveprompt", 'n', gocui.ModNone, DeletePrompt); err != nil {
		return err
	}
	if err := g.SetKeybinding("editsecret", gocui.KeyCtrlS, gocui.ModNone, SavePrompt); err != nil {
		return err
	}
	if err := g.SetKeybinding("secret", 'e', gocui.ModNone, EditSecret); err != nil {
		return err
	}
	return nil
}
