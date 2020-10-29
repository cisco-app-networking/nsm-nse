package ipstate

func (m *PrefixIdentifier) Validate() error {
	err := m.GetIdentifier().Validate()
	if err != nil {
		return err
	}
	err = m.GetAddrFamily().Validate()
	if err != nil {
		return err
	}
	return nil
}
