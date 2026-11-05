package applicationruntime

import (
	"fmt"
	"strconv"
	"strings"
)

// ConfigurationVersion is independent of capability and deployment SemVer.
type ConfigurationVersion [4]uint32

func ParseConfigurationVersion(value string) (ConfigurationVersion, error) {
	var version ConfigurationVersion
	parts := strings.Split(value, ".")
	if len(parts) != 4 {
		return version, fmt.Errorf("schema_invalid: configuration version")
	}
	for i, part := range parts {
		if part == "" || len(part) > 1 && part[0] == '0' {
			return version, fmt.Errorf("schema_invalid: configuration version")
		}
		for _, char := range part {
			if char < '0' || char > '9' {
				return version, fmt.Errorf("schema_invalid: configuration version")
			}
		}
		n, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return version, fmt.Errorf("schema_invalid: configuration version")
		}
		version[i] = uint32(n)
	}
	return version, nil
}
func (v ConfigurationVersion) String() string {
	return fmt.Sprintf("%d.%d.%d.%d", v[0], v[1], v[2], v[3])
}
func (v ConfigurationVersion) Compare(other ConfigurationVersion) int {
	for i, n := range v {
		if n < other[i] {
			return -1
		}
		if n > other[i] {
			return 1
		}
	}
	return 0
}
func (v ConfigurationVersion) Next() (ConfigurationVersion, error) {
	if v[3] == ^uint32(0) {
		return v, fmt.Errorf("conflict: version exhausted")
	}
	v[3]++
	return v, nil
}
