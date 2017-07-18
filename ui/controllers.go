package ui

import (
	"encoding/json"
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/jroimartin/gocui"
	"github.com/rackerlabs/vault-commander/api"
)

var sidelegend = "↑ - cursor up\n↓ - cursor down\nTab - switch windows\nRet - select mount"
var mainlegend = "Tab - switch windows\nRet - view secret\na - add secret\nd - delete secret\nSpace - page down"
var secretlegend = "e - edit secret\nq - quit view"
var editlegend = "C-l - Open in $EDITOR\nC-x - quit don't save\nC-s - save"
var editmode string

func NextView(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() == "side" {
		_, err := g.SetCurrentView("main")
		UpdateLegend(g, mainlegend)
		return err
	}
	_, err := g.SetCurrentView("side")
	UpdateLegend(g, sidelegend)
	return err
}

func Quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func GetLine(g *gocui.Gui, v *gocui.View) error {
	var l string
	var err error

	_, cy := v.Cursor()
	if l, err = v.Line(cy); err != nil {
		l = ""
	}

	v, _ = g.View("main")
	v.Highlight = true
	v.SelBgColor = gocui.ColorGreen
	v.SelFgColor = gocui.ColorBlack
	v.Clear()
	var keys []string
	api.ListAllKeys(keys, l, "", v)
	if err := v.SetCursor(0, 0); err != nil {
		return err
	}
	if err := v.SetOrigin(0, 0); err != nil {
		return err
	}
	//cx, cy := v.Cursor()

	g.SetCurrentView("main")
	UpdateLegend(g, mainlegend)
	UpdateLog(g, fmt.Sprintf("Viewing secrets on %s mount", l))
	return nil
}

func ViewSecret(g *gocui.Gui, v *gocui.View) error {
	var l string
	var err error

	_, cy := v.Cursor()
	if l, err = v.Line(cy); err != nil {
		l = ""
	}

	if l == "" {
		return nil
	}

	maxX, maxY := g.Size()
	v = CreateView(g, "secret", -1, -1, maxX, maxY-9)
	fmt.Fprintln(v, api.ReadValue(l, v))
	UpdateLegend(g, secretlegend)
	UpdateLog(g, fmt.Sprintf("Viewing secret contents of %s", l))
	return nil
}

func DeleteKey(g *gocui.Gui, v *gocui.View) error {
	var secretpath string
	var err error
	x, _ := g.View("main")
	_, cy := x.Cursor()

	if secretpath, err = x.Line(cy); err != nil {
		secretpath = ""
	}

	err = api.Delete(secretpath)

	if err != nil {
		log.Panicln(err)
	}
	UpdateLog(g, fmt.Sprintf("Deleted secret %s", secretpath))
	HomeView(g, v)
	return nil
}

func HomeView(g *gocui.Gui, v *gocui.View) error {
	views := g.Views()
	for i := 0; i < len(views); i++ {
		if views[i].Name() == "side" || views[i].Name() == "main" || views[i].Name() == "log" || views[i].Name() == "legend" {
			continue
		} else {
			if err := g.DeleteView(views[i].Name()); err != nil {
				return err
			}
		}
	}
	if _, err := g.SetCurrentView("main"); err != nil {
		return err
	}
	UpdateLegend(g, mainlegend)
	v, _ = g.View("side")
	GetLine(g, v)
	return nil
}

func CancelEdit(g *gocui.Gui, v *gocui.View) error {
	var secretpath string
	var err error

	if editmode == "Editing" {
		x, _ := g.View("main")
		_, cy := x.Cursor()
		if secretpath, err = x.Line(cy); err != nil {
			secretpath = ""
		}
	} else if editmode == "Writing" {
		x, _ := g.View("addkeyprompt")
		secretpath = x.Buffer()
		secretpath = strings.TrimSpace(secretpath)
	}

	UpdateLog(g, fmt.Sprintf("Canceled edit of %s.", secretpath))
	HomeView(g, v)
	return nil
}

func DeleteKeyPrompt(g *gocui.Gui, v *gocui.View) error {
	x, _ := g.View("main")
	var secretpath string
	var err error

	_, cy := x.Cursor()
	if secretpath, err = x.Line(cy); err != nil {
		secretpath = ""
	}

	if secretpath == "" {
		return nil
	}

	secretlength := len(secretpath) + 19

	maxX, maxY := g.Size()
	v = CreateView(g, "deletekeyprompt", maxX/2-secretlength/2, maxY/2, maxX/2+secretlength/2, maxY/2+2)
	fmt.Fprintln(v, "Delete "+secretpath+"? (y/n)")

	return nil
}

func AddKeyPrompt(g *gocui.Gui, v *gocui.View) error {
	x, _ := g.View("main")
	var secretpath string
	var err error

	_, cy := x.Cursor()
	if secretpath, err = x.Line(cy); err != nil {
		secretpath = ""
	}

	if secretpath == "" {
		v, _ = g.View("side")
		_, cy := x.Cursor()
		secretpath, _ = v.Line(cy)
	}

	secretpath = regexp.MustCompile("/\\w*$").Split(secretpath, 2)[0]
	secretlength := len(secretpath) + 19

	maxX, maxY := g.Size()
	v = CreateView(g, "addkeyprompt", maxX/2-(secretlength+5)/2, maxY/2, maxX/2+(secretlength+5)/2, maxY/2+2)

	fmt.Fprintln(v, secretpath+"/")

	if err := v.SetCursor(len(secretpath)+1, 0); err != nil {
		return err
	}

	return nil
}

