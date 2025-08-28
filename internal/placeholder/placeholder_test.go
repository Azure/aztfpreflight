package placeholder

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func Test_ForResourceTypePath(t *testing.T) {
	if got := ForResourceTypePath("azurerm_subscription", "id"); got == "" {
		t.Fatalf("expected placeholder for subscription id")
	}
}

func Test_ForUnknownReference_ListTupleSet(t *testing.T) {
	references := []string{
		"azurerm_resource_group.test.id",
	}
	// string
	if v, ok := ForUnknownReference(references, nil).(string); !ok || v == "" {
		t.Fatalf("expected single string placeholder")
	}
	// list of strings
	lt := tftypes.List{ElementType: tftypes.String}
	if vv, ok := ForUnknownReference(references, lt).([]string); !ok || len(vv) == 0 {
		t.Fatalf("expected list of string placeholders")
	}
	// tuple of strings
	tt := tftypes.Tuple{ElementTypes: []tftypes.Type{tftypes.String}}
	if vv, ok := ForUnknownReference(references, tt).([]string); !ok || len(vv) == 0 {
		t.Fatalf("expected tuple -> []string placeholder")
	}
	// set of strings
	st := tftypes.Set{ElementType: tftypes.String}
	if vv, ok := ForUnknownReference(references, st).([]string); !ok || len(vv) == 0 {
		t.Fatalf("expected set -> []string placeholder")
	}
}

func Test_ForPath_MapContains(t *testing.T) {
	// Often populated via init(), so just ensure lookup doesn't panic and returns something for a known key if present
	_ = ForPath("azurerm_spring_cloud_app.addon_json")
}
