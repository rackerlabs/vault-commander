package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	vault "github.com/hashicorp/vault/api"
	"github.com/jroimartin/gocui"
)

var cl *vault.Client
var keys []string

func main() {
	c := vault.DefaultConfig()
	cl, _ = vault.NewClient(c)
	token := vaultToken()
	cl.SetToken(token)

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	g.Cursor = true

	g.SetManagerFunc(layout)

	if err := keybindings(g); err != nil {
		log.Panicln(err)
	}

	newLayout(g)

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
}

func listMounts(cl *vault.Client) []string {
	mounts, _ := cl.Sys().ListMounts()
	var mountsWithKeys []string
	for k, _ := range mounts {
		if mountCheck(k, cl) {
			mountsWithKeys = append(mountsWithKeys, k)
		}
	}
	return mountsWithKeys
}

func getLine(g *gocui.Gui, v *gocui.View) error {
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
	listAllKeys(keys, l, "", v)
	g.SetCurrentView("main")
	return nil
}
func viewSecret(g *gocui.Gui, v *gocui.View) error {
	var l string
	var err error

	_, cy := v.Cursor()
	if l, err = v.Line(cy); err != nil {
		l = ""
	}

	maxX, maxY := g.Size()
	if v, err := g.SetView("secret", -1, -1, maxX, maxY); err != nil {
		v.Frame = false

		if err != gocui.ErrUnknownView {
			return err
		}

		if _, err := g.SetCurrentView("secret"); err != nil {
			return err
		}

		readValue(l, cl, v)
	}
	return nil
}

func readValue(path string, cl *vault.Client, v *gocui.View) {
	resp, err := cl.Logical().Read(path)
	if err != nil {
		fmt.Println(err)
	}

	for key, value := range resp.Data {
		fmt.Fprintln(v, key, "\t", value)
	}
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	if v, err := g.SetView("side", 1, 1, 30, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.Frame = true
		v.Title = "Mounts"
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		for _, mount := range listMounts(cl) {
			fmt.Fprintln(v, mount)
		}
	}
	if v, err := g.SetView("main", 30, 1, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = false
		v.Frame = true
		v.Title = "Keys"
		v.Wrap = true
	}
	return nil
}

func newLayout(g *gocui.Gui) error {
	maxX, maxY := g.Size()

	if v, err := g.SetView("side", 1, 1, 30, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Highlight = true
		v.Frame = true
		v.Title = "Mounts"
		v.SelBgColor = gocui.ColorGreen
		v.SelFgColor = gocui.ColorBlack
		for _, mount := range listMounts(cl) {
			fmt.Fprintln(v, mount)
		}
	}
	if v, err := g.SetView("main", 30, 1, maxX-1, maxY-1); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = false
		v.Frame = true
		v.Title = "Keys"
		v.Wrap = true
	}
	g.SetCurrentView("side")
	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func keybindings(g *gocui.Gui) error {
	if err := g.SetKeybinding("side", gocui.KeyTab, gocui.ModNone, nextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyEnter, gocui.ModNone, getLine); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyEnter, gocui.ModNone, viewSecret); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyTab, gocui.ModNone, nextView); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("", 'q', gocui.ModNone, delSecret); err != nil {
		return err
	}
	return nil
}

func cursorDown(g *gocui.Gui, v *gocui.View) error {
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

func cursorUp(g *gocui.Gui, v *gocui.View) error {
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

func nextView(g *gocui.Gui, v *gocui.View) error {
	if v == nil || v.Name() == "side" {
		_, err := g.SetCurrentView("main")
		return err
	}
	_, err := g.SetCurrentView("side")
	return err
}

func vaultToken() string {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}

	path := filepath.Join(usr.HomeDir, ".vault-token")

	file, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	var vault_token string

	// vault_token file is only one line
	for scanner.Scan() {
		vault_token = scanner.Text()
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return vault_token
}

func listAllKeys(keys []string, path string, parent string, v *gocui.View) {
	parent = fmt.Sprintf("%s%s", parent, path)
	vaultListing := listKeys(parent, cl)
	for _, key := range vaultListing {
		if strings.HasSuffix(key.(string), "/") {
			listAllKeys(keys, key.(string), parent, v)
		} else {
			fmt.Fprintln(v, parent+key.(string))
			keys = append(keys, parent+key.(string))
		}
	}
}

func listKeys(path string, cl *vault.Client) []interface{} {
	resp, err := cl.Logical().List(path)
	if err != nil {
		fmt.Println(err)
	}

	if slice, ok := resp.Data["keys"]; ok {
		return slice.([]interface{})
	} else {
		return nil
	}
	return nil
}

func mountCheck(path string, cl *vault.Client) bool {
	resp, err := cl.Logical().List(path)
	if err != nil {
		fmt.Println(err)
	}

	if resp == nil {
		return false
	} else {
		return true
	}
}

func delSecret(g *gocui.Gui, v *gocui.View) error {
	err := g.DeleteView("secret")
	if err != nil {
		return err
	}
	_, err = g.SetCurrentView("main")
	if err != nil {
		return err
	}
	return nil
}
