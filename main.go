package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	vault "github.com/hashicorp/vault/api"
	"github.com/jroimartin/gocui"
)

var cl *vault.Client
var keys []string

var sidelegend = "↑ - cursor up\n↓ - cursor down\nTab - switch windows\nRet - select mount"
var mainlegend = "↑ - cursor up\n↓ - cursor down\nTab - switch windows\nRet - view secret\nd - delete secret\nSpace - page down"
var secretlegend = "e - edit secret\nq - quit view"
var editlegend = "C-x - quit don't save\nC-s - save"

var mp string

func init() {
	flag.StringVar(&mp, "mount", "", "Vault Mount")
}

func main() {
	flag.Parse()
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
	if err := v.SetCursor(0, 0); err != nil {
		return err
	}
	if err := v.SetOrigin(0, 0); err != nil {
		return err
	}
	//cx, cy := v.Cursor()

	g.SetCurrentView("main")
	updateLegend(g, mainlegend)
	updateLog(g, fmt.Sprintf("Viewing secrets on %s mount", l))
	return nil
}

func getFlag(g *gocui.Gui) error {
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
	listAllKeys(keys, fmt.Sprintf("%s/", mp), "", v)
	g.SetCurrentView("main")
	updateLegend(g, mainlegend)
	updateLog(g, fmt.Sprintf("Viewing secrets on %s mount", mp))
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
	if v, err := g.SetView("secret", -1, -1, maxX, maxY-7); err != nil {
		v.Frame = false
		v.Wrap = true

		if err != gocui.ErrUnknownView {
			return err
		}

		if _, err := g.SetCurrentView("secret"); err != nil {
			return err
		}

		readValue(l, cl, v)
	}
	updateLegend(g, secretlegend)
	updateLog(g, fmt.Sprintf("Viewing secret contents of %s", l))
	return nil
}

func editSecret(g *gocui.Gui, v *gocui.View) error {
	secret := v.Buffer()
	maxX, maxY := g.Size()
	if v, err := g.SetView("editsecret", -1, -1, maxX, maxY-7); err != nil {
		v.Editable = true
		v.Frame = false
		v.Wrap = true

		if err != gocui.ErrUnknownView {
			return err
		}

		if _, err := g.SetCurrentView("editsecret"); err != nil {
			return err
		}

		fmt.Fprintln(v, secret)
	}
	updateLegend(g, editlegend)

	var secretpath string
	var err error
	x, _ := g.View("main")
	_, cy := x.Cursor()

	if secretpath, err = x.Line(cy); err != nil {
		secretpath = ""
	}

	updateLog(g, fmt.Sprintf("Editing secret contents of %s", secretpath))
	return nil
}

func cancelEdit(g *gocui.Gui, v *gocui.View) error {
	var secretpath string
	var err error
	x, _ := g.View("main")
	_, cy := x.Cursor()

	if secretpath, err = x.Line(cy); err != nil {
		secretpath = ""
	}

	updateLog(g, fmt.Sprintf("Canceled edit of %s", secretpath))
	homeView(g, v)
	return nil
}

func deleteKeyPrompt(g *gocui.Gui, v *gocui.View) error {
	x, _ := g.View("main")
	var secretpath string
	var err error

	_, cy := x.Cursor()
	if secretpath, err = x.Line(cy); err != nil {
		secretpath = ""
	}
	secretlength := len(secretpath) + 19

	maxX, maxY := g.Size()
	if v, err := g.SetView("deletekeyprompt", maxX/2-secretlength/2, maxY/2, maxX/2+secretlength/2, maxY/2+2); err != nil {
		v.Frame = true
		v.Title = "WARNING"

		if err != gocui.ErrUnknownView {
			return err
		}

		if _, err := g.SetCurrentView("deletekeyprompt"); err != nil {
			return err
		}

		fmt.Fprintln(v, "Delete "+secretpath+"? (y/n)")
	}
	return nil
}

func savePrompt(g *gocui.Gui, v *gocui.View) error {
	x, _ := g.View("main")
	var secretpath string
	var err error

	_, cy := x.Cursor()
	if secretpath, err = x.Line(cy); err != nil {
		secretpath = ""
	}
	secretlength := len(secretpath) + 19

	maxX, maxY := g.Size()
	if v, err := g.SetView("saveprompt", maxX/2-secretlength/2, maxY/2, maxX/2+secretlength/2, maxY/2+2); err != nil {
		v.Frame = true
		v.Title = "Save Changes"

		if err != gocui.ErrUnknownView {
			return err
		}

		if _, err := g.SetCurrentView("saveprompt"); err != nil {
			return err
		}

		fmt.Fprintln(v, "Overwrite "+secretpath+"? (y/n)")
	}
	return nil
}

