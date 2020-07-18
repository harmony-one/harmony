package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestStringFlag(t *testing.T) {
	tests := []struct {
		flagName   string
		deprecated string
		hidden     bool
		defValue   string
		args       []string
		expValue   string
	}{
		{
			defValue: "default",
			args:     []string{},
			expValue: "default",
		},
		{
			defValue: "default",
			args:     []string{"--test", "custom"},
			expValue: "custom",
		},
		{
			defValue:   "default",
			args:       []string{},
			deprecated: "deprecated",
			expValue:   "default",
		},
		{
			defValue: "default",
			args:     []string{},
			hidden:   true,
			expValue: "default",
		},
	}
	for i, test := range tests {
		flagName := "test"
		flag := StringFlag{
			Name:       flagName,
			Deprecated: test.deprecated,
			Hidden:     test.hidden,
			DefValue:   test.defValue,
		}
		var (
			gotValue string
			err      error
		)
		cmd := makeTestCommand(func(cmd *cobra.Command, args []string) {
			gotValue, err = cmd.Flags().GetString(flagName)
			if err != nil {
				t.Fatalf("Test %v: %v", i, err)
			}
		})
		if err := flag.RegisterTo(cmd.Flags()); err != nil {
			t.Fatalf("Test %v: %v", i, err)
		}
		cmd.SetArgs(test.args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Test %v: %v", i, err)
		}
		if gotValue != test.expValue {
			t.Errorf("Test %v: unexpected flag value %v / %v", i, gotValue, test.expValue)
		}
	}
}

func makeTestCommand(run func(cmd *cobra.Command, args []string)) *cobra.Command {
	return &cobra.Command{Run: run}
}
