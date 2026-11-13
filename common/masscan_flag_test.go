package common

import "testing"

func TestValidateMasscanExtraArgs(t *testing.T) {
	valid := []string{
		"",
		"--wait 3",
		"--wait=3 --adapter-ip 10.0.0.1",
	}
	for _, raw := range valid {
		if err := validateMasscanExtraArgs(raw); err != nil {
			t.Errorf("validateMasscanExtraArgs(%q) = %v, want nil", raw, err)
		}
	}

	invalid := []string{
		"-p 80",
		"-p80,443",
		"--ports 80",
		"--rate 1000",
		"--rate=1000",
		"-iL hosts.txt",
		"--includefile hosts.txt",
		"--exclude 10.0.0.1",
		"--excludefile excl.txt",
	}
	for _, raw := range invalid {
		if err := validateMasscanExtraArgs(raw); err == nil {
			t.Errorf("validateMasscanExtraArgs(%q) = nil, want error", raw)
		}
	}
}
