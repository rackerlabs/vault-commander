package ui

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/jroimartin/gocui"
	termbox "github.com/nsf/termbox-go"
)

type lineEditor struct {
	gocuiEditor gocui.Editor
}

var le lineEditor

func (e *lineEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) {
	switch {
	case key == gocui.KeyArrowRight:
		x, _ := v.Cursor()
		if x >= len(v.ViewBuffer())-3 {
			return
		}

	case key == gocui.KeyArrowDown:
		return

	case key == gocui.KeyHome:
		v.SetCursor(0, 0)
		return

	case key == gocui.KeyEnd:
		v.SetCursor(len(v.ViewBuffer())-2, 0)
		return
	}

	e.gocuiEditor.Edit(v, key, ch, mod)
}

func OpenEditor(g *gocui.Gui, v *gocui.View) error {
	file, err := ioutil.TempFile(os.TempDir(), "vault-commander-")
	if err != nil {
		log.Panicln(err)
	}
	defer os.Remove(file.Name())

	sbuffer, _ := g.View("editsecret")
	val := sbuffer.Buffer()
	if val != "" {
		fmt.Fprint(file, val)
	}
	file.Close()

	info, err := os.Stat(file.Name())
	if err != nil {
		log.Panicln(err)
	}

	syseditor := os.Getenv("EDITOR")
	if syseditor == "" {
		syseditor = "vim"
	}

	cmd := exec.Command(syseditor, file.Name())
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	termbox.Close()
	err = cmd.Run()
	termbox.Init()

	// sync termbox to reset console settings
	// this is required because the external editor can modify the console
	defer g.Execute(func(_ *gocui.Gui) error {
		termbox.Close()
		termbox.Init()
		return err
	})
	if err != nil {
		log.Panicln("oh noe!")
	}

	newInfo, err := os.Stat(file.Name())
	if err != nil || newInfo.ModTime().Before(info.ModTime()) {
		log.Panicln(err)
	}

	newVal, err := ioutil.ReadFile(file.Name())
	if err != nil {
		log.Panicln(err)
	}

	v.SetCursor(0, 0)
	v.SetOrigin(0, 0)
	v.Clear()
	fmt.Fprint(v, strings.TrimSpace(string(newVal)))
	return nil
}
