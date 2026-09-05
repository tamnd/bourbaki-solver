package main

import (
	"strings"
	"testing"
)

// A route asked for by name and not reachable over ssh is not the route file
// being wrong, and the two send you to different places to fix it.
func TestAskingProbeForARouteWithNoBoxBlamesTheSelectionAndNotTheFile(t *testing.T) {
	flags := fleetFlags{only: "codex,zen"}
	err := flags.noSSHHost("/a/routes.json", "")
	if err == nil {
		t.Fatal("an empty selection was allowed to go on")
	}
	if !strings.Contains(err.Error(), "codex,zen") {
		t.Errorf("%q does not name what was asked for", err)
	}
	if strings.Contains(err.Error(), "/a/routes.json") {
		t.Errorf("%q blames the route file, which holds ssh routes and was not asked for them", err)
	}
}

func TestWithNothingSelectedTheRouteFileIsStillWhatIsNamed(t *testing.T) {
	var flags fleetFlags
	err := flags.noSSHHost("/a/routes.json", "enabled ")
	if err == nil {
		t.Fatal("a route file with no ssh host was allowed to go on")
	}
	if !strings.Contains(err.Error(), "/a/routes.json") {
		t.Errorf("%q does not name the file that has nothing in it", err)
	}
	if !strings.Contains(err.Error(), "enabled") {
		t.Errorf("%q drops the enabled qualifier, and a disabled ssh route is not one to use", err)
	}
}
