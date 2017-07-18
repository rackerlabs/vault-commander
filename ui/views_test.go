package ui

import (
	"fmt"
	"testing"

	"github.com/jroimartin/gocui"
)

func TestView(t *testing.T) {
	g, _ := gocui.NewGui(gocui.OutputNormal)
	v := CreateView(g, "test", -1, -1, 10, 10)
	fmt.Println(v)

	//	expected := "yes"
	//	actual := "no"
	//	if actual != expected {
	//		t.Errorf("Test failed, expected: '%s', got:  '%s'", expected, actual)
	//	}
}