func saveSecret(g *gocui.Gui, v *gocui.View) error {
	x, _ := g.View("main")
	secretbuffer, _ := g.View("secret")
	originalsecret := secretbuffer.Buffer()

	var secretpath string
	var err error

	_, cy := x.Cursor()
	if secretpath, err = x.Line(cy); err != nil {
		secretpath = ""
	}

	v, _ = g.View("editsecret")
	secret := v.Buffer()

	var mdata map[string]interface{}
	var odata map[string]interface{}

	err = json.Unmarshal([]byte(secret), &mdata)
	if err != nil {
		updateLog(g, err.Error())
		updateLog(g, "ERROR: Write cancelled")
		g.DeleteView("saveprompt")
		if _, err := g.SetCurrentView("editsecret"); err != nil {
			return err
		}
		updateLegend(g, editlegend)
		return nil
	}

	err = json.Unmarshal([]byte(originalsecret), &odata)
	if err != nil {
		fmt.Println("error:", err)
	}

	resp, err := cl.Logical().Read(secretpath)
	if err != nil {
		fmt.Println(err)
	}
	if !reflect.DeepEqual(odata, resp.Data) {
		updateLog(g, fmt.Sprintf("ERROR: Value of secret changed while editing. Write cancelled."))
	}

	_, err = cl.Logical().Write(secretpath, mdata)
	if err != nil {
		updateLog(g, err.Error())
	} else {
		updateLog(g, fmt.Sprintf("Wrote secret contents to %s", secretpath))
		deletePrompt(g, v)
	}

	return nil
}

func readValue(path string, cl *vault.Client, v *gocui.View) {
	resp, err := cl.Logical().Read(path)
	if err != nil {
		fmt.Println(err)
	}

	j, _ := json.MarshalIndent(resp.Data, "", "\t")
	fmt.Fprintln(v, string(j))
}

func updateLegend(g *gocui.Gui, legend string) {
	v, _ := g.View("legend")
	v.Clear()
	fmt.Fprintln(v, legend)
}

func updateLog(g *gocui.Gui, log string) {
	v, _ := g.View("log")
	t := time.Now()
	longForm := "2006-01-02 3:04pm (MST)"
	fmt.Fprintln(v, t.Format(longForm), log)
}

func layout(g *gocui.Gui) error {
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
		for _, mount := range listMounts(cl) {
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

func newLayout(g *gocui.Gui) error {
	layout(g)
	if mp != "" {
		getFlag(g)
	} else {
		g.SetCurrentView("side")
		updateLegend(g, sidelegend)
	}

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
	if err := g.SetKeybinding("deletekeyprompt", 'y', gocui.ModNone, deleteKey); err != nil {
		return err
	}
	if err := g.SetKeybinding("deletekeyprompt", 'n', gocui.ModNone, homeView); err != nil {
		return err
	}
	if err := g.SetKeybinding("main", 'd', gocui.ModNone, deleteKeyPrompt); err != nil {
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
	if err := g.SetKeybinding("main", gocui.KeySpace, gocui.ModNone, pageDown); err != nil {
		return err
	}
	if err := g.SetKeybinding("side", gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
		return err
	}
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		return err
	}
	if err := g.SetKeybinding("secret", 'q', gocui.ModNone, homeView); err != nil {
		return err
	}
	if err := g.SetKeybinding("editsecret", gocui.KeyCtrlX, gocui.ModNone, cancelEdit); err != nil {
		return err
	}
	if err := g.SetKeybinding("saveprompt", 'y', gocui.ModNone, saveSecret); err != nil {
		return err
	}
	if err := g.SetKeybinding("saveprompt", 'n', gocui.ModNone, deletePrompt); err != nil {
		return err
	}
	if err := g.SetKeybinding("editsecret", gocui.KeyCtrlS, gocui.ModNone, savePrompt); err != nil {
		return err
	}
	if err := g.SetKeybinding("secret", 'e', gocui.ModNone, editSecret); err != nil {
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

func pageDown(g *gocui.Gui, v *gocui.View) error {
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
		updateLegend(g, mainlegend)
		return err
	}
	_, err := g.SetCurrentView("side")
	updateLegend(g, sidelegend)
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
		//	fmt.Println(err)
	}

	if resp == nil {
		return false
	} else {
		return true
	}
}

func homeView(g *gocui.Gui, v *gocui.View) error {
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
	updateLegend(g, mainlegend)
	v, _ = g.View("side")
	getLine(g, v)
	return nil
}

func deletePrompt(g *gocui.Gui, v *gocui.View) error {
	g.DeleteView("saveprompt")
	g.DeleteView("editsecret")
	g.DeleteView("secret")
	if _, err := g.SetCurrentView("main"); err != nil {
		return err
	}
	updateLegend(g, mainlegend)
	return nil
}

func deleteKey(g *gocui.Gui, v *gocui.View) error {
	var secretpath string
	var err error
	x, _ := g.View("main")
	_, cy := x.Cursor()

	if secretpath, err = x.Line(cy); err != nil {
		secretpath = ""
	}
	_, err = cl.Logical().Delete(secretpath)
	if err != nil {
		log.Panicln(err)
	}
	updateLog(g, fmt.Sprintf("Deleted secret %s", secretpath))
	homeView(g, v)
	return nil
}
