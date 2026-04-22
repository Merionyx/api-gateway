package convertfmt

import "testing"

func TestNormalizeExtFromSourcePath(t *testing.T) {
	t.Parallel()
	if _, err := NormalizeExtFromSourcePath("a.md"); err == nil {
		t.Fatal("md")
	}
	if e, err := NormalizeExtFromSourcePath("x.YML"); err != nil || e != ".yml" {
		t.Fatalf("%s %v", e, err)
	}
}

func TestOutputExt(t *testing.T) {
	t.Parallel()
	if e, err := OutputExt("a.yaml", "json"); err != nil || e != ".json" {
		t.Fatalf("%s %v", e, err)
	}
	if e, err := OutputExt("a.yaml", ""); err != nil || e != ".yaml" {
		t.Fatalf("%s %v", e, err)
	}
	if _, err := OutputExt("a.yaml", "xml"); err == nil {
		t.Fatal()
	}
}

func TestConvert_yamlToJson(t *testing.T) {
	t.Parallel()
	out, err := Convert([]byte("a: 1\n"), ".yaml", "json")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 3 {
		t.Fatalf("%q", out)
	}
}

func TestConvert_jsonToYaml(t *testing.T) {
	t.Parallel()
	out, err := Convert([]byte(`{"a":1}`), ".json", "yaml")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) < 2 {
		t.Fatalf("%q", out)
	}
}

func TestConvert_errors(t *testing.T) {
	t.Parallel()
	if _, err := Convert([]byte("x"), ".yaml", "bad"); err == nil {
		t.Fatal("bad target")
	}
	if _, err := Convert([]byte("x"), ".md", "yaml"); err == nil {
		t.Fatal("bad source ext")
	}
	if _, err := Convert([]byte(":\n"), ".yaml", "json"); err == nil {
		t.Fatal("parse yaml")
	}
}
