package applicationruntime

import "testing"

func TestConfigurationVersion(t *testing.T) {
	for _, raw := range []string{"1.2.3", "1.2.3.01", "1.2.-1.0", "1.2.3.4294967296", "1.2.3.+1"} {
		if _, err := ParseConfigurationVersion(raw); err == nil {
			t.Fatal(raw)
		}
	}
	a, err := ParseConfigurationVersion("1.2.3.9")
	if err != nil {
		t.Fatal(err)
	}
	b, err := a.Next()
	if err != nil || b.String() != "1.2.3.10" || b.Compare(a) != 1 {
		t.Fatal(b, err)
	}
	max, _ := ParseConfigurationVersion("1.2.3.4294967295")
	if _, err = max.Next(); err == nil {
		t.Fatal("overflow")
	}
}
