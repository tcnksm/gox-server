package main

import "testing"

func TestGuessPlatform(t *testing.T) {

	tests := []struct {
		in             string
		extOS, extArch string
	}{
		{
			in:      "Darwin i386",
			extOS:   "darwin",
			extArch: "amd64",
		},

		{
			in:      "Linux x86_64",
			extOS:   "linux",
			extArch: "amd64",
		},

		{
			in:      "Linux i386",
			extOS:   "linux",
			extArch: "386",
		},

		{
			in:      "Windows",
			extOS:   "windows",
			extArch: "386",
		},
	}

	for i, tt := range tests {
		targetOS, targetArch := guessPlatform(tt.in)

		if targetOS != tt.extOS {
			t.Errorf("#%d expects %s to eq %s", i, targetOS, tt.extOS)
		}

		if targetArch != tt.extArch {
			t.Errorf("#%d expects %s to eq %s", i, targetArch, tt.extArch)
		}
	}
}