func EditSecret(g *gocui.Gui, v *gocui.View) error {
	var secretpath, secret string
	var err error

	if v.Name() == "secret" {
		editmode = "Editing"
		secret = v.Buffer()
		x, _ := g.View("main")
		_, cy := x.Cursor()
		if secretpath, err = x.Line(cy); err != nil {
			secretpath = ""
		}
	} else if v.Name() == "addkeyprompt" {
		editmode = "Writing"
		secret = ""
		x, _ := g.View("addkeyprompt")
		_, cy := x.Cursor()
		if secretpath, err = x.Line(cy); err != nil {
			secretpath = ""
		}
		secretpath = strings.TrimSpace(secretpath)
	}

	maxX, maxY := g.Size()
	v = CreateView(g, "editsecret", -1, -1, maxX, maxY-9)
	fmt.Fprintln(v, secret)
	UpdateLegend(g, editlegend)
	UpdateLog(g, fmt.Sprintf("%s secret contents of %s", editmode, secretpath))
	return nil
}

func CursorDown(g *gocui.Gui, v *gocui.View) error {
	_, cy := v.Cursor()
	line, _ := v.Line(cy + 1)
	if line == "" {
		return nil
	}

	if v != nil {
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy+1); err != nil {
			ox, oy := v.Origin()
			if err := v.SetOrigin(ox, oy+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func PageDown(g *gocui.Gui, v *gocui.View) error {
	//Todo: Page down should use maxY
	_, cy := v.Cursor()
	line, _ := v.Line(cy + 25)
	if line == "" {
		return nil
	}

	if v != nil {
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy+25); err != nil {
			ox, oy := v.Origin()
			if err := v.SetOrigin(ox, oy+25); err != nil {
				return err
			}
		}
	}
	return nil
}

func CursorUp(g *gocui.Gui, v *gocui.View) error {
	if v != nil {
		ox, oy := v.Origin()
		cx, cy := v.Cursor()
		if err := v.SetCursor(cx, cy-1); err != nil && oy > 0 {
			if err := v.SetOrigin(ox, oy-1); err != nil {
				return err
			}
		}
	}
	return nil
}

func MainView(g *gocui.Gui, v *gocui.View) error {
	views := g.Views()
	for i := 0; i < len(views); i++ {
		if views[i].Name() == "side" || views[i].Name() == "main" || views[i].Name() == "log" || views[i].Name() == "legend" {
			continue
		} else {
			if err := g.DeleteView(views[i].Name()); err != nil {
				return err
			}
		}
	}
	if _, err := g.SetCurrentView("main"); err != nil {
		return err
	}
	UpdateLegend(g, mainlegend)
	return nil
}

func SaveSecret(g *gocui.Gui, v *gocui.View) error {
	x, _ := g.View("main")

	var secretpath, secret string
	var err error

	v, _ = g.View("editsecret")
	secret = v.Buffer()

	if editmode == "Editing" {
		_, cy := x.Cursor()
		if secretpath, err = x.Line(cy); err != nil {
			secretpath = ""
		}
		if !checkForChanges(g, secretpath) {
			UpdateLog(g, fmt.Sprintf("ERROR: Value of secret changed while editing. Write cancelled."))
			v, _ := g.View("main")
			DeletePrompt(g, v)
			return nil
		}
	} else if editmode == "Writing" {
		x, _ := g.View("addkeyprompt")
		secretpath = x.Buffer()
		secretpath = strings.TrimSpace(secretpath)
	}

	var mdata map[string]interface{}

	err = json.Unmarshal([]byte(secret), &mdata)
	if err != nil {
		UpdateLog(g, err.Error())
		UpdateLog(g, "ERROR: Write cancelled")
		g.DeleteView("saveprompt")
		if _, err := g.SetCurrentView("editsecret"); err != nil {
			return err
		}
		UpdateLegend(g, editlegend)
		return nil
	}

	err = api.Write(secretpath, mdata)

	if err != nil {
		UpdateLog(g, err.Error())
	} else {
		UpdateLog(g, fmt.Sprintf("Wrote secret contents to %s", secretpath))
		DeletePrompt(g, v)
	}

	return nil
}

func DeletePrompt(g *gocui.Gui, v *gocui.View) error {
	g.DeleteView("addkeyprompt")
	g.DeleteView("saveprompt")
	g.DeleteView("editsecret")
	g.DeleteView("secret")
	if _, err := g.SetCurrentView("main"); err != nil {
		return err
	}
	UpdateLegend(g, mainlegend)
	v, _ = g.View("side")
	GetLine(g, v)

	return nil
}

func checkForChanges(g *gocui.Gui, secretpath string) bool {
	secretbuffer, _ := g.View("secret")
	originalsecret := secretbuffer.Buffer()

	var odata map[string]interface{}

	err := json.Unmarshal([]byte(originalsecret), &odata)
	if err != nil {
		fmt.Println("error:", err)
	}
	return api.ComparePathToValue(secretpath, odata)
}

func SavePrompt(g *gocui.Gui, v *gocui.View) error {
	var secretpath string
	var err error

	x, _ := g.View("main")
	if editmode == "Editing" {
		_, cy := x.Cursor()
		if secretpath, err = x.Line(cy); err != nil {
			secretpath = ""
		}
	} else if editmode == "Writing" {
		x, _ := g.View("addkeyprompt")
		secretpath = x.Buffer()
		secretpath = strings.TrimSpace(secretpath)
	}
	secretlength := len(secretpath) + 19

	maxX, maxY := g.Size()
	v = CreateView(g, "saveprompt", maxX/2-secretlength/2, maxY/2, maxX/2+secretlength/2, maxY/2+2)
	fmt.Fprintln(v, "Overwrite "+secretpath+"? (y/n)")
	return nil
}
