package nseconfig

import (
	"bytes"
	"fmt"
	"net"
)

type InvalidConfigErrors []error

func (v InvalidConfigErrors) Error() string {
	b := bytes.NewBufferString("validation failed with errors: \n")
	for _, err := range v {
		fmt.Fprintf(b, "\t%s\n", err)
	}
	return b.String()
}

func (c *WCM) validate() error {
	if c == nil {
		return nil
	}
	var errs InvalidConfigErrors
	if empty(c.Address) {
		errs = append(errs, fmt.Errorf("wcm address is not set"))
	}
	if empty(c.Name) {
		errs = append(errs, fmt.Errorf("wcm name is not set"))
	}
	if empty(c.ConnectivityDomain) {
		errs = append(errs, fmt.Errorf("connectivity domain is not set"))
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (v VL3) validate() error {
	var errs InvalidConfigErrors

	if _, _, err := net.ParseCIDR(v.WCMD.DefaultPrefixPool); err != nil {
		errs = append(errs, fmt.Errorf("prefix pool is not a valid subnet: %s", err))
	}
	for i, r := range v.WCMD.Routes {
		if _, _, err := net.ParseCIDR(r); err != nil {
			errs = append(errs, fmt.Errorf("route nr %d with value %s is not a valid subnet: %s", i, r, err))
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (e Endpoint) validate() error {
	var errs InvalidConfigErrors
	if len(errs) > 0 {
		return errs
	}
	for _, err := range []error{
		e.WCM.validate(),
		e.VL3.validate(),
	} {
		if err != nil {
			if verr, ok := err.(InvalidConfigErrors); ok {
				errs = append(errs, verr...)
			} else {
				errs = append(errs, err)
			}
		}
	}
	if e.WCM == nil {
		e.WCM = &WCM{}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (c Config) validate() error {
	if len(c.Endpoints) == 0 {
		return fmt.Errorf("no endpoints provided")
	}

	var errs InvalidConfigErrors

	for _, endp := range c.Endpoints {
		if err := endp.validate(); err != nil {
			if verr, ok := err.(InvalidConfigErrors); ok {
				errs = append(errs, verr...)
			} else {
				errs = append(errs, err)
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}
