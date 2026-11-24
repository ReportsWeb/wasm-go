package reportsweb

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
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
		return errors.New("帳票定義の形式が不正です。")
	}
	return nil
}

func (b *Builder) SetDefinition(d Map) error {
	if b.page != nil {
		return errors.New("ページを終了してから定義を変更してください。")
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
		return errors.New("次のページを開始する前にpageEndで現在のページを終了してください。")
	}
	if definition != nil {
		if err := ValidateDefinition(definition); err != nil {
			return err
		}
	}
	if definition == nil && b.definition == nil {
		return errors.New("先に帳票定義を設定してください。")
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
			return nil, errors.New("数値には有限の値を指定してください。")
		}
		return jsNumber(float64(v), 32), nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, errors.New("数値には有限の値を指定してください。")
		}
		return jsNumber(v, 64), nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, json.Number:
		return fmt.Sprint(v), nil
	}
	return nil, errors.New("値は文字列・数値・真偽値・nullのいずれかで指定してください。")
}

// jsNumber は小数を TypeScript/Node.js 版（JavaScript の数の文字列）と同じ書き方にする（OP-R3-WRAP）。
// 以前は fmt.Sprint で 1234567.5 が 1.2345675e+06、1000000.0 が 1e+06 と指数で帳票に出た。
// 最短で元の値に戻る桁を使い、指数は 1e21 以上・1e-7 未満だけ（JavaScript の Number#toString）。
func jsNumber(f float64, bits int) string {
	if f == 0 {
		return "0"
	}
	s := strconv.FormatFloat(f, 'e', -1, bits)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	mant, expText, _ := strings.Cut(s, "e")
	exp, _ := strconv.Atoi(expText)
	digits := strings.TrimRight(strings.Replace(mant, ".", "", 1), "0")
	if digits == "" {
		return "0"
	}
	n, k := exp+1, len(digits)
	var r string
	switch {
	case k <= n && n <= 21:
		r = digits + strings.Repeat("0", n-k)
	case 0 < n && n <= 21:
		r = digits[:n] + "." + digits[n:]
	case -6 < n && n <= 0:
		r = "0." + strings.Repeat("0", -n) + digits
	default:
		e := n - 1
		sign := "+"
		if e < 0 {
			sign, e = "-", -e
		}
		r = digits[:1]
		if k > 1 {
			r += "." + digits[1:]
		}
		r += "e" + sign + strconv.Itoa(e)
	}
	if neg {
		return "-" + r
	}
	return r
}
func required(v string) (string, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return "", errors.New("名前を指定してください。")
	}
	return v, nil
}

// 当該ページで有効な帳票定義の物体。
func (b *Builder) objects() []any {
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
	return objects
}

// requireObject：Reports.net の Write と同じく、帳票に無い名前はその場で断る（OP-R3-WRAP。TypeScript/Node.js 版と同じ）。
// 以前は黙って受け、その値は帳票に出なかった。
func (b *Builder) requireObject(name string) error {
	for _, raw := range b.objects() {
		if o, ok := raw.(Map); ok {
			n := o["Name"]
			if n == nil {
				n = o["name"]
			}
			if n == strings.TrimSpace(name) {
				return nil
			}
		}
	}
	return fmt.Errorf("帳票にオブジェクト「%s」がありません。", name)
}
func (b *Builder) add(name, kind string, value any, index int, drawing bool, ix, iy *int) error {
	if b.page == nil {
		return errors.New("先にページを開始してください。")
	}
	if err := validateIndices(index, ix, iy); err != nil {
		return err
	}
	n, e := required(name)
	if e != nil {
		return e
	}
	if e := b.requireObject(n); e != nil {
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
	objects := b.objects()
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
		return errors.New("先にページを開始してください。")
	}
	if err := validateIndices(index, ix, iy); err != nil {
		return err
	}
	n, e := required(name)
	if e != nil {
		return e
	}
	if e := b.requireObject(n); e != nil {
		return e
	}
	// 型を空で渡したときは、他の言語と同じく当該ページの定義から推論する（OP-R3-WRAP。以前は「a name is required」で断った）。
	if strings.TrimSpace(kind) == "" {
		kind = b.inferredType(n)
	}
	k, e := required(kind)
	if e != nil {
		return e
	}
	normalized := Map{}
	for key, value := range values {
		if value == nil {
			return errors.New("属性値にnullは指定できません。")
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
		return errors.New("先にページを開始してください。")
	}
	b.pages = append(b.pages, b.page)
	b.page = nil
	return nil
}
func (b *Builder) Value() (Map, error) {
	if b.page != nil {
		return nil, errors.New("終了していないページがあります。ページを終了してください。")
	}
	if b.definition == nil {
		return nil, errors.New("先に帳票定義を設定してください。")
	}
	if len(b.pages) == 0 {
		return nil, errors.New("少なくとも1ページを終了してください。")
	}
	return copyDocument(Map{"Format": Format, "Version": Version, "Definition": b.definition, "Pages": b.pages}).(Map), nil
}

func validateIndices(index int, x, y *int) error {
	if index < 0 || (x == nil) != (y == nil) || (x != nil && (*x < 0 || *y < 0)) {
		return errors.New("インデックスには0以上の整数を指定し、繰返し座標はxとyを両方指定してください。")
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
func requireWritable(path string) error {
    info, err := os.Stat(path)
    if os.IsNotExist(err) { return nil }; if err != nil { return err }
    if !info.Mode().IsRegular() { return nil }
    if info.Mode().Perm() & 0222 == 0 { return errors.New("読み取り専用のため保存できません。別名で保存してください。") }
    probe, err := os.OpenFile(path, os.O_WRONLY, 0)
    if errors.Is(err, os.ErrPermission) { return errors.New("読み取り専用のため保存できません。別名で保存してください。") }
    if err != nil { return err }; return probe.Close()
}
func (b *Builder) Save(path string) error {
	if e := requireWritable(path); e != nil { return e }
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
	if e := requireWritable(path); e != nil { return e }
	return os.Rename(tmp, path)
}
