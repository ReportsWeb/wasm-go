package reportsweb

import "testing"

func TestIndicesSnapshotAndFailedDefinition(t *testing.T) {
	b := New()
	d := Map{"Version": "1", "Objects": []any{}}
	if err := b.SetDefinition(d); err != nil {
		t.Fatal(err)
	}
	bad := Map{"Version": "1", "Objects": []any{func() {}}}
	if b.PageStart(bad) == nil {
		t.Fatal("unserializable definition accepted")
	}
	if err := b.PageStart(nil); err != nil {
		t.Fatal("failed start changed state", err)
	}
	x, y := 0, 0
	for _, err := range []error{
		b.SetValueAt("x", "bad", -1, true), b.SetRepeatedValue("x", "bad", -1, 0, true),
		b.ChangeRepeatedAttributes("x", Map{}, 0, &x, nil, "DynamicText"),
		b.ChangeRepeatedAttributes("x", Map{}, 0, nil, &y, "DynamicText"),
	} {
		if err == nil {
			t.Fatal("invalid indices accepted")
		}
	}
	if err := b.SetValue("x", "original"); err != nil {
		t.Fatal(err)
	}
	if err := b.PageEnd(); err != nil {
		t.Fatal(err)
	}
	v, _ := b.Value()
	v["Definition"].(Map)["Version"] = "changed"
	v["Pages"].([]any)[0].(Map)["Values"].([]any)[0].(Map)["Value"] = "changed"
	again, _ := b.Value()
	p := again["Pages"].([]any)[0].(Map)
	if again["Definition"].(Map)["Version"] != "1" || p["Index"] != 1 || len(p["Values"].([]any)) != 1 || p["Values"].([]any)[0].(Map)["Value"] != "original" {
		t.Fatal("snapshot aliases builder or failed calls changed it", again)
	}
}

