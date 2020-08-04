package nseconfig

import (
	"bytes"
	"fmt"
	"net"
	"testing"

	"gopkg.in/yaml.v3"
	"gotest.tools/assert"
)

func TestNewConfig(t *testing.T) {
	for name, tc := range map[string]struct {
		file   string
		config *Config
		err    error
	}{
		"success": {
			file: testFile1,
			config: &Config{Endpoints: []*Endpoint{{NseServices: &NseServices{
				Name:               "wcm1",
				Address:            "golang.com:9000",
				AccessToken:        "123123",
				ConnectivityDomain: "test-connectivity-domain",
			}, VL3: VL3{
				IPAM: IPAM{
					DefaultPrefixPool: "192.168.33.0/24",
					PrefixLength:      24,
					Routes:            []string{"192.168.34.0/24"},
					ServerAddress:     "wcmd-example.wcm-cisco.com",
				},
				Ifname:      "nsm3",
				NameServers: []string{"nms.google.com", "nms.google.com2"},
			}}}},
		},
		"success-minimal-config": {
			file: testFile3,
			config: &Config{Endpoints: []*Endpoint{{VL3: VL3{
				IPAM: IPAM{
					DefaultPrefixPool: "192.168.33.0/24",
					Routes:            []string{"192.168.34.0/24"},
				},
				Ifname: "endpoint0",
			}}}},
		},
		"validation-errors": {
			file: testFile2,
			err: InvalidConfigErrors([]error{
				fmt.Errorf("wcm address is not set"),
				fmt.Errorf("wcm name is not set"),
				fmt.Errorf("connectivity domain is not set"),
				fmt.Errorf("prefix pool is not a valid subnet: %s", &net.ParseError{Type: "CIDR address", Text: "invalid-pull"}),
				fmt.Errorf("route nr %d with value %s is not a valid subnet: %s", 0, "invalid-route1", &net.ParseError{Type: "CIDR address", Text: "invalid-route1"}),
				fmt.Errorf("route nr %d with value %s is not a valid subnet: %s", 1, "invalid-route2", &net.ParseError{Type: "CIDR address", Text: "invalid-route2"}),
			}),
		},
	} {
		t.Run(name, func(t *testing.T) {
			cfg := &Config{}
			err := NewConfig(yaml.NewDecoder(bytes.NewBufferString(tc.file)), cfg)
			if tc.err != nil {
				if err == nil {
					t.Fatal("error should not be empty")
				}

				assert.Equal(t, tc.err.Error(), err.Error())
			} else {
				assert.DeepEqual(t, tc.config, cfg)
			}
		})
	}
}

const testFile1 = `
endpoints:
  - wcm:
      name: wcm1
      address: golang.com:9000
      accessToken: 123123
      connectivityDomain: test-connectivity-domain
    vl3:
      wcmd:
        defaultPrefixPool: 192.168.33.0/24
        prefixLength: 24
        routes: [192.168.34.0/24]
        serverAddress: wcmd-example.wcm-cisco.com
      ifName: nsm3
      nameServers: [nms.google.com, nms.google.com2]
`

const testFile2 = `
endpoints:
  - wcm:
      name: ""
      address: ""
    vl3:
      wcmd:
        defaultPrefixPool: invalid-pull
        prefixLength: 100
        routes: [invalid-route1, invalid-route2]
      ifName: 
      nameServers: []
`

const testFile3 = `
endpoints:
  - vl3:
      wcmd:
        defaultPrefixPool: 192.168.33.0/24
        routes: [192.168.34.0/24]
      ifName: endpoint0
`
