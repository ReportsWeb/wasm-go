package reportsweb

import "testing"

func TestLifecycleAndValues(t *testing.T) {
	d := Map{"Version": "1", "CoordinateUnit": "mm", "Objects": []any{}}
	b := New()
	if err := b.SetDefinition(d); err != nil {
		t.Fatal(err)
	}
	if err := b.PageStart(nil); err != nil {
		t.Fatal(err)
	}
	if err := b.SetValue(" total ", 12.5); err != nil {
		t.Fatal(err)
	}
	if err := b.ChangeAttributes("total", Map{"bold": true}, 0); err != nil {
		t.Fatal(err)
	}
	if err := b.PageEnd(); err != nil {
		t.Fatal(err)
	}
	v, err := b.Value()
	if err != nil {
		t.Fatal(err)
	}
	p := v["Pages"].([]any)[0].(Map)
	got := p["Values"].([]any)[0].(Map)
	if got["Name"] != "total" || got["Value"] != "12.5" {
		t.Fatalf("unexpected value: %#v", got)
	}
}
func TestRejectsInvalidState(t *testing.T) {
	if New().PageStart(nil) == nil {
		t.Fatal("definition must be required")
	}
	d := Map{"Version": "1", "CoordinateUnit": "mm", "Objects": []any{}}
	b := New()
	_ = b.SetDefinition(d)
	_ = b.PageStart(nil)
	if b.SetValue("x", Map{"bad": 1}) == nil {
		t.Fatal("object value must fail")
	}
	if _, e := b.Value(); e == nil {
		t.Fatal("unfinished page must fail")
	}
}
