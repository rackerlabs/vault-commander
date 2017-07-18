package main

import (
	"flag"
	"log"

	"github.com/jroimartin/gocui"
	"github.com/rackerlabs/vault-commander/ui"
)

var mp string
var sidelegend = "↑ - cursor up\n↓ - cursor down\nTab - switch windows\nRet - select mount"
var mainlegend = "Tab - switch windows\nRet - view secret\na - add secret\nd - delete secret\nSpace - page down"
var secretlegend = "e - edit secret\nq - quit view"
var editlegend = "C-l - Open in $EDITOR\nC-x - quit don't save\nC-s - save"

func init() {
	flag.StringVar(&mp, "mount", "", "Vault Mount")
}

func main() {
	flag.Parse()

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	g.Cursor = true

	g.SetManagerFunc(ui.MainScreen)

	if err := ui.Keybindings(g); err != nil {
		log.Panicln(err)
	}

	ui.InitScreen(g, mp)

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
}
