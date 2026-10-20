package main

import (
	"os"
	"path/filepath"
	"testing"
)

// B18: the definition image was returned as a data: URI in ImagePath with an empty
// ImageDataBase64; the Pure Java SDK refused such a PREPEJ.
func TestInlineEmbedsDefinitionImageInImageDataBase64(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "images"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "images", "kakuin.png"), []byte{1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}
	a := &app{resources: dir, assetBase: "/api?action=asset&name="}
	var v any = map[string]any{"Definition": map[string]any{"Objects": []any{
		map[string]any{"Name": "Image1", "ImagePath": "/api?action=asset&name=kakuin.png", "ImageDataBase64": ""},
		map[string]any{"Name": "Image2", "ImagePath": "data:image/png;base64,BAUG", "ImageDataBase64": ""},
	}}, "Pages": []any{map[string]any{"Values": []any{map[string]any{"Name": "Image1", "Value": "/api?action=asset&name=kakuin.png"}}}}}
	if err := inline(v, a); err != nil {
		t.Fatal(err)
	}
	objects := v.(map[string]any)["Definition"].(map[string]any)["Objects"].([]any)
	for i, want := range []string{"AQID", "BAUG"} {
		o := objects[i].(map[string]any)
		if o["ImagePath"] != "" || o["ImageDataBase64"] != want {
			t.Fatalf("object %d: ImagePath=%q ImageDataBase64=%q", i, o["ImagePath"], o["ImageDataBase64"])
		}
	}
	value := v.(map[string]any)["Pages"].([]any)[0].(map[string]any)["Values"].([]any)[0].(map[string]any)["Value"]
	if value != "data:image/png;base64,AQID" {
		t.Fatalf("image value = %q", value)
	}
}