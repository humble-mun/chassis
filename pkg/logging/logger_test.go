package logging

import (
	"flag"
	"testing"

	"github.com/spf13/pflag"
)

func TestInitRegistersVKlogFlag(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	pfs := pflag.NewFlagSet("test", pflag.ContinueOnError)

	Init(fs, pfs)

	if fs.Lookup("v") == nil {
		t.Fatal("expected klog -v flag to be registered on the go flag set")
	}
	if pfs.Lookup("vklog") == nil {
		t.Fatal("expected --vklog flag to be registered on the pflag set")
	}
}

func TestNewLogger(t *testing.T) {
	logger := NewLogger()
	if logger.GetSink() == nil {
		t.Fatal("expected NewLogger to return a logger with a non-nil sink")
	}
}
