package command

import "testing"

func TestVersion(t *testing.T) {
	f := newFake(t, 99)
	cmd, err := newVersion(parse(t, "version", "-v"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if code, err := cmd.Run(f.env); code != ExitOK || err != nil || f.stdout.String() != "mergediff 2026-11-19 (a1b2c3d)\n" {
		t.Fatalf("code %d, err %v, stdout %q", code, err, f.stdout.String())
	}

	if cmd, err := newVersion(parse(t, "version", "--help"), nil); err != nil || cmd != (Help{}) {
		t.Errorf("version --help = %#v, %v; want the general help", cmd, err)
	}
	for _, tt := range []struct {
		args []string
		msg  string
	}{
		{[]string{"version", "x"}, "version takes no arguments, got 1"},
		{[]string{"version", "-o", "m"}, "-o is not valid with version"},
		{[]string{"version", "--window-state", "normal"}, "--window-state is not valid with version"},
	} {
		opt := parse(t, tt.args...)
		_, err := newVersion(opt, opt.Positionals()[1:])
		wantUsageError(t, err, tt.msg)
	}
}
