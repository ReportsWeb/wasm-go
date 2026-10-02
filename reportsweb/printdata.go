package reportsweb

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
)

const Format = "Reports.net PrintData"
const Version = "1.0"

type Map = map[string]any

type Builder struct {
	definition Map
	pages      []any
	page       Map
}

func New() *Builder { return &Builder{} }

func clone(value any) (any, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var out any
	err = json.Unmarshal(b, &out)
	return out, err
}

func ValidateDefinition(d Map) error {
	_, objects := d["Objects"].([]any)
	unit, _ := d["CoordinateUnit"].(string)
	portable := d["Version"] != nil && objects && (unit == "" || strings.EqualFold(unit, "mm") || strings.EqualFold(unit, "px"))
	pages, webPages := d["pages"].([]any)
	web := d["format"] == "pao-reports-web" && fmt.Sprint(d["version"]) == "1" && webPages && len(pages) > 0
	if !portable && !web {
		return errors.New("invalid report definition")
	}
	return nil
}

func (b *Builder) SetDefinition(d Map) error {
	if b.page != nil {
		return errors.New("cannot change the definition inside a page")
	}
	if err := ValidateDefinition(d); err != nil {
		return err
	}
	v, err := clone(d)
	if err != nil {
		return err
	}
	if b.definition != nil {
		for _, raw := range b.pages {
			p := raw.(Map)
			if p["Definition"] == nil {
				p["Definition"], _ = clone(b.definition)
			}
		}
	}
	b.definition = v.(Map)
	return nil
}

func (b *Builder) PageStart(definition Map) error {
	if b.page != nil {
		return errors.New("call PageEnd before starting another page")
	}
	if definition != nil {
		if err := ValidateDefinition(definition); err != nil {
			return err
		}
	}
	if definition == nil && b.definition == nil {
		return errors.New("set a definition first")
	}
	var d any
	if definition != nil {
		var err error
		d, err = clone(definition)
		if err != nil {
			return err
		}
	}
	b.page = Map{"Index": len(b.pages) + 1, "Definition": d, "Values": []any{}, "DynamicAttributes": []any{}}
	return nil
}

func scalar(value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch v := value.(type) {
	case string:
		return v, nil
	case bool:
		return fmt.Sprint(v), nil
	case float32:
		if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
			return nil, errors.New("numbers must be finite")
		}
		return fmt.Sprint(v), nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("numbers must be finite")
		}
		return fmt.Sprint(v), nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
		return fmt.Sprint(v), nil
	}
	return nil, errors.New("values must be scalar")
}
func required(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", errors.New("a name is required")
	}
	return v, nil
}
func (b *Builder) add(name, kind string, value any, index int, drawing bool, ix, iy *int) error {
	if b.page == nil {
		return errors.New("call PageStart first")
	}
	if err := validateIndices(index, ix, iy); err != nil {
		return err
	}
	n, e := required(name)
	if e != nil {
		return e
	}
	k, e := required(kind)
	if e != nil {
		return e
	}
	v, e := scalar(value)
	if e != nil {
		return e
	}
	item := Map{"Name": n, "Type": k, "Index": index, "Drawing": drawing, "Value": v}
	if ix != nil {
		item["IndexX"] = *ix
		item["IndexY"] = *iy
	}
	b.page["Values"] = append(b.page["Values"].([]any), item)
	return nil
}
func (b *Builder) SetValue(name string, value any) error { return b.SetValueAt(name, value, 0, true) }
func (b *Builder) SetValueAt(name string, value any, index int, drawing bool) error {
	return b.add(name, b.inferredType(name), value, index, drawing, nil, nil)
}
func (b *Builder) SetRepeatedValue(name string, value any, x, y int, drawing bool) error {
	return b.add(name, b.inferredType(name), value, 0, drawing, &x, &y)
}
func (b *Builder) SetTypedValue(name, kind string, value any, index int, drawing bool) error {
	return b.add(name, kind, value, index, drawing, nil, nil)
}
func (b *Builder) SetImage(name string, value any) error {
	return b.SetTypedValue(name, "DynamicImage", value, 0, true)
}
func (b *Builder) SetBarcode(name string, value any) error {
	return b.SetTypedValue(name, "DynamicBarcode", value, 0, true)
}

