package cmd

import "testing"

func TestHuiUpdateRetainsLegacyUpgradeCommand(t *testing.T) {
	for _, name := range []string{"update", "upgrade"} {
		command, _, err := rootCmd.Find([]string{name})
		if err != nil || command != upgradeCmd {
			t.Fatalf("%s does not resolve to verified upgrader: %v", name, err)
		}
	}
	if rootCmd.Use != "hui" {
		t.Fatalf("public CLI=%q", rootCmd.Use)
	}
}
