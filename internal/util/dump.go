package util

import (
	"fmt"
	"os"

	"github.com/yassinebenaid/godump"
)

type MonoColor struct {
	R, G, B int
}

func (m MonoColor) Apply(s string) string {
	return fmt.Sprintf("%s", s)
}

func Dump(v any) error {
	var d godump.Dumper

	d.Theme = godump.Theme{}
	return d.Println(v)
}

func DumpFile(v any, filename string) error {
	var d godump.Dumper

	d.Theme = godump.Theme{}
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return d.Fprintln(f, v)
}
