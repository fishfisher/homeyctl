package cmd

import (
	"encoding/json"
	"net/http"
	"testing"
)

// A house shaped like a real one: devices live in rooms, while the floor and
// the house above them hold none directly.
const testZones = `{
	"home":  {"id":"home","name":"Home","parent":""},
	"floor": {"id":"floor","name":"Main floor","parent":"home"},
	"bath":  {"id":"bath","name":"Bath ","parent":"floor"},
	"kids":  {"id":"kids","name":"Kids","parent":"floor"},
	"yard":  {"id":"yard","name":"Yard","parent":"home"}
}`

const testDevices = `{
	"d1":{"id":"d1","name":"Bath light","class":"light","zone":"bath"},
	"d2":{"id":"d2","name":"Kids lamp","class":"light","zone":"kids"},
	"d3":{"id":"d3","name":"Yard socket","class":"socket","zone":"yard"}
}`

func listDevicesInZone(t *testing.T, zone, match string) []Device {
	t.Helper()
	testFlowServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/manager/zones/zone/":
			_, _ = w.Write([]byte(testZones))
		case "/api/manager/devices/device/":
			_, _ = w.Write([]byte(testDevices))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	oldJSON, oldMatch := jsonFlag, devicesMatchFilter
	jsonFlag, devicesMatchFilter = true, match
	t.Cleanup(func() {
		jsonFlag, devicesMatchFilter = oldJSON, oldMatch
		_ = devicesListCmd.Flags().Set("zone", "")
	})
	_ = devicesListCmd.Flags().Set("zone", zone)
	out := captureStdout(t, func() {
		if err := devicesListCmd.RunE(devicesListCmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	var got []Device
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("invalid JSON %q: %v", out, err)
	}
	return got
}

func deviceIDs(devices []Device) []string {
	ids := make([]string, len(devices))
	for i, d := range devices {
		ids[i] = d.ID
	}
	return ids
}

func TestDevicesZoneIncludesSubzones(t *testing.T) {
	// "Main floor" holds no devices itself; its rooms do.
	got := deviceIDs(listDevicesInZone(t, "Main floor", ""))
	if len(got) != 2 || got[0] != "d1" || got[1] != "d2" {
		t.Fatalf("floor = %v, want [d1 d2] (sorted by name, yard excluded)", got)
	}
}

func TestDevicesZoneRootListsEverything(t *testing.T) {
	if got := listDevicesInZone(t, "Home", ""); len(got) != 3 {
		t.Fatalf("root zone = %v, want all 3", deviceIDs(got))
	}
}

func TestDevicesZoneMatchesNameWithStraySpace(t *testing.T) {
	// The zone is stored as "Bath " but shown and typed as "Bath".
	got := deviceIDs(listDevicesInZone(t, "bath", ""))
	if len(got) != 1 || got[0] != "d1" {
		t.Fatalf("bath = %v, want [d1]", got)
	}
}

func TestDevicesZoneCombinesWithMatch(t *testing.T) {
	got := deviceIDs(listDevicesInZone(t, "Home", "lamp"))
	if len(got) != 1 || got[0] != "d2" {
		t.Fatalf("home+lamp = %v, want [d2]", got)
	}
}

func TestZoneSubtreeSurvivesParentCycle(t *testing.T) {
	zones := map[string]Zone{
		"a": {ID: "a", Parent: "b"},
		"b": {ID: "b", Parent: "a"},
	}
	if got := zoneSubtree(zones, "a"); !got["a"] || !got["b"] || len(got) != 2 {
		t.Fatalf("subtree = %v", got)
	}
}
