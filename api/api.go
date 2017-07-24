package api

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"

	vault "github.com/hashicorp/vault/api"
	"github.com/jroimartin/gocui"
)

var cl *vault.Client

func init() {
	c := vault.DefaultConfig()
	cl, _ = vault.NewClient(c)
	token := vaultToken()
	cl.SetToken(token)
}

func ListMounts() []string {
	mounts, _ := cl.Sys().ListMounts()
	var mountsWithKeys []string
	for k, l := range mounts {
		if mountCheck(k, l) {
			mountsWithKeys = append(mountsWithKeys, k)
		}
	}
	return mountsWithKeys
}

func ListAllKeys(keys []string, path string, parent string, v *gocui.View) {
	parent = fmt.Sprintf("%s%s", parent, path)
	vaultListing := listKeys(parent)
	for _, key := range vaultListing {
		if strings.HasSuffix(key.(string), "/") {
			ListAllKeys(keys, key.(string), parent, v)
		} else {
			fmt.Fprintln(v, parent+key.(string))
			keys = append(keys, parent+key.(string))
		}
	}
}

func listKeys(path string) []interface{} {
	resp, err := cl.Logical().List(path)
	if err != nil {
		fmt.Println(err)
	}

	if resp == nil {
		return nil
	}

	if slice, ok := resp.Data["keys"]; ok {
		return slice.([]interface{})
	} else {
		return nil
	}
	return nil
}

func mountCheck(path string, t *vault.MountOutput) bool {
	_, err := cl.Logical().List(path)
	if t.Type != "generic" && t.Type != "cubbyhole" {
		return false
	}

	if err != nil {
		return false
	}
	return true
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

func ReadValue(path string, v *gocui.View) string {
	resp, err := cl.Logical().Read(path)
	buf := new(bytes.Buffer)
	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "\t")

	if err != nil {
		fmt.Println(err)
	}

	enc.Encode(&resp.Data)
	return string(buf.String())
}

func ComparePathToValue(path string, contents map[string]interface{}) bool {
	resp, err := cl.Logical().Read(path)
	if err != nil {
		log.Panicln(err)
	}
	if reflect.DeepEqual(contents, resp.Data) {
		return true
	}
	return false
}

func Write(secretpath string, mdata map[string]interface{}) error {
	_, err := cl.Logical().Write(secretpath, mdata)
	return err
}

func Delete(secretpath string) error {
	_, err := cl.Logical().Delete(secretpath)
	return err
}
