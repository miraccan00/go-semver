package semver

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestReadWriteVersionFromFile(t *testing.T) {
	file := "testdata/VERSION"
	_ = ioutil.WriteFile(file, []byte("1.2.3\n"), 0644)
	defer os.Remove(file)

	v, err := ReadVersionFromFile(file)
	if err != nil {
		t.Fatalf("ReadVersionFromFile failed: %v", err)
	}
	if v.Major != 1 || v.Minor != 2 || v.Patch != 3 {
		t.Errorf("unexpected version: %+v", v)
	}

	v.BumpPatch()
	if err := WriteVersionToFile(file, v); err != nil {
		t.Fatalf("WriteVersionToFile failed: %v", err)
	}
	v2, _ := ReadVersionFromFile(file)
	if v2.Patch != 4 {
		t.Errorf("patch bump failed: got %d", v2.Patch)
	}
}

func TestParseVersion(t *testing.T) {
	v, err := ParseVersion("2.5.9")
	if err != nil {
		t.Fatalf("ParseVersion failed: %v", err)
	}
	if v.Major != 2 || v.Minor != 5 || v.Patch != 9 {
		t.Errorf("unexpected version: %+v", v)
	}
}

func TestBumpByCommitMessage(t *testing.T) {
	cases := []struct {
		msg    string
		start  SemVer
		expect  SemVer
	}{
		{"fix: something", SemVer{1,2,3}, SemVer{1,2,4}},
		{"feat: new", SemVer{1,2,3}, SemVer{1,3,0}},
		{"feat!: breaking", SemVer{1,2,3}, SemVer{2,0,0}},
		{"fix!: breaking", SemVer{1,2,3}, SemVer{2,0,0}},
		{"BREAKING CHANGE: api", SemVer{1,2,3}, SemVer{2,0,0}},
		{"chore: nothing", SemVer{1,2,3}, SemVer{1,2,3}},
	}
	for _, c := range cases {
		v := c.start
		BumpByCommitMessage(&v, c.msg)
		if v != c.expect {
			t.Errorf("msg %q: got %+v, want %+v", c.msg, v, c.expect)
		}
	}
}