func (b *Builder) ChangeAttributes(name string, values Map, index int) error {
	return b.ChangeRepeatedAttributes(name, values, index, nil, nil, b.inferredType(name))
}

// 型を省略するAPIだけが当該ページの定義を参照する。明示型は変更しない。
func (b *Builder) inferredType(name string) string {
	d := b.definition
	if b.page != nil {
		if p, ok := b.page["Definition"].(Map); ok {
			d = p
		}
	}
	objects, _ := d["Objects"].([]any)
	objects = append([]any{}, objects...)
	if pages, ok := d["pages"].([]any); ok {
		for _, raw := range pages {
			if p, ok := raw.(Map); ok {
				if items, ok := p["objects"].([]any); ok {
					objects = append(objects, items...)
				}
			}
		}
	}
	kinds := map[string]string{"0": "Text", "1": "Line", "2": "Square", "3": "Circle", "4": "Image", "5": "Barcode"}
	web := map[string]string{"text": "Text", "line": "Line", "rectangle": "Square", "ellipse": "Circle", "image": "Image", "barcode": "Barcode"}
	for _, raw := range objects {
		o, ok := raw.(Map)
		if !ok {
			continue
		}
		n := o["Name"]
		if n == nil {
			n = o["name"]
		}
		if n != strings.TrimSpace(name) {
			continue
		}
		if k, ok := kinds[fmt.Sprint(o["Kind"])]; ok {
			return "Dynamic" + k
		}
		if k, ok := web[fmt.Sprint(o["kind"])]; ok {
			return "Dynamic" + k
		}
	}
	return "DynamicText"
}
func (b *Builder) ChangeRepeatedAttributes(name string, values Map, index int, ix, iy *int, kind string) error {
	if b.page == nil {
		return errors.New("call PageStart first")
	}
	if err := validateIndices(index, ix, iy); err != nil {
		return err
	}
	n, e := required(name)
	if e != nil {
		return e
	}
	k, e := required(kind)
	if e != nil {
		return e
	}
	normalized := Map{}
	for key, value := range values {
		if value == nil {
			return errors.New("attribute values cannot be null")
		}
		v, e := scalar(value)
		if e != nil {
			return e
		}
		normalized[key] = v
	}
	item := Map{"Name": n, "Type": k, "Index": index, "Values": normalized}
	if ix != nil {
		item["IndexX"] = *ix
		item["IndexY"] = *iy
	}
	b.page["DynamicAttributes"] = append(b.page["DynamicAttributes"].([]any), item)
	return nil
}
func (b *Builder) PageEnd() error {
	if b.page == nil {
		return errors.New("call PageStart first")
	}
	b.pages = append(b.pages, b.page)
	b.page = nil
	return nil
}
func (b *Builder) Value() (Map, error) {
	if b.page != nil {
		return nil, errors.New("an unfinished page exists")
	}
	if b.definition == nil {
		return nil, errors.New("a definition is required")
	}
	if len(b.pages) == 0 {
		return nil, errors.New("at least one completed page is required")
	}
	return copyDocument(Map{"Format": Format, "Version": Version, "Definition": b.definition, "Pages": b.pages}).(Map), nil
}

func validateIndices(index int, x, y *int) error {
	if index < 0 || (x == nil) != (y == nil) || (x != nil && (*x < 0 || *y < 0)) {
		return errors.New("indices must be non-negative; supply both x and y, or neither")
	}
	return nil
}

// Return a snapshot without converting integer indices to floating point.
func copyDocument(value any) any {
	switch v := value.(type) {
	case Map:
		out := Map{}
		for key, item := range v {
			out[key] = copyDocument(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = copyDocument(item)
		}
		return out
	default:
		return value
	}
}
func (b *Builder) JSON(pretty bool) ([]byte, error) {
	v, e := b.Value()
	if e != nil {
		return nil, e
	}
	if pretty {
		return json.MarshalIndent(v, "", "  ")
	}
	return json.Marshal(v)
}
func (b *Builder) Save(path string) error {
	data, e := b.JSON(true)
	if e != nil {
		return e
	}
	dir := filepath.Dir(path)
	f, e := os.CreateTemp(dir, ".print-*.tmp")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(append(data, '\n')); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, path)
}
