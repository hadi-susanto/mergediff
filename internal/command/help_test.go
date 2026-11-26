package command

import (
	"strings"
	"testing"
)

func TestUsageListsSettingsFlags(t *testing.T) {
	for _, want := range []string{"--theme system|light|dark\n", "light/dark setting (default system)", "--window-state normal|maximized\n", "(default normal)"} {
		if !strings.Contains(Usage, want) {
			t.Errorf("usage is missing %q:\n%s", want, Usage)
		}
	}
}

func TestUsageExplainsReservedWords(t *testing.T) {
	for _, want := range []string{"help, version, settings and resolve are reserved words", "mergediff -- help other.txt", "mergediff -o MERGED -- help local remote"} {
		if !strings.Contains(Usage, want) {
			t.Errorf("usage is missing %q", want)
		}
	}
}

func TestHelp(t *testing.T) {
	tests := []struct {
		args []string // after "help"
		want string   // substring of stdout
	}{
		{nil, "mergediff resolve [--auto-apply]"},
		{[]string{"settings"}, "mergediff settings unset KEY"},
		{[]string{"resolve"}, "mergediff resolve [--auto-apply]"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			f := newFake(t, 99)
			cmd, err := newHelp(parse(t, "help"), tt.args)
			if err != nil {
				t.Fatal(err)
			}
			if code, err := cmd.Run(f.env); code != ExitOK || err != nil || !strings.Contains(f.stdout.String(), tt.want) {
				t.Fatalf("code %d, err %v, stdout %q", code, err, f.stdout.String())
			}
		})
	}
}

func TestHelpErrors(t *testing.T) {
	tests := []struct {
		args []string
		msg  string
	}{
		{[]string{"help", "nope"}, `unknown help topic "nope" (known: resolve, settings)`},
		{[]string{"help", "settings", "resolve"}, "help takes at most one topic, got 2"},
		{[]string{"help", "-o", "m"}, "-o is not valid with help"},
		{[]string{"help", "--auto-apply"}, "--auto-apply is not valid with help"},
		{[]string{"help", "--theme", "dark"}, "--theme is not valid with help"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			opt := parse(t, tt.args...)
			_, err := newHelp(opt, opt.Positionals()[1:])
			if ue := wantUsageError(t, err, tt.msg); ue.Usage != "" {
				t.Errorf("Usage = %q, want the general one", ue.Usage)
			}
		})
	}
}
