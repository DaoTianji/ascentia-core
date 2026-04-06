package memorywork

import "testing"

func TestParseMemoryItems(t *testing.T) {
	raw := `Here is JSON:
[{"category":"user","key":"pref_lang","content":"用户偏好中文"}]
`
	items, err := parseMemoryItems(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Key != "pref_lang" {
		t.Fatalf("%+v", items)
	}
}

func TestStripJSONFence(t *testing.T) {
	s := stripJSONFence("```json\n[]\n```")
	if s != "[]" {
		t.Fatal(s)
	}
}
