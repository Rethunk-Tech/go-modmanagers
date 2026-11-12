package r2modman

import (
	"bytes"
	"maps"
	"slices"
	"testing"
)

func FuzzParseVersion(f *testing.F) {
	f.Add("1.2.3")
	f.Add("0.0.0")
	f.Add("10.20.30")
	f.Add("1.0.0")
	f.Fuzz(func(t *testing.T, s string) {
		v, err := ParseVersion(s)
		if err != nil {
			return
		}
		got, err := ParseVersion(v.String())
		if err != nil {
			t.Fatalf("String %q did not parse: %v", v.String(), err)
		}
		if got != v {
			t.Fatalf("round-trip %v != %v", got, v)
		}
	})
}

func FuzzWriteExport(f *testing.F) {
	seed, err := WriteExport(Export{
		ProfileName: "Default",
		Community:   "lethalcompany",
		Mods:        []Mod{{Name: "Owner-Mod", Version: Version{1, 0, 0}, Enabled: true}},
		Files:       map[string][]byte{"BepInEx/config/foo.cfg": []byte("a = 1\n")},
	})
	if err != nil {
		f.Fatal(err)
	}
	empty, err := WriteExport(Export{ProfileName: "Empty"})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add(empty)
	f.Add([]byte("PK"))
	f.Fuzz(func(t *testing.T, data []byte) {
		exp, err := ReadExport(data)
		if err != nil {
			return
		}
		out, err := WriteExport(exp)
		if err != nil {
			t.Fatalf("WriteExport after ReadExport: %v", err)
		}
		got, err := ReadExport(out)
		if err != nil {
			t.Fatalf("ReadExport of WriteExport: %v", err)
		}
		if got.ProfileName != exp.ProfileName || got.Community != exp.Community {
			t.Fatalf("header %q/%q != %q/%q", got.ProfileName, got.Community, exp.ProfileName, exp.Community)
		}
		if !slices.Equal(got.Mods, exp.Mods) {
			t.Fatalf("mods %#v != %#v", got.Mods, exp.Mods)
		}
		if !maps.EqualFunc(got.Files, exp.Files, bytes.Equal) {
			t.Fatalf("files %#v != %#v", got.Files, exp.Files)
		}
	})
}

func FuzzCodeKey(f *testing.F) {
	f.Add("01234567-89ab-cdef-0123-456789abcdef")
	f.Add(importLink + "01234567-89ab-cdef-0123-456789abcdef")
	f.Add(importLink + "01234567-89ab-cdef-0123-456789abcdef/")
	f.Add("not-a-key")
	f.Fuzz(func(t *testing.T, text string) {
		key, ok := CodeKey(text)
		if !ok {
			if IsCodeKey(text) {
				t.Fatal("IsCodeKey true when CodeKey rejected")
			}
			return
		}
		if !IsCodeKey(text) {
			t.Fatal("IsCodeKey false when CodeKey accepted")
		}
		if key == "" {
			t.Fatal("empty key")
		}
		if _, again := CodeKey(key); !again {
			t.Fatalf("accepted key %q is not itself a key", key)
		}
	})
}
