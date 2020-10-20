package ipstate

import (
	"bytes"
	"fmt"
)

type validationErrors []error

func (e validationErrors) Error() string {
	b := bytes.NewBufferString("")
	for _, err := range e {
		_, _ = fmt.Fprintf(b, "\t%s", err)
	}
	return b.String()
}

func (m *PrefixIdentifier) Validate() error {
	var errs validationErrors
	err := m.GetIdentifier().Validate()
	if err != nil {
		errs = append(errs, err)
	}
	err = m.GetAddrFamily().Validate()
	if err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}
