package netplugin

import (
	"testing"
)

// fixedPSK serves a fixed passphrase.
func fixedAgent(psk string) *Agent {
	b := &Backend{}
	b.setPSK(psk)
	return NewAgent(b)
}

func TestGetSecretsReturnsPSK(t *testing.T) {
	a := fixedAgent("hunter2")
	secrets, derr := a.GetSecrets(nil, "", "802-11-wireless-security", nil, 0)
	if derr != nil {
		t.Fatalf("GetSecrets = err %v", derr)
	}
	set := secrets["802-11-wireless-security"]
	if set == nil {
		t.Fatal("missing setting key")
	}
	if got, _ := set["psk"].Value().(string); got != "hunter2" {
		t.Errorf("psk = %q", got)
	}
}

func TestGetSecretsEmptyWhenNoPSK(t *testing.T) {
	a := fixedAgent("")
	secrets, derr := a.GetSecrets(nil, "", "802-11-wireless-security", nil, 0)
	if derr != nil {
		t.Fatalf("GetSecrets = err %v", derr)
	}
	if len(secrets) != 0 {
		t.Errorf("secrets = %v, want empty", secrets)
	}
}

func TestGetSecretsIgnoresOtherSettings(t *testing.T) {
	a := fixedAgent("hunter2")
	secrets, _ := a.GetSecrets(nil, "", "802-1x", nil, 0)
	if len(secrets) != 0 {
		t.Errorf("secrets for 802-1x = %v, want empty", secrets)
	}
}

func TestSecretKeyIsVariantString(t *testing.T) {
	a := fixedAgent("x")
	secrets, _ := a.GetSecrets(nil, "", "802-11-wireless-security", nil, 0)
	v := secrets["802-11-wireless-security"]["psk"]
	// dbus.MakeVariant of a string yields a string value.
	if _, ok := v.Value().(string); !ok {
		t.Errorf("psk variant value = %T", v.Value())
	}
}
